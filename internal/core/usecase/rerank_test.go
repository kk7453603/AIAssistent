package usecase

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestRerankHybridCandidatesChangesOrder(t *testing.T) {
	fused := []domain.RetrievedChunk{
		{DocumentID: "doc-1", Filename: "generic.txt", ChunkIndex: 0, Text: "unrelated text", Score: 0.95},
		{DocumentID: "doc-2", Filename: "risk_report.txt", ChunkIndex: 0, Text: "risk level high", Score: 1.0},
	}

	reranked := rerankHybridCandidates("risk report", fused, 2)
	if len(reranked) != 2 {
		t.Fatalf("expected 2 reranked candidates, got %d", len(reranked))
	}
	if reranked[0].DocumentID != "doc-2" {
		t.Fatalf("expected doc-2 first after rerank, got %s", reranked[0].DocumentID)
	}
}

func TestRerankHybridCandidatesHandlesEmptyInput(t *testing.T) {
	out := rerankHybridCandidates("risk", nil, 10)
	if len(out) != 0 {
		t.Fatalf("expected empty output, got %d", len(out))
	}
}
