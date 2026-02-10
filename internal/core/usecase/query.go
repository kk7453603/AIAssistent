package usecase

import (
	"context"
	"fmt"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type QueryUseCase struct {
	embedder  ports.Embedder
	vectorDB  ports.VectorStore
	generator ports.AnswerGenerator
}

func NewQueryUseCase(
	embedder ports.Embedder,
	vectorDB ports.VectorStore,
	generator ports.AnswerGenerator,
) *QueryUseCase {
	return &QueryUseCase{
		embedder:  embedder,
		vectorDB:  vectorDB,
		generator: generator,
	}
}

func (uc *QueryUseCase) Answer(
	ctx context.Context,
	question string,
	limit int,
	filter domain.SearchFilter,
) (*domain.Answer, error) {
	if limit <= 0 {
		limit = 5
	}

	queryVector, err := uc.embedder.EmbedQuery(ctx, question)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	chunks, err := uc.vectorDB.Search(ctx, queryVector, limit, filter)
	if err != nil {
		return nil, fmt.Errorf("search vector db: %w", err)
	}

	answerText, err := uc.generator.GenerateAnswer(ctx, question, chunks)
	if err != nil {
		return nil, fmt.Errorf("generate answer: %w", err)
	}

	return &domain.Answer{
		Text:    answerText,
		Sources: chunks,
	}, nil
}
