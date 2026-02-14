package usecase

import (
	"fmt"
	"sort"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type fusedCandidate struct {
	chunk domain.RetrievedChunk
	score float64
}

func fuseCandidatesRRF(semantic, lexical []domain.RetrievedChunk, rrfK int) []domain.RetrievedChunk {
	if rrfK <= 0 {
		rrfK = 60
	}

	acc := make(map[string]fusedCandidate, len(semantic)+len(lexical))
	addList := func(chunks []domain.RetrievedChunk) {
		for rank, chunk := range chunks {
			key := retrievalChunkKey(chunk)
			candidate := acc[key]
			candidate.chunk = preferRicherChunk(candidate.chunk, chunk)
			candidate.score += 1.0 / float64(rrfK+rank+1)
			acc[key] = candidate
		}
	}

	addList(semantic)
	addList(lexical)

	out := make([]domain.RetrievedChunk, 0, len(acc))
	for _, c := range acc {
		chunk := c.chunk
		chunk.Score = c.score
		out = append(out, chunk)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].DocumentID != out[j].DocumentID {
			return out[i].DocumentID < out[j].DocumentID
		}
		if out[i].ChunkIndex != out[j].ChunkIndex {
			return out[i].ChunkIndex < out[j].ChunkIndex
		}
		return out[i].Filename < out[j].Filename
	})

	return out
}

func trimCandidates(chunks []domain.RetrievedChunk, limit int) []domain.RetrievedChunk {
	if limit <= 0 || len(chunks) <= limit {
		return chunks
	}
	return chunks[:limit]
}

func retrievalChunkKey(chunk domain.RetrievedChunk) string {
	if chunk.DocumentID != "" && chunk.ChunkIndex >= 0 {
		return fmt.Sprintf("%s:%d", chunk.DocumentID, chunk.ChunkIndex)
	}
	return fmt.Sprintf("%s|%s|%s", chunk.DocumentID, chunk.Filename, chunk.Text)
}

func preferRicherChunk(current, candidate domain.RetrievedChunk) domain.RetrievedChunk {
	if current.DocumentID == "" && current.Filename == "" && current.Text == "" {
		return candidate
	}
	if current.Text == "" && candidate.Text != "" {
		current.Text = candidate.Text
	}
	if current.Filename == "" && candidate.Filename != "" {
		current.Filename = candidate.Filename
	}
	if current.Category == "" && candidate.Category != "" {
		current.Category = candidate.Category
	}
	if current.DocumentID == "" && candidate.DocumentID != "" {
		current.DocumentID = candidate.DocumentID
	}
	if current.ChunkIndex < 0 && candidate.ChunkIndex >= 0 {
		current.ChunkIndex = candidate.ChunkIndex
	}
	return current
}
