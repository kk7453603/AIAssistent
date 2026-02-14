package config

import "testing"

func TestLoadIncludesAdvancedRetrievalDefaults(t *testing.T) {
	t.Setenv("RAG_RETRIEVAL_MODE", "")
	t.Setenv("RAG_HYBRID_CANDIDATES", "")
	t.Setenv("RAG_FUSION_STRATEGY", "")
	t.Setenv("RAG_FUSION_RRF_K", "")
	t.Setenv("RAG_RERANK_TOP_N", "")

	cfg := Load()
	if cfg.RAGRetrievalMode != "semantic" {
		t.Fatalf("expected default retrieval mode semantic, got %q", cfg.RAGRetrievalMode)
	}
	if cfg.RAGHybridCandidates != 30 {
		t.Fatalf("expected default hybrid candidates 30, got %d", cfg.RAGHybridCandidates)
	}
	if cfg.RAGFusionStrategy != "rrf" {
		t.Fatalf("expected default fusion strategy rrf, got %q", cfg.RAGFusionStrategy)
	}
	if cfg.RAGFusionRRFK != 60 {
		t.Fatalf("expected default fusion rrf k 60, got %d", cfg.RAGFusionRRFK)
	}
	if cfg.RAGRerankTopN != 20 {
		t.Fatalf("expected default rerank top n 20, got %d", cfg.RAGRerankTopN)
	}
}

func TestLoadParsesAdvancedRetrievalOverrides(t *testing.T) {
	t.Setenv("RAG_RETRIEVAL_MODE", "hybrid+rerank")
	t.Setenv("RAG_HYBRID_CANDIDATES", "40")
	t.Setenv("RAG_FUSION_STRATEGY", "rrf")
	t.Setenv("RAG_FUSION_RRF_K", "75")
	t.Setenv("RAG_RERANK_TOP_N", "12")

	cfg := Load()
	if cfg.RAGRetrievalMode != "hybrid+rerank" {
		t.Fatalf("expected retrieval mode override, got %q", cfg.RAGRetrievalMode)
	}
	if cfg.RAGHybridCandidates != 40 {
		t.Fatalf("expected hybrid candidates 40, got %d", cfg.RAGHybridCandidates)
	}
	if cfg.RAGFusionRRFK != 75 {
		t.Fatalf("expected fusion rrf k 75, got %d", cfg.RAGFusionRRFK)
	}
	if cfg.RAGRerankTopN != 12 {
		t.Fatalf("expected rerank top n 12, got %d", cfg.RAGRerankTopN)
	}
}
