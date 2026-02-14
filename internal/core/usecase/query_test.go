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
	limit int
	err   error
}

func (f *queryVectorFake) IndexChunks(context.Context, *domain.Document, []string, [][]float32) error {
	return nil
}
func (f *queryVectorFake) Search(_ context.Context, _ []float32, limit int, _ domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	f.limit = limit
	if f.err != nil {
		return nil, f.err
	}
	return []domain.RetrievedChunk{{DocumentID: "doc-1", Text: "chunk"}}, nil
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
	uc := NewQueryUseCase(embedder, vector, generator)

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
}

func TestQueryUseCaseAnswerEmbedError(t *testing.T) {
	uc := NewQueryUseCase(&queryEmbedderFake{err: errors.New("embed fail")}, &queryVectorFake{}, &queryGeneratorFake{})
	_, err := uc.Answer(context.Background(), "q", 3, domain.SearchFilter{})
	if err == nil {
		t.Fatalf("expected error")
	}
}
