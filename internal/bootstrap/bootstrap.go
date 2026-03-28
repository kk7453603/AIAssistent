package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/config"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
	"github.com/kirillkom/personal-ai-assistant/internal/core/usecase"
	"github.com/kirillkom/personal-ai-assistant/internal/observability/metrics"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/chunking"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/extractor/metadata"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/extractor/plaintext"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/fallback"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/ollama"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/openaicompat"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/routing"
	paamcp "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/mcp"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/queue/nats"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/repository/postgres"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/resilience"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/storage/localfs"
	sourceobsidian "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/source/obsidian"
	sourceupload "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/source/upload"
	sourceweb "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/source/web"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/vector/qdrant"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/websearch/searxng"
)

type App struct {
	Config config.Config

	Queue            ports.MessageQueue
	Repo             ports.DocumentRepository
	IngestUC         ports.DocumentIngestor
	ProcessUC        ports.DocumentProcessor
	EnrichUC         ports.DocumentEnricher
	QueryUC          ports.DocumentQueryService
	AgentUC          ports.AgentChatService
	ToolRegistry     *paamcp.ToolRegistry
	MCPClientMgr     *paamcp.ClientManager
	WebSearcher      ports.WebSearcher
	Tasks            ports.TaskStore
	ModelProviderMap map[string]string // model ID → provider name (e.g., "paa-huggingface" → "huggingface")

	closeFn func()
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	db, err := postgres.OpenDB(cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	repo := postgres.NewDocumentRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}
	conversationRepo := postgres.NewConversationRepository(db)
	taskRepo := postgres.NewTaskRepository(db)
	memoryRepo := postgres.NewMemoryRepository(db)

	storage, err := localfs.New(cfg.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("init object storage: %w", err)
	}

	resilienceExecutor := resilience.NewExecutor(resilience.Config{
		RetryMaxAttempts:        cfg.ResilienceRetryMaxAttempts,
		RetryInitialBackoff:     time.Duration(cfg.ResilienceRetryInitialBackoffMS) * time.Millisecond,
		RetryMaxBackoff:         time.Duration(cfg.ResilienceRetryMaxBackoffMS) * time.Millisecond,
		RetryMultiplier:         cfg.ResilienceRetryMultiplier,
		BreakerEnabled:          cfg.ResilienceBreakerEnabled,
		BreakerMinRequests:      uint32(cfg.ResilienceBreakerMinRequests),
		BreakerFailureRatio:     cfg.ResilienceBreakerFailureRatio,
		BreakerOpenTimeout:      time.Duration(cfg.ResilienceBreakerOpenMS) * time.Millisecond,
		BreakerHalfOpenMaxCalls: uint32(cfg.ResilienceBreakerHalfOpenCalls),
	})

	retryOnFailedConnect := cfg.NATSRetryOnFailedConnect
	queue, err := nats.NewWithOptions(cfg.NATSURL, cfg.NATSSubject, nats.Options{
		ConnectTimeout:       time.Duration(cfg.NATSConnectTimeoutMS) * time.Millisecond,
		ReconnectWait:        time.Duration(cfg.NATSReconnectWaitMS) * time.Millisecond,
		MaxReconnects:        cfg.NATSMaxReconnects,
		RetryOnFailedConnect: &retryOnFailedConnect,
		ResilienceExecutor:   resilienceExecutor,
	})
	if err != nil {
		return nil, fmt.Errorf("init message queue: %w", err)
	}

	ollamaClient := ollama.NewWithOptions(cfg.OllamaURL, cfg.OllamaGenModel, cfg.OllamaEmbedModel, ollama.Options{
		PlannerModel:       cfg.OllamaPlannerModel,
		ResilienceExecutor: resilienceExecutor,
	})

	// Resolve model name for external providers.
	llmModel := cfg.LLMModel
	if llmModel == "" {
		llmModel = cfg.OllamaGenModel
	}

	// Select LLM provider (generator + classifier).
	var classifier ports.DocumentClassifier
	var generator ports.AnswerGenerator
	llmProvider := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
	llmURL := resolveProviderURL(llmProvider, cfg.LLMProviderURL)
	switch llmProvider {
	case "openai-compat", "groq", "together", "openrouter", "cerebras", "huggingface":
		oacClient := openaicompat.New(llmURL, cfg.LLMProviderKey, llmModel,
			openaicompat.Options{ExtraHeaders: providerHeaders(llmProvider)})
		classifier = openaicompat.NewClassifier(oacClient)
		generator = openaicompat.NewGenerator(oacClient)
	default: // "ollama"
		classifier = ollama.NewClassifier(ollamaClient)
		generator = ollama.NewGenerator(ollamaClient)
	}

	// Fallback LLM (optional).
	logger := slog.Default()
	var fbGen ports.AnswerGenerator
	var fbCls ports.DocumentClassifier
	if fbProvider := strings.ToLower(strings.TrimSpace(cfg.LLMFallbackProvider)); fbProvider != "" && fbProvider != llmProvider {
		fbModel := cfg.LLMFallbackModel
		if fbModel == "" {
			fbModel = cfg.OllamaGenModel
		}
		fbURL := resolveProviderURL(fbProvider, cfg.LLMFallbackURL)
		switch fbProvider {
		case "openai-compat", "groq", "together", "openrouter", "cerebras", "huggingface":
			fbClient := openaicompat.New(fbURL, cfg.LLMFallbackKey, fbModel,
				openaicompat.Options{ExtraHeaders: providerHeaders(fbProvider)})
			fbCls = openaicompat.NewClassifier(fbClient)
			fbGen = openaicompat.NewGenerator(fbClient)
		default: // "ollama"
			fbCls = ollama.NewClassifier(ollamaClient)
			fbGen = ollama.NewGenerator(ollamaClient)
		}
		generator = fallback.NewGenerator(generator, fbGen, logger)
		classifier = fallback.NewClassifier(classifier, fbCls, logger)
	}

	// Extra LLM providers (model-based routing via UI).
	modelProviderMap := make(map[string]string)
	if extras := cfg.ParseExtraProviders(); len(extras) > 0 {
		generators := map[string]ports.AnswerGenerator{llmProvider: generator}
		classifiers := map[string]ports.DocumentClassifier{llmProvider: classifier}

		for _, extra := range extras {
			extraModel := extra.Model
			if extraModel == "" {
				extraModel = cfg.OllamaGenModel
			}
			url := resolveProviderURL(extra.Name, extra.URL)
			oacClient := openaicompat.New(url, extra.Key, extraModel,
				openaicompat.Options{ExtraHeaders: providerHeaders(extra.Name)})
			var eGen ports.AnswerGenerator = openaicompat.NewGenerator(oacClient)
			var eCls ports.DocumentClassifier = openaicompat.NewClassifier(oacClient)
			// Wrap in fallback if configured.
			if fbGen != nil {
				eGen = fallback.NewGenerator(eGen, fbGen, logger)
				eCls = fallback.NewClassifier(eCls, fbCls, logger)
			}
			generators[extra.Name] = eGen
			classifiers[extra.Name] = eCls
			modelProviderMap["paa-"+extra.Name] = extra.Name
		}

		generator = routing.NewGenerator(generators, llmProvider, logger)
		classifier = routing.NewClassifier(classifiers, llmProvider, logger)
	}

	// Select reranker provider (independent from LLM).
	var reranker ports.Reranker
	rerankModel := cfg.RerankModel
	if rerankModel == "" {
		rerankModel = llmModel
	}
	rerankProvider := strings.ToLower(strings.TrimSpace(cfg.RerankProvider))
	switch rerankProvider {
	case "openai-compat":
		oacClient := openaicompat.New(cfg.RerankProviderURL, cfg.RerankProviderKey, rerankModel)
		reranker = openaicompat.NewReranker(oacClient)
	case "ollama":
		reranker = ollama.NewReranker(ollamaClient)
	default: // "fallback"
		reranker = usecase.NewFallbackReranker()
	}

	// Select embedding provider.
	var embedder ports.Embedder
	embedProvider := strings.ToLower(strings.TrimSpace(cfg.EmbedProvider))
	switch embedProvider {
	case "openai-compat":
		oacClient := openaicompat.New(cfg.EmbedProviderURL, cfg.EmbedProviderKey, cfg.OllamaEmbedModel)
		embedder = openaicompat.NewEmbedder(oacClient, cfg.OllamaEmbedModel)
	default: // "ollama"
		embedder = ollama.NewEmbedder(ollamaClient)
	}

	vectorDB := qdrant.NewWithOptions(cfg.QdrantURL, cfg.QdrantCollection, qdrant.Options{
		ResilienceExecutor: resilienceExecutor,
	})
	memoryVector := qdrant.NewMemoryClientWithOptions(cfg.QdrantURL, cfg.QdrantMemoryCollection, qdrant.Options{
		ResilienceExecutor: resilienceExecutor,
	})

	if cfg.QdrantEmbedDim > 0 {
		if err := vectorDB.EnsureCollection(ctx, cfg.QdrantEmbedDim); err != nil {
			return nil, fmt.Errorf("ensure qdrant documents collection: %w", err)
		}
		if err := memoryVector.EnsureCollection(ctx, cfg.QdrantEmbedDim); err != nil {
			return nil, fmt.Errorf("ensure qdrant memory collection: %w", err)
		}
	}
	chunkStrategy := strings.ToLower(strings.TrimSpace(cfg.ChunkStrategy))
	var defaultChunker ports.Chunker
	switch chunkStrategy {
	case "", "fixed":
		defaultChunker = chunking.NewSplitter(cfg.ChunkSize, cfg.ChunkOverlap)
	case "markdown", "md":
		defaultChunker = chunking.NewMarkdownSplitter(cfg.ChunkSize, cfg.ChunkOverlap)
	default:
		return nil, fmt.Errorf("unsupported CHUNK_STRATEGY=%q: use fixed or markdown", cfg.ChunkStrategy)
	}

	chunkerRegistry := chunking.NewRegistry(defaultChunker)
	if chunkConfigs := config.ParseChunkConfig(cfg.ChunkConfig); chunkConfigs != nil {
		for sourceType, cc := range chunkConfigs {
			size := cc.Size
			if size <= 0 {
				size = cfg.ChunkSize
			}
			overlap := cc.Overlap
			if overlap < 0 {
				overlap = cfg.ChunkOverlap
			}
			switch strings.ToLower(cc.Strategy) {
			case "markdown", "md":
				chunkerRegistry.Register(sourceType, chunking.NewMarkdownSplitter(size, overlap))
			default:
				chunkerRegistry.Register(sourceType, chunking.NewSplitter(size, overlap))
			}
		}
	}
	extractor := plaintext.NewExtractor(storage)

	fusionStrategy := domain.FusionStrategy(strings.ToLower(strings.TrimSpace(cfg.RAGFusionStrategy)))
	if fusionStrategy != domain.FusionStrategyRRF {
		return nil, fmt.Errorf("unsupported RAG_FUSION_STRATEGY=%q: only rrf is supported", cfg.RAGFusionStrategy)
	}

	sourceAdapters := map[string]ports.SourceAdapter{
		"upload":   sourceupload.New(),
		"obsidian": sourceobsidian.New(),
		"web":      sourceweb.New(nil),
	}
	ingestUC := usecase.NewIngestDocumentUseCase(repo, storage, queue, sourceAdapters)
	metaExtractor := metadata.New()
	processUC := usecase.NewProcessDocumentUseCase(repo, extractor, metaExtractor, chunkerRegistry, embedder, vectorDB, queue)
	enrichUC := usecase.NewEnrichDocumentUseCase(repo, extractor, classifier, vectorDB)
	queryUC := usecase.NewQueryUseCase(embedder, vectorDB, generator, usecase.QueryOptions{
		RetrievalMode:         domain.RetrievalMode(strings.ToLower(strings.TrimSpace(cfg.RAGRetrievalMode))),
		HybridCandidates:      cfg.RAGHybridCandidates,
		FusionStrategy:        fusionStrategy,
		FusionRRFK:            cfg.RAGFusionRRFK,
		RerankTopN:            cfg.RAGRerankTopN,
		Reranker:              reranker,
		QueryExpansionEnabled: cfg.QueryExpansionEnabled,
		QueryExpansionCount:   cfg.QueryExpansionCount,
	})
	// Web search (optional).
	var webSearcher ports.WebSearcher
	if cfg.WebSearchEnabled && cfg.WebSearchURL != "" {
		webSearcher = searxng.New(cfg.WebSearchURL, cfg.WebSearchLimit)
	}

	// MCP client manager (connects to external MCP servers).
	var mcpConfigs []paamcp.ServerConfig
	if cfg.MCPServers != "" {
		if err := json.Unmarshal([]byte(cfg.MCPServers), &mcpConfigs); err != nil {
			slog.Warn("mcp_servers_config_parse_error", "err", err, "raw", cfg.MCPServers)
		}
	}
	mcpClientMgr := paamcp.NewClientManager(ctx, mcpConfigs)
	toolRegistry := paamcp.NewToolRegistry(mcpClientMgr)

	agentUC := usecase.NewAgentChatUseCase(
		queryUC,
		embedder,
		conversationRepo,
		taskRepo,
		memoryRepo,
		memoryVector,
		webSearcher,
		nil, // obsidianWriter — set later via SetObsidianWriter after Router is created
		toolRegistry,
		domain.AgentLimits{
			MaxIterations:       cfg.AgentMaxIterations,
			Timeout:             time.Duration(cfg.AgentTimeoutSeconds) * time.Second,
			PlannerTimeout:      time.Duration(cfg.AgentPlannerTimeoutSeconds) * time.Second,
			ToolTimeout:         time.Duration(cfg.AgentToolTimeoutSeconds) * time.Second,
			ShortMemoryMessages: cfg.AgentShortMemoryMsgs,
			SummaryEveryTurns:   cfg.AgentSummaryEveryTurns,
			MemoryTopK:          cfg.AgentMemoryTopK,
			KnowledgeTopK:       cfg.AgentKnowledgeTopK,
			IntentRouterEnabled: cfg.AgentIntentRouterEnabled,
		},
		metrics.NewAgentMetrics("agent"),
	)

	return &App{
		Config:           cfg,
		Queue:            queue,
		Repo:             repo,
		ModelProviderMap: modelProviderMap,

		IngestUC:       ingestUC,
		ProcessUC:      processUC,
		EnrichUC:       enrichUC,
		QueryUC:        queryUC,
		AgentUC:        agentUC,
		ToolRegistry:   toolRegistry,
		MCPClientMgr:   mcpClientMgr,
		WebSearcher:    webSearcher,
		Tasks:          taskRepo,

		closeFn: func() {
			toolRegistry.Close()
			queue.Close()
			_ = db.Close()
		},
	}, nil
}

func (a *App) Close() {
	if a.closeFn != nil {
		a.closeFn()
	}
}

// resolveProviderURL returns the default base URL for known providers when no explicit URL is given.
func resolveProviderURL(provider, explicitURL string) string {
	if explicitURL != "" {
		return explicitURL
	}
	switch provider {
	case "huggingface":
		return "https://router.huggingface.co/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "together":
		return "https://api.together.xyz/v1"
	case "cerebras":
		return "https://api.cerebras.ai/v1"
	default:
		return explicitURL
	}
}

// providerHeaders returns provider-specific HTTP headers.
func providerHeaders(provider string) map[string]string {
	switch provider {
	case "huggingface":
		return map[string]string{"X-Wait-For-Model": "true"}
	default:
		return nil
	}
}
