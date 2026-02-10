package bootstrap

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kirillkom/personal-ai-assistant/internal/config"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
	"github.com/kirillkom/personal-ai-assistant/internal/core/usecase"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/chunking"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/extractor/plaintext"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/ollama"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/queue/nats"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/repository/postgres"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/storage/localfs"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/vector/qdrant"
)

type App struct {
	Config config.Config

	DB    *sql.DB
	Queue *nats.Queue

	Repo ports.DocumentRepository

	IngestUC  *usecase.IngestDocumentUseCase
	ProcessUC *usecase.ProcessDocumentUseCase
	QueryUC   *usecase.QueryUseCase
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

	storage, err := localfs.New(cfg.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("init object storage: %w", err)
	}

	queue, err := nats.New(cfg.NATSURL, cfg.NATSSubject)
	if err != nil {
		return nil, fmt.Errorf("init message queue: %w", err)
	}

	ollamaClient := ollama.New(cfg.OllamaURL, cfg.OllamaGenModel, cfg.OllamaEmbedModel)
	classifier := ollama.NewClassifier(ollamaClient)
	embedder := ollama.NewEmbedder(ollamaClient)
	generator := ollama.NewGenerator(ollamaClient)

	vectorDB := qdrant.New(cfg.QdrantURL, cfg.QdrantCollection)
	chunker := chunking.NewSplitter(cfg.ChunkSize, cfg.ChunkOverlap)
	extractor := plaintext.NewExtractor(storage)

	ingestUC := usecase.NewIngestDocumentUseCase(repo, storage, queue)
	processUC := usecase.NewProcessDocumentUseCase(repo, extractor, classifier, chunker, embedder, vectorDB)
	queryUC := usecase.NewQueryUseCase(embedder, vectorDB, generator)

	return &App{
		Config: cfg,
		DB:     db,
		Queue:  queue,
		Repo:   repo,

		IngestUC:  ingestUC,
		ProcessUC: processUC,
		QueryUC:   queryUC,
	}, nil
}

func (a *App) Close() {
	if a.Queue != nil {
		a.Queue.Close()
	}
	if a.DB != nil {
		_ = a.DB.Close()
	}
}
