package usecase

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestFuseCandidatesRRFDeduplicatesByChunkKey(t *testing.T) {
	semantic := []domain.RetrievedChunk{
		{DocumentID: "doc-1", ChunkIndex: 0, Filename: "a.txt", Text: "a", Score: 0.9},
		{DocumentID: "doc-2", ChunkIndex: 0, Filename: "b.txt", Text: "b", Score: 0.8},
	}
	lexical := []domain.RetrievedChunk{
		{DocumentID: "doc-2", ChunkIndex: 0, Filename: "b.txt", Text: "b", Score: 1.0},
		{DocumentID: "doc-3", ChunkIndex: 1, Filename: "c.txt", Text: "c", Score: 0.7},
	}

	fused := fuseCandidatesRRF(semantic, lexical, 60)
	if len(fused) != 3 {
		t.Fatalf("expected 3 fused candidates, got %d", len(fused))
	}
	if fused[0].DocumentID != "doc-2" {
		t.Fatalf("expected doc-2 first after RRF fusion, got %s", fused[0].DocumentID)
	}
}

func TestFuseCandidatesRRFTieBreakStable(t *testing.T) {
	semantic := []domain.RetrievedChunk{{DocumentID: "doc-b", ChunkIndex: 0, Filename: "b.txt", Text: "b"}}
	lexical := []domain.RetrievedChunk{{DocumentID: "doc-a", ChunkIndex: 0, Filename: "a.txt", Text: "a"}}

	fused := fuseCandidatesRRF(semantic, lexical, 1000)
	if len(fused) != 2 {
		t.Fatalf("expected 2 fused candidates, got %d", len(fused))
	}
	if fused[0].DocumentID != "doc-a" {
		t.Fatalf("expected tie-break by document id, got first=%s", fused[0].DocumentID)
	}
}
