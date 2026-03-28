package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type ProcessDocumentUseCase struct {
	repo          ports.DocumentRepository
	extractor     ports.TextExtractor
	metaExtractor ports.MetadataExtractor
	chunkers      ports.ChunkerRegistry
	embedder      ports.Embedder
	vectorDB      ports.VectorStore
	queue         ports.MessageQueue
}

func NewProcessDocumentUseCase(
	repo ports.DocumentRepository,
	extractor ports.TextExtractor,
	metaExtractor ports.MetadataExtractor,
	chunkers ports.ChunkerRegistry,
	embedder ports.Embedder,
	vectorDB ports.VectorStore,
	queue ports.MessageQueue,
) *ProcessDocumentUseCase {
	return &ProcessDocumentUseCase{
		repo:          repo,
		extractor:     extractor,
		metaExtractor: metaExtractor,
		chunkers:      chunkers,
		embedder:      embedder,
		vectorDB:      vectorDB,
		queue:         queue,
	}
}

func (uc *ProcessDocumentUseCase) ProcessByID(ctx context.Context, documentID string) error {
	if err := uc.markStatus(ctx, documentID, domain.StatusProcessing, ""); err != nil {
		return fmt.Errorf("set status=processing: %w", err)
	}

	_, err := uc.processPipeline(ctx, documentID)
	if err != nil {
		if failErr := uc.markFailed(ctx, documentID, err); failErr != nil {
			return fmt.Errorf("%w; mark failed status: %v", err, failErr)
		}
		return err
	}

	if err := uc.markStatus(ctx, documentID, domain.StatusReady, ""); err != nil {
		return fmt.Errorf("set status=ready: %w", err)
	}

	return nil
}

func (uc *ProcessDocumentUseCase) processPipeline(ctx context.Context, documentID string) (*domain.Document, error) {
	doc, err := uc.loadDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}

	text, err := uc.extractText(ctx, doc)
	if err != nil {
		return nil, err
	}

	meta, err := uc.extractMetadata(ctx, doc, text)
	if err != nil {
		return nil, err
	}

	chunks, err := uc.chunk(ctx, text, meta.SourceType)
	if err != nil {
		return nil, err
	}

	vectors, err := uc.embed(ctx, chunks)
	if err != nil {
		return nil, err
	}

	uc.applyMetadata(doc, meta)

	if err := uc.index(ctx, doc, chunks, vectors); err != nil {
		return nil, err
	}

	// Publish enrichment event (best-effort — don't fail the pipeline).
	if pubErr := uc.queue.PublishDocumentEnrich(ctx, doc.ID); pubErr != nil {
		slog.Warn("publish_enrich_failed", "document_id", doc.ID, "error", pubErr)
	}

	return doc, nil
}

func (uc *ProcessDocumentUseCase) loadDocument(ctx context.Context, documentID string) (*domain.Document, error) {
	doc, err := uc.repo.GetByID(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("fetch document by id: %w", err)
	}
	return doc, nil
}

func (uc *ProcessDocumentUseCase) extractText(ctx context.Context, doc *domain.Document) (string, error) {
	text, err := uc.extractor.Extract(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("extract text: %w", err)
	}
	if text == "" {
		return "", domain.WrapError(domain.ErrInvalidInput, "extract text", errors.New("empty extracted text"))
	}
	return text, nil
}

func (uc *ProcessDocumentUseCase) extractMetadata(ctx context.Context, doc *domain.Document, text string) (domain.DocumentMetadata, error) {
	meta, err := uc.metaExtractor.ExtractMetadata(ctx, doc, text)
	if err != nil {
		return domain.DocumentMetadata{}, fmt.Errorf("extract metadata: %w", err)
	}
	return meta, nil
}

func (uc *ProcessDocumentUseCase) chunk(_ context.Context, text string, sourceType string) ([]string, error) {
	chunker := uc.chunkers.ForSource(sourceType)
	chunks := chunker.Split(text)
	if len(chunks) == 0 {
		return nil, domain.WrapError(domain.ErrInvalidInput, "chunk document", errors.New("chunking produced zero chunks"))
	}
	return chunks, nil
}

func (uc *ProcessDocumentUseCase) embed(ctx context.Context, chunks []string) ([][]float32, error) {
	vectors, err := uc.embedder.Embed(ctx, chunks)
	if err != nil {
		return nil, fmt.Errorf("embed chunks: %w", err)
	}
	if len(vectors) != len(chunks) {
		return nil, domain.WrapError(
			domain.ErrInvalidInput,
			"embed chunks",
			fmt.Errorf("vectors/chunks mismatch: %d/%d", len(vectors), len(chunks)),
		)
	}
	return vectors, nil
}

func (uc *ProcessDocumentUseCase) index(ctx context.Context, doc *domain.Document, chunks []string, vectors [][]float32) error {
	if err := uc.vectorDB.IndexChunks(ctx, doc, chunks, vectors); err != nil {
		return fmt.Errorf("index chunks in vector db: %w", err)
	}
	return nil
}

func (uc *ProcessDocumentUseCase) markStatus(ctx context.Context, documentID string, status domain.DocumentStatus, errMessage string) error {
	return uc.repo.UpdateStatus(ctx, documentID, status, errMessage)
}

func (uc *ProcessDocumentUseCase) markFailed(ctx context.Context, documentID string, processErr error) error {
	if processErr == nil {
		return nil
	}
	return uc.markStatus(ctx, documentID, domain.StatusFailed, processErr.Error())
}

func (uc *ProcessDocumentUseCase) applyMetadata(doc *domain.Document, meta domain.DocumentMetadata) {
	doc.SourceType = meta.SourceType
	doc.Category = meta.Category
	doc.Tags = meta.Tags
	doc.Title = meta.Title
	doc.Summary = meta.Summary
	doc.Headers = meta.Headers
	doc.Path = meta.Path
}
