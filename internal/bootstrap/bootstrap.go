package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/config"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
	"github.com/kirillkom/personal-ai-assistant/internal/core/usecase"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/chunking"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/extractor/plaintext"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/ollama"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/openaicompat"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/queue/nats"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/repository/postgres"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/resilience"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/storage/localfs"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/vector/qdrant"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/websearch/searxng"
)

type App struct {
	Config config.Config

	Queue     ports.MessageQueue
	Repo      ports.DocumentRepository
	IngestUC  ports.DocumentIngestor
	ProcessUC ports.DocumentProcessor
	QueryUC   ports.DocumentQueryService
	AgentUC   ports.AgentChatService

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
	switch llmProvider {
	case "openai-compat", "groq", "together", "openrouter", "cerebras":
		oacClient := openaicompat.New(cfg.LLMProviderURL, cfg.LLMProviderKey, llmModel)
		classifier = openaicompat.NewClassifier(oacClient)
		generator = openaicompat.NewGenerator(oacClient)
	default: // "ollama"
		classifier = ollama.NewClassifier(ollamaClient)
		generator = ollama.NewGenerator(ollamaClient)
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
	var chunker ports.Chunker = chunking.NewSplitter(cfg.ChunkSize, cfg.ChunkOverlap)
	switch chunkStrategy {
	case "", "fixed":
		// default splitter
	case "markdown", "md":
		chunker = chunking.NewMarkdownSplitter(cfg.ChunkSize, cfg.ChunkOverlap)
	default:
		return nil, fmt.Errorf("unsupported CHUNK_STRATEGY=%q: use fixed or markdown", cfg.ChunkStrategy)
	}
	extractor := plaintext.NewExtractor(storage)

	fusionStrategy := domain.FusionStrategy(strings.ToLower(strings.TrimSpace(cfg.RAGFusionStrategy)))
	if fusionStrategy != domain.FusionStrategyRRF {
		return nil, fmt.Errorf("unsupported RAG_FUSION_STRATEGY=%q: only rrf is supported", cfg.RAGFusionStrategy)
	}

	ingestUC := usecase.NewIngestDocumentUseCase(repo, storage, queue)
	processUC := usecase.NewProcessDocumentUseCase(repo, extractor, classifier, chunker, embedder, vectorDB)
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

	agentUC := usecase.NewAgentChatUseCase(
		queryUC,
		embedder,
		conversationRepo,
		taskRepo,
		memoryRepo,
		memoryVector,
		webSearcher,
		nil, // obsidianWriter — set later via SetObsidianWriter after Router is created
		domain.AgentLimits{
			MaxIterations:       cfg.AgentMaxIterations,
			Timeout:             time.Duration(cfg.AgentTimeoutSeconds) * time.Second,
			PlannerTimeout:      time.Duration(cfg.AgentPlannerTimeoutSeconds) * time.Second,
			ToolTimeout:         time.Duration(cfg.AgentToolTimeoutSeconds) * time.Second,
			ShortMemoryMessages: cfg.AgentShortMemoryMsgs,
			SummaryEveryTurns:   cfg.AgentSummaryEveryTurns,
			MemoryTopK:          cfg.AgentMemoryTopK,
			KnowledgeTopK:       cfg.AgentKnowledgeTopK,
		},
	)

	return &App{
		Config: cfg,
		Queue:  queue,
		Repo:   repo,

		IngestUC:  ingestUC,
		ProcessUC: processUC,
		QueryUC:   queryUC,
		AgentUC:   agentUC,

		closeFn: func() {
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
