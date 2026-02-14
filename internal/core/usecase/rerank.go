package usecase

import (
	"sort"
	"strings"
	"unicode"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func rerankHybridCandidates(question string, fused []domain.RetrievedChunk, topN int) []domain.RetrievedChunk {
	if len(fused) == 0 {
		return fused
	}
	if topN <= 0 || topN > len(fused) {
		topN = len(fused)
	}

	head := make([]domain.RetrievedChunk, topN)
	copy(head, fused[:topN])
	queryTokens := toTokenSet(question)

	minScore := head[0].Score
	maxScore := head[0].Score
	for _, chunk := range head[1:] {
		if chunk.Score < minScore {
			minScore = chunk.Score
		}
		if chunk.Score > maxScore {
			maxScore = chunk.Score
		}
	}

	rangeScore := maxScore - minScore
	normalize := func(v float64) float64 {
		if rangeScore <= 0 {
			if v > 0 {
				return 1
			}
			return 0
		}
		return (v - minScore) / rangeScore
	}

	for i := range head {
		normalizedFused := normalize(head[i].Score)
		overlap := tokenOverlap(queryTokens, toTokenSet(head[i].Text))
		filenameBoost := filenameTokenHit(queryTokens, head[i].Filename)
		head[i].Score = 0.60*normalizedFused + 0.30*overlap + 0.10*filenameBoost
	}

	sort.SliceStable(head, func(i, j int) bool {
		if head[i].Score != head[j].Score {
			return head[i].Score > head[j].Score
		}
		if head[i].DocumentID != head[j].DocumentID {
			return head[i].DocumentID < head[j].DocumentID
		}
		if head[i].ChunkIndex != head[j].ChunkIndex {
			return head[i].ChunkIndex < head[j].ChunkIndex
		}
		return head[i].Filename < head[j].Filename
	})

	if topN == len(fused) {
		return head
	}

	out := make([]domain.RetrievedChunk, 0, len(fused))
	out = append(out, head...)
	out = append(out, fused[topN:]...)
	return out
}

func tokenOverlap(query, chunk map[string]struct{}) float64 {
	if len(query) == 0 || len(chunk) == 0 {
		return 0
	}
	matches := 0
	for token := range query {
		if _, ok := chunk[token]; ok {
			matches++
		}
	}
	return float64(matches) / float64(len(query))
}

func filenameTokenHit(query map[string]struct{}, filename string) float64 {
	if len(query) == 0 || filename == "" {
		return 0
	}
	filename = strings.ToLower(filename)
	for token := range query {
		if token == "" {
			continue
		}
		if strings.Contains(filename, token) {
			return 1
		}
	}
	return 0
}

func toTokenSet(s string) map[string]struct{} {
	tokens := splitAlphaNumLower(s)
	out := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		out[token] = struct{}{}
	}
	return out
}

func splitAlphaNumLower(s string) []string {
	if s == "" {
		return nil
	}

	tokens := make([]string, 0, 16)
	var b strings.Builder
	for _, r := range s {
		r = unicode.ToLower(r)
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			tokens = append(tokens, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens
}
