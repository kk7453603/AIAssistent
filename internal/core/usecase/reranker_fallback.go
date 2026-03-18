package usecase

import (
	"context"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// FallbackReranker implements ports.Reranker using the existing token-overlap scoring.
type FallbackReranker struct{}

func NewFallbackReranker() *FallbackReranker {
	return &FallbackReranker{}
}

func (r *FallbackReranker) Rerank(_ context.Context, query string, chunks []domain.RetrievedChunk, topN int) ([]domain.RetrievedChunk, error) {
	return rerankHybridCandidates(query, chunks, topN), nil
}
