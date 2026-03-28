package usecase

import (
	"context"
	"log/slog"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type EnrichDocumentUseCase struct {
	repo       ports.DocumentRepository
	extractors ports.ExtractorRegistry
	classifier ports.DocumentClassifier
	vectorDB   ports.VectorStore
}

func NewEnrichDocumentUseCase(
	repo ports.DocumentRepository,
	extractors ports.ExtractorRegistry,
	classifier ports.DocumentClassifier,
	vectorDB ports.VectorStore,
) *EnrichDocumentUseCase {
	return &EnrichDocumentUseCase{
		repo:       repo,
		extractors: extractors,
		classifier: classifier,
		vectorDB:   vectorDB,
	}
}

func (uc *EnrichDocumentUseCase) EnrichByID(ctx context.Context, documentID string) error {
	doc, err := uc.repo.GetByID(ctx, documentID)
	if err != nil {
		slog.Warn("enrich_load_doc_failed", "document_id", documentID, "error", err)
		return nil // non-fatal
	}

	text, err := uc.extractors.ForMimeType(doc.MimeType).Extract(ctx, doc)
	if err != nil {
		slog.Warn("enrich_extract_text_failed", "document_id", documentID, "error", err)
		return nil
	}

	classification, err := uc.classifier.Classify(ctx, text)
	if err != nil {
		slog.Warn("enrich_classify_failed", "document_id", documentID, "error", err)
		return nil
	}

	merged := mergeClassification(doc, classification)

	if err := uc.repo.SaveClassification(ctx, doc.ID, merged); err != nil {
		slog.Warn("enrich_save_classification_failed", "document_id", documentID, "error", err)
		return nil
	}

	payload := map[string]any{
		"category":    merged.Category,
		"subcategory": merged.Subcategory,
		"tags":        merged.Tags,
	}
	if err := uc.vectorDB.UpdateChunksPayload(ctx, doc.ID, doc.SourceType, payload); err != nil {
		slog.Warn("enrich_update_qdrant_failed", "document_id", documentID, "error", err)
	}

	slog.Info("enrich_completed", "document_id", documentID, "confidence", merged.Confidence)
	return nil
}

// mergeClassification merges LLM classification with existing deterministic metadata.
// Deterministic category wins if present; tags are unioned; LLM summary wins if present.
func mergeClassification(doc *domain.Document, llm domain.Classification) domain.Classification {
	merged := domain.Classification{
		Confidence: llm.Confidence,
	}

	// Category: deterministic wins.
	if doc.Category != "" {
		merged.Category = doc.Category
	} else {
		merged.Category = llm.Category
	}

	merged.Subcategory = llm.Subcategory

	// Summary: LLM wins.
	if llm.Summary != "" {
		merged.Summary = llm.Summary
	} else {
		merged.Summary = doc.Summary
	}

	// Tags: union.
	tagSet := make(map[string]struct{})
	for _, t := range doc.Tags {
		tagSet[t] = struct{}{}
	}
	for _, t := range llm.Tags {
		tagSet[t] = struct{}{}
	}
	merged.Tags = make([]string, 0, len(tagSet))
	for t := range tagSet {
		merged.Tags = append(merged.Tags, t)
	}

	return merged
}
