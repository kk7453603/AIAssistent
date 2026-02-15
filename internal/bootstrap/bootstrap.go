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
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/queue/nats"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/repository/postgres"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/resilience"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/storage/localfs"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/vector/qdrant"
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
		ResilienceExecutor: resilienceExecutor,
	})
	classifier := ollama.NewClassifier(ollamaClient)
	embedder := ollama.NewEmbedder(ollamaClient)
	generator := ollama.NewGenerator(ollamaClient)

	vectorDB := qdrant.NewWithOptions(cfg.QdrantURL, cfg.QdrantCollection, qdrant.Options{
		ResilienceExecutor: resilienceExecutor,
	})
	memoryVector := qdrant.NewMemoryClientWithOptions(cfg.QdrantURL, cfg.QdrantMemoryCollection, qdrant.Options{
		ResilienceExecutor: resilienceExecutor,
	})
	chunker := chunking.NewSplitter(cfg.ChunkSize, cfg.ChunkOverlap)
	extractor := plaintext.NewExtractor(storage)

	fusionStrategy := domain.FusionStrategy(strings.ToLower(strings.TrimSpace(cfg.RAGFusionStrategy)))
	if fusionStrategy != domain.FusionStrategyRRF {
		return nil, fmt.Errorf("unsupported RAG_FUSION_STRATEGY=%q: only rrf is supported", cfg.RAGFusionStrategy)
	}

	ingestUC := usecase.NewIngestDocumentUseCase(repo, storage, queue)
	processUC := usecase.NewProcessDocumentUseCase(repo, extractor, classifier, chunker, embedder, vectorDB)
	queryUC := usecase.NewQueryUseCase(embedder, vectorDB, generator, usecase.QueryOptions{
		RetrievalMode:    domain.RetrievalMode(strings.ToLower(strings.TrimSpace(cfg.RAGRetrievalMode))),
		HybridCandidates: cfg.RAGHybridCandidates,
		FusionStrategy:   fusionStrategy,
		FusionRRFK:       cfg.RAGFusionRRFK,
		RerankTopN:       cfg.RAGRerankTopN,
	})
	agentUC := usecase.NewAgentChatUseCase(
		queryUC,
		embedder,
		conversationRepo,
		taskRepo,
		memoryRepo,
		memoryVector,
		domain.AgentLimits{
			MaxIterations:       cfg.AgentMaxIterations,
			Timeout:             time.Duration(cfg.AgentTimeoutSeconds) * time.Second,
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
