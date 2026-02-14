package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type ProcessDocumentUseCase struct {
	repo       ports.DocumentRepository
	extractor  ports.TextExtractor
	classifier ports.DocumentClassifier
	chunker    ports.Chunker
	embedder   ports.Embedder
	vectorDB   ports.VectorStore
}

func NewProcessDocumentUseCase(
	repo ports.DocumentRepository,
	extractor ports.TextExtractor,
	classifier ports.DocumentClassifier,
	chunker ports.Chunker,
	embedder ports.Embedder,
	vectorDB ports.VectorStore,
) *ProcessDocumentUseCase {
	return &ProcessDocumentUseCase{
		repo:       repo,
		extractor:  extractor,
		classifier: classifier,
		chunker:    chunker,
		embedder:   embedder,
		vectorDB:   vectorDB,
	}
}

func (uc *ProcessDocumentUseCase) ProcessByID(ctx context.Context, documentID string) error {
	if err := uc.markStatus(ctx, documentID, domain.StatusProcessing, ""); err != nil {
		return fmt.Errorf("set status=processing: %w", err)
	}

	doc, classification, err := uc.processPipeline(ctx, documentID)
	if err != nil {
		if failErr := uc.markFailed(ctx, documentID, err); failErr != nil {
			return fmt.Errorf("%w; mark failed status: %v", err, failErr)
		}
		return err
	}

	if err := uc.persistClassification(ctx, doc.ID, classification); err != nil {
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

func (uc *ProcessDocumentUseCase) processPipeline(ctx context.Context, documentID string) (*domain.Document, domain.Classification, error) {
	doc, err := uc.loadDocument(ctx, documentID)
	if err != nil {
		return nil, domain.Classification{}, err
	}

	text, err := uc.extractText(ctx, doc)
	if err != nil {
		return nil, domain.Classification{}, err
	}

	classification, err := uc.classify(ctx, text)
	if err != nil {
		return nil, domain.Classification{}, err
	}

	chunks, err := uc.chunk(ctx, text)
	if err != nil {
		return nil, domain.Classification{}, err
	}

	vectors, err := uc.embed(ctx, chunks)
	if err != nil {
		return nil, domain.Classification{}, err
	}

	uc.applyClassification(doc, classification)

	if err := uc.index(ctx, doc, chunks, vectors); err != nil {
		return nil, domain.Classification{}, err
	}

	return doc, classification, nil
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

func (uc *ProcessDocumentUseCase) classify(ctx context.Context, text string) (domain.Classification, error) {
	classification, err := uc.classifier.Classify(ctx, text)
	if err != nil {
		return domain.Classification{}, fmt.Errorf("classify document: %w", err)
	}
	return classification, nil
}

func (uc *ProcessDocumentUseCase) chunk(_ context.Context, text string) ([]string, error) {
	chunks := uc.chunker.Split(text)
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

func (uc *ProcessDocumentUseCase) persistClassification(ctx context.Context, documentID string, classification domain.Classification) error {
	if err := uc.repo.SaveClassification(ctx, documentID, classification); err != nil {
		return fmt.Errorf("save classification: %w", err)
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

func (uc *ProcessDocumentUseCase) applyClassification(doc *domain.Document, classification domain.Classification) {
	doc.Category = classification.Category
	doc.Subcategory = classification.Subcategory
	doc.Tags = classification.Tags
	doc.Confidence = classification.Confidence
	doc.Summary = classification.Summary
}
