package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
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

type extractorRegistryFake struct {
	text string
	err  error
}

func (f *extractorRegistryFake) ForMimeType(string) ports.TextExtractor {
	return &extractorFake{text: f.text, err: f.err}
}

type metadataExtractorFake struct {
	meta domain.DocumentMetadata
	err  error
}

func (f *metadataExtractorFake) ExtractMetadata(context.Context, *domain.Document, string) (domain.DocumentMetadata, error) {
	if f.err != nil {
		return domain.DocumentMetadata{}, f.err
	}
	return f.meta, nil
}

type queueFake struct {
	publishedEnrichIDs []string
	publishErr         error
}

func (f *queueFake) PublishDocumentIngested(context.Context, string) error { return nil }
func (f *queueFake) SubscribeDocumentIngested(context.Context, func(context.Context, string) error) error {
	return nil
}
func (f *queueFake) PublishDocumentEnrich(_ context.Context, docID string) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	f.publishedEnrichIDs = append(f.publishedEnrichIDs, docID)
	return nil
}
func (f *queueFake) SubscribeDocumentEnrich(context.Context, func(context.Context, string) error) error {
	return nil
}

type graphStoreFake struct{}

func (f *graphStoreFake) UpsertDocument(context.Context, domain.GraphNode) error { return nil }
func (f *graphStoreFake) AddLink(context.Context, string, string, string) error  { return nil }
func (f *graphStoreFake) AddSimilarity(context.Context, string, string, float64) error {
	return nil
}
func (f *graphStoreFake) RemoveSimilarities(context.Context, string) error { return nil }
func (f *graphStoreFake) GetRelated(context.Context, string, int, int) ([]domain.GraphRelation, error) {
	return nil, nil
}
func (f *graphStoreFake) FindByTitle(context.Context, string) ([]domain.GraphNode, error) {
	return nil, nil
}
func (f *graphStoreFake) GetGraph(context.Context, domain.GraphFilter) (*domain.Graph, error) {
	return &domain.Graph{}, nil
}

type chunkerFake struct {
	chunks []string
}

func (f *chunkerFake) Split(string) []string { return f.chunks }

type chunkerRegistryFake struct {
	chunks []string
}

func (f *chunkerRegistryFake) ForSource(string) ports.Chunker {
	return &chunkerFake{chunks: f.chunks}
}

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

func (f *vectorFake) SearchLexical(context.Context, string, int, domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	return nil, nil
}

func (f *vectorFake) UpdateChunksPayload(context.Context, string, string, map[string]any) error { return nil }

func TestProcessByIDSuccess(t *testing.T) {
	repo := &processRepoFake{doc: &domain.Document{ID: "doc-1", Filename: "test.md"}}
	q := &queueFake{}
	uc := NewProcessDocumentUseCase(
		repo,
		&extractorRegistryFake{text: "Some text"},
		&metadataExtractorFake{meta: domain.DocumentMetadata{Category: "general", SourceType: "markdown"}},
		&chunkerRegistryFake{chunks: []string{"a", "b"}},
		&embedderFake{vectors: [][]float32{{1}, {2}}},
		&vectorFake{},
		q,
		&graphStoreFake{},
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
	if len(q.publishedEnrichIDs) != 1 || q.publishedEnrichIDs[0] != "doc-1" {
		t.Fatalf("expected enrich publish for doc-1, got %v", q.publishedEnrichIDs)
	}
}

func TestProcessByIDMarksFailedOnExtractError(t *testing.T) {
	repo := &processRepoFake{doc: &domain.Document{ID: "doc-1"}}
	uc := NewProcessDocumentUseCase(
		repo,
		&extractorRegistryFake{err: errors.New("extract fail")},
		&metadataExtractorFake{},
		&chunkerRegistryFake{chunks: []string{"a"}},
		&embedderFake{vectors: [][]float32{{1}}},
		&vectorFake{},
		&queueFake{},
		&graphStoreFake{},
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
		&extractorRegistryFake{text: "text"},
		&metadataExtractorFake{meta: domain.DocumentMetadata{Category: "general"}},
		&chunkerRegistryFake{chunks: []string{"a", "b"}},
		&embedderFake{vectors: [][]float32{{1}}},
		&vectorFake{},
		&queueFake{},
		&graphStoreFake{},
	)

	err := uc.ProcessByID(context.Background(), "doc-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(repo.statusCalls) != 2 || repo.statusCalls[1].status != domain.StatusFailed {
		t.Fatalf("expected final failed status, got %+v", repo.statusCalls)
	}
}
