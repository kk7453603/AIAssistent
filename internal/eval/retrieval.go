package eval

import (
	"math"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// PrecisionAtK computes precision@K: fraction of retrieved chunks whose filename
// appears in the expected filenames list.
func PrecisionAtK(retrieved []domain.RetrievedChunk, expected []string) float64 {
	if len(retrieved) == 0 {
		return 0
	}
	expectedSet := toSet(expected)
	relevant := 0
	for _, c := range retrieved {
		if expectedSet[c.Filename] {
			relevant++
		}
	}
	return float64(relevant) / float64(len(retrieved))
}

// RecallAtK computes recall@K: fraction of expected filenames that appear in
// the retrieved chunks.
func RecallAtK(retrieved []domain.RetrievedChunk, expected []string) float64 {
	if len(expected) == 0 {
		return 0
	}
	found := make(map[string]bool)
	for _, c := range retrieved {
		found[c.Filename] = true
	}
	hits := 0
	for _, e := range expected {
		if found[e] {
			hits++
		}
	}
	return float64(hits) / float64(len(expected))
}

// MRR computes Mean Reciprocal Rank: 1/rank of the first relevant result.
func MRR(retrieved []domain.RetrievedChunk, expected []string) float64 {
	expectedSet := toSet(expected)
	for i, c := range retrieved {
		if expectedSet[c.Filename] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// NDCG computes Normalized Discounted Cumulative Gain.
// Relevance is binary: 1 if the chunk filename is in expected, 0 otherwise.
func NDCG(retrieved []domain.RetrievedChunk, expected []string) float64 {
	if len(retrieved) == 0 || len(expected) == 0 {
		return 0
	}
	expectedSet := toSet(expected)

	// DCG: sum of rel_i / log2(i+2) for i starting at 0
	dcg := 0.0
	for i, c := range retrieved {
		if expectedSet[c.Filename] {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}

	// Ideal DCG: all relevant documents at the top
	idealCount := len(expected)
	if idealCount > len(retrieved) {
		idealCount = len(retrieved)
	}
	idcg := 0.0
	for i := range idealCount {
		idcg += 1.0 / math.Log2(float64(i+2))
	}

	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}
