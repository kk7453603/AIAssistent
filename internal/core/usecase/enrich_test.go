package usecase

import (
	"context"
	"fmt"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type enrichRepoFake struct {
	doc        *domain.Document
	getErr     error
	savedCls   domain.Classification
	savedClsID string
	saveClsErr error
}

func (f *enrichRepoFake) Create(context.Context, *domain.Document) error { return nil }
func (f *enrichRepoFake) GetByID(context.Context, string) (*domain.Document, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	copyDoc := *f.doc
	return &copyDoc, nil
}
func (f *enrichRepoFake) UpdateStatus(context.Context, string, domain.DocumentStatus, string) error {
	return nil
}
func (f *enrichRepoFake) SaveClassification(_ context.Context, id string, cls domain.Classification) error {
	if f.saveClsErr != nil {
		return f.saveClsErr
	}
	f.savedClsID = id
	f.savedCls = cls
	return nil
}

type enrichVectorFake struct {
	updatedDocID   string
	updatedPayload map[string]any
	updateErr      error
}

func (f *enrichVectorFake) IndexChunks(context.Context, *domain.Document, []string, [][]float32) error {
	return nil
}
func (f *enrichVectorFake) Search(context.Context, []float32, int, domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	return nil, nil
}
func (f *enrichVectorFake) SearchLexical(context.Context, string, int, domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	return nil, nil
}
func (f *enrichVectorFake) UpdateChunksPayload(_ context.Context, docID string, payload map[string]any) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updatedDocID = docID
	f.updatedPayload = payload
	return nil
}

// classifierFake for enrichment tests (the LLM classifier used by enrich)
type enrichClassifierFake struct {
	cls domain.Classification
	err error
}

func (f *enrichClassifierFake) Classify(context.Context, string) (domain.Classification, error) {
	if f.err != nil {
		return domain.Classification{}, f.err
	}
	return f.cls, nil
}

func TestEnrichByID_Success(t *testing.T) {
	doc := &domain.Document{
		ID:       "doc-1",
		Filename: "test.md",
		Category: "existing-cat",
		Tags:     []string{"existing-tag"},
	}
	repo := &enrichRepoFake{doc: doc}
	vectorDB := &enrichVectorFake{}
	cls := &enrichClassifierFake{cls: domain.Classification{
		Category:   "llm-cat",
		Tags:       []string{"llm-tag"},
		Summary:    "LLM summary",
		Confidence: 0.85,
	}}

	uc := NewEnrichDocumentUseCase(repo, &extractorFake{text: "some text"}, cls, vectorDB)

	err := uc.EnrichByID(context.Background(), "doc-1")
	if err != nil {
		t.Fatalf("EnrichByID() error = %v", err)
	}

	// Category should keep deterministic value (existing-cat), not LLM.
	if repo.savedCls.Category != "existing-cat" {
		t.Errorf("category = %q, want %q (deterministic wins)", repo.savedCls.Category, "existing-cat")
	}
	// Tags should be union.
	if len(repo.savedCls.Tags) != 2 {
		t.Errorf("tags = %v, want union of [existing-tag] and [llm-tag]", repo.savedCls.Tags)
	}
	// Summary should be LLM.
	if repo.savedCls.Summary != "LLM summary" {
		t.Errorf("summary = %q, want %q", repo.savedCls.Summary, "LLM summary")
	}
	// Qdrant payload updated.
	if vectorDB.updatedDocID != "doc-1" {
		t.Errorf("qdrant update doc_id = %q, want %q", vectorDB.updatedDocID, "doc-1")
	}
}

func TestEnrichByID_ClassifierError_NoFail(t *testing.T) {
	doc := &domain.Document{ID: "doc-1", Filename: "test.md", Tags: []string{}}
	repo := &enrichRepoFake{doc: doc}
	cls := &enrichClassifierFake{err: fmt.Errorf("LLM timeout")}

	uc := NewEnrichDocumentUseCase(repo, &extractorFake{text: "text"}, cls, &enrichVectorFake{})

	err := uc.EnrichByID(context.Background(), "doc-1")
	// Should NOT return error — enrichment failure is non-fatal.
	if err != nil {
		t.Fatalf("EnrichByID() should not fail on classifier error, got %v", err)
	}
}
