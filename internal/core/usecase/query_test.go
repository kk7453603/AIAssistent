package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type queryEmbedderFake struct {
	query string
	err   error
}

func (f *queryEmbedderFake) Embed(context.Context, []string) ([][]float32, error) { return nil, nil }
func (f *queryEmbedderFake) EmbedQuery(_ context.Context, text string) ([]float32, error) {
	f.query = text
	if f.err != nil {
		return nil, f.err
	}
	return []float32{0.1, 0.2}, nil
}

type queryVectorFake struct {
	limit            int
	lexicalLimit     int
	searchCalls      int
	searchLexCalls   int
	err              error
	lexicalErr       error
	semanticResponse []domain.RetrievedChunk
	lexicalResponse  []domain.RetrievedChunk
}

func (f *queryVectorFake) IndexChunks(context.Context, *domain.Document, []string, [][]float32) error {
	return nil
}

func (f *queryVectorFake) Search(_ context.Context, _ []float32, limit int, _ domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	f.searchCalls++
	f.limit = limit
	if f.err != nil {
		return nil, f.err
	}
	if f.semanticResponse != nil {
		return f.semanticResponse, nil
	}
	return []domain.RetrievedChunk{{DocumentID: "doc-1", ChunkIndex: 0, Text: "chunk", Score: 0.9}}, nil
}

func (f *queryVectorFake) SearchLexical(_ context.Context, _ string, limit int, _ domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	f.searchLexCalls++
	f.lexicalLimit = limit
	if f.lexicalErr != nil {
		return nil, f.lexicalErr
	}
	if f.lexicalResponse != nil {
		return f.lexicalResponse, nil
	}
	return []domain.RetrievedChunk{{DocumentID: "doc-2", ChunkIndex: 1, Text: "lex", Score: 0.8}}, nil
}

type queryGeneratorFake struct {
	err error
}

func (f *queryGeneratorFake) GenerateAnswer(context.Context, string, []domain.RetrievedChunk) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "answer", nil
}
func (f *queryGeneratorFake) GenerateFromPrompt(_ context.Context, prompt string) (string, error) {
	return prompt, nil
}

func TestQueryUseCaseAnswerDefaultLimit(t *testing.T) {
	embedder := &queryEmbedderFake{}
	vector := &queryVectorFake{}
	generator := &queryGeneratorFake{}
	uc := NewQueryUseCase(embedder, vector, generator, QueryOptions{})

	answer, err := uc.Answer(context.Background(), "q", 0, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if answer.Text != "answer" {
		t.Fatalf("expected answer text, got %s", answer.Text)
	}
	if vector.limit != 5 {
		t.Fatalf("expected default limit=5, got %d", vector.limit)
	}
	if answer.Retrieval.Mode != domain.RetrievalModeSemantic {
		t.Fatalf("expected semantic retrieval mode, got %s", answer.Retrieval.Mode)
	}
}

func TestQueryUseCaseAnswerEmbedError(t *testing.T) {
	uc := NewQueryUseCase(&queryEmbedderFake{err: errors.New("embed fail")}, &queryVectorFake{}, &queryGeneratorFake{}, QueryOptions{})
	_, err := uc.Answer(context.Background(), "q", 3, domain.SearchFilter{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestQueryUseCaseHybridUsesBothBranchesAndCandidateLimit(t *testing.T) {
	vector := &queryVectorFake{}
	uc := NewQueryUseCase(
		&queryEmbedderFake{},
		vector,
		&queryGeneratorFake{},
		QueryOptions{
			RetrievalMode:    domain.RetrievalModeHybrid,
			HybridCandidates: 30,
			FusionStrategy:   domain.FusionStrategyRRF,
			FusionRRFK:       60,
		},
	)

	answer, err := uc.Answer(context.Background(), "q", 5, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if vector.searchCalls != 1 {
		t.Fatalf("expected semantic search called once, got %d", vector.searchCalls)
	}
	if vector.searchLexCalls != 1 {
		t.Fatalf("expected lexical search called once, got %d", vector.searchLexCalls)
	}
	if vector.limit != 30 || vector.lexicalLimit != 30 {
		t.Fatalf("expected candidate limit 30, got semantic=%d lexical=%d", vector.limit, vector.lexicalLimit)
	}
	if answer.Retrieval.Mode != domain.RetrievalModeHybrid {
		t.Fatalf("expected hybrid mode, got %s", answer.Retrieval.Mode)
	}
}

func TestQueryUseCaseHybridRerankAppliesRerank(t *testing.T) {
	vector := &queryVectorFake{
		semanticResponse: []domain.RetrievedChunk{
			{DocumentID: "doc-1", Filename: "alpha.txt", ChunkIndex: 0, Text: "alpha risk", Score: 1.0},
			{DocumentID: "doc-2", Filename: "beta.txt", ChunkIndex: 0, Text: "beta", Score: 0.9},
		},
		lexicalResponse: []domain.RetrievedChunk{
			{DocumentID: "doc-2", Filename: "beta.txt", ChunkIndex: 0, Text: "beta", Score: 1.0},
			{DocumentID: "doc-1", Filename: "alpha.txt", ChunkIndex: 0, Text: "alpha risk", Score: 0.9},
		},
	}
	uc := NewQueryUseCase(
		&queryEmbedderFake{},
		vector,
		&queryGeneratorFake{},
		QueryOptions{
			RetrievalMode:    domain.RetrievalModeHybridRerank,
			HybridCandidates: 10,
			FusionStrategy:   domain.FusionStrategyRRF,
			FusionRRFK:       60,
			RerankTopN:       2,
		},
	)

	answer, err := uc.Answer(context.Background(), "alpha", 2, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if !answer.Retrieval.RerankApplied {
		t.Fatalf("expected rerank applied")
	}
	if len(answer.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(answer.Sources))
	}
	if answer.Sources[0].DocumentID != "doc-1" {
		t.Fatalf("expected doc-1 first after rerank, got %s", answer.Sources[0].DocumentID)
	}
}

func TestNewQueryUseCaseNormalizesInvalidOptions(t *testing.T) {
	uc := NewQueryUseCase(
		&queryEmbedderFake{},
		&queryVectorFake{},
		&queryGeneratorFake{},
		QueryOptions{
			RetrievalMode:    domain.RetrievalMode("invalid"),
			FusionStrategy:   domain.FusionStrategy("invalid"),
			HybridCandidates: -1,
			FusionRRFK:       0,
			RerankTopN:       -1,
		},
	)

	if uc.retrievalMode != domain.RetrievalModeSemantic {
		t.Fatalf("expected semantic fallback, got %s", uc.retrievalMode)
	}
	if uc.fusionStrategy != domain.FusionStrategyRRF {
		t.Fatalf("expected rrf fallback, got %s", uc.fusionStrategy)
	}
	if uc.hybridCandidates <= 0 || uc.fusionRRFK <= 0 || uc.rerankTopN <= 0 {
		t.Fatalf("expected positive defaults, got hybrid=%d rrfk=%d rerank=%d", uc.hybridCandidates, uc.fusionRRFK, uc.rerankTopN)
	}
}
