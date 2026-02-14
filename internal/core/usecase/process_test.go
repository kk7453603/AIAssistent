package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type statusCall struct {
	status domain.DocumentStatus
	errMsg string
}

type processRepoFake struct {
	doc              *domain.Document
	getErr           error
	saveErr          error
	statusErr        error
	failStatusErr    error
	statusCalls      []statusCall
	classification   domain.Classification
	classificationID string
}

func (f *processRepoFake) Create(context.Context, *domain.Document) error { return nil }

func (f *processRepoFake) GetByID(context.Context, string) (*domain.Document, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	copyDoc := *f.doc
	return &copyDoc, nil
}

func (f *processRepoFake) UpdateStatus(_ context.Context, _ string, status domain.DocumentStatus, errMessage string) error {
	f.statusCalls = append(f.statusCalls, statusCall{status: status, errMsg: errMessage})
	if status == domain.StatusFailed && f.failStatusErr != nil {
		return f.failStatusErr
	}
	if f.statusErr != nil {
		return f.statusErr
	}
	return nil
}

func (f *processRepoFake) SaveClassification(_ context.Context, id string, cls domain.Classification) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.classificationID = id
	f.classification = cls
	return nil
}

type extractorFake struct {
	text string
	err  error
}

func (f *extractorFake) Extract(context.Context, *domain.Document) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.text, nil
}

type classifierFake struct {
	cls domain.Classification
	err error
}

func (f *classifierFake) Classify(context.Context, string) (domain.Classification, error) {
	if f.err != nil {
		return domain.Classification{}, f.err
	}
	return f.cls, nil
}

type chunkerFake struct {
	chunks []string
}

func (f *chunkerFake) Split(string) []string { return f.chunks }

type embedderFake struct {
	vectors [][]float32
	err     error
}

func (f *embedderFake) Embed(context.Context, []string) ([][]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vectors, nil
}

func (f *embedderFake) EmbedQuery(context.Context, string) ([]float32, error) { return nil, nil }

type vectorFake struct {
	err error
}

func (f *vectorFake) IndexChunks(context.Context, *domain.Document, []string, [][]float32) error {
	return f.err
}

func (f *vectorFake) Search(context.Context, []float32, int, domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	return nil, nil
}

func TestProcessByIDSuccess(t *testing.T) {
	repo := &processRepoFake{doc: &domain.Document{ID: "doc-1"}}
	uc := NewProcessDocumentUseCase(
		repo,
		&extractorFake{text: "text"},
		&classifierFake{cls: domain.Classification{Category: "general"}},
		&chunkerFake{chunks: []string{"a", "b"}},
		&embedderFake{vectors: [][]float32{{1}, {2}}},
		&vectorFake{},
	)

	if err := uc.ProcessByID(context.Background(), "doc-1"); err != nil {
		t.Fatalf("ProcessByID() error = %v", err)
	}
	if len(repo.statusCalls) != 2 {
		t.Fatalf("expected 2 status calls, got %d", len(repo.statusCalls))
	}
	if repo.statusCalls[0].status != domain.StatusProcessing || repo.statusCalls[1].status != domain.StatusReady {
		t.Fatalf("unexpected status sequence: %+v", repo.statusCalls)
	}
	if repo.classificationID != "doc-1" {
		t.Fatalf("expected classification save for doc-1, got %s", repo.classificationID)
	}
}

func TestProcessByIDMarksFailedOnExtractError(t *testing.T) {
	repo := &processRepoFake{doc: &domain.Document{ID: "doc-1"}}
	uc := NewProcessDocumentUseCase(
		repo,
		&extractorFake{err: errors.New("extract fail")},
		&classifierFake{},
		&chunkerFake{chunks: []string{"a"}},
		&embedderFake{vectors: [][]float32{{1}}},
		&vectorFake{},
	)

	err := uc.ProcessByID(context.Background(), "doc-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(repo.statusCalls) != 2 {
		t.Fatalf("expected processing + failed status updates, got %d", len(repo.statusCalls))
	}
	if repo.statusCalls[1].status != domain.StatusFailed {
		t.Fatalf("expected failed status, got %+v", repo.statusCalls[1])
	}
}

func TestProcessByIDMarksFailedOnVectorMismatch(t *testing.T) {
	repo := &processRepoFake{doc: &domain.Document{ID: "doc-1"}}
	uc := NewProcessDocumentUseCase(
		repo,
		&extractorFake{text: "text"},
		&classifierFake{cls: domain.Classification{Category: "general"}},
		&chunkerFake{chunks: []string{"a", "b"}},
		&embedderFake{vectors: [][]float32{{1}}},
		&vectorFake{},
	)

	err := uc.ProcessByID(context.Background(), "doc-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(repo.statusCalls) != 2 || repo.statusCalls[1].status != domain.StatusFailed {
		t.Fatalf("expected final failed status, got %+v", repo.statusCalls)
	}
}
