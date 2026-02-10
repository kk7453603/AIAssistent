package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"

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
	if err := uc.repo.UpdateStatus(ctx, documentID, domain.StatusProcessing, ""); err != nil {
		return fmt.Errorf("set status=processing: %w", err)
	}

	doc, err := uc.repo.GetByID(ctx, documentID)
	if err != nil {
		uc.fail(ctx, documentID, err)
		return fmt.Errorf("fetch document by id: %w", err)
	}

	text, err := uc.extractor.Extract(ctx, doc)
	if err != nil {
		uc.fail(ctx, documentID, err)
		return fmt.Errorf("extract text: %w", err)
	}

	if text == "" {
		err = errors.New("empty extracted text")
		uc.fail(ctx, documentID, err)
		return err
	}

	classification, err := uc.classifier.Classify(ctx, text)
	if err != nil {
		uc.fail(ctx, documentID, err)
		return fmt.Errorf("classify document: %w", err)
	}

	chunks := uc.chunker.Split(text)
	if len(chunks) == 0 {
		err = errors.New("chunking produced zero chunks")
		uc.fail(ctx, documentID, err)
		return err
	}

	vectors, err := uc.embedder.Embed(ctx, chunks)
	if err != nil {
		uc.fail(ctx, documentID, err)
		return fmt.Errorf("embed chunks: %w", err)
	}

	if len(vectors) != len(chunks) {
		err = fmt.Errorf("vectors/chunks mismatch: %d/%d", len(vectors), len(chunks))
		uc.fail(ctx, documentID, err)
		return err
	}

	doc.Category = classification.Category
	doc.Subcategory = classification.Subcategory
	doc.Tags = classification.Tags
	doc.Confidence = classification.Confidence
	doc.Summary = classification.Summary

	if err := uc.vectorDB.IndexChunks(ctx, doc, chunks, vectors); err != nil {
		uc.fail(ctx, documentID, err)
		return fmt.Errorf("index chunks in vector db: %w", err)
	}

	if err := uc.repo.SaveClassification(ctx, documentID, classification); err != nil {
		uc.fail(ctx, documentID, err)
		return fmt.Errorf("save classification: %w", err)
	}

	if err := uc.repo.UpdateStatus(ctx, documentID, domain.StatusReady, ""); err != nil {
		return fmt.Errorf("set status=ready: %w", err)
	}

	return nil
}

func (uc *ProcessDocumentUseCase) fail(ctx context.Context, documentID string, processErr error) {
	if err := uc.repo.UpdateStatus(ctx, documentID, domain.StatusFailed, processErr.Error()); err != nil {
		log.Printf("failed to mark document %s as failed: %v", documentID, err)
	}
}
