package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestReranker_Rerank_Empty(t *testing.T) {
	reranker := NewReranker(New("http://unused", "", "model"))
	result, err := reranker.Rerank(context.Background(), "query", nil, 5)
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d", len(result))
	}
}

func TestReranker_Rerank_Success(t *testing.T) {
	callIdx := 0
	scores := []float64{3.0, 8.0, 5.0} // normalized: 0.3, 0.8, 0.5
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		score := scores[callIdx%len(scores)]
		callIdx++
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: `{"score":` + jsonFloat(score) + `}`}}},
		})
	}))
	defer server.Close()

	client := New(server.URL, "", "model")
	reranker := NewReranker(client)

	chunks := []domain.RetrievedChunk{
		{Text: "chunk A", Score: 0.9},
		{Text: "chunk B", Score: 0.8},
		{Text: "chunk C", Score: 0.7},
	}

	result, err := reranker.Rerank(context.Background(), "test query", chunks, 3)
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	// After reranking, chunk B (score 8/10=0.8) should be first
	if result[0].Score < result[1].Score {
		t.Fatalf("expected results sorted by score descending, got scores: %.2f, %.2f", result[0].Score, result[1].Score)
	}
}

func TestReranker_Rerank_LLMError_Fallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(server.URL, "", "model")
	reranker := NewReranker(client)

	chunks := []domain.RetrievedChunk{
		{Text: "chunk A", Score: 0.9},
		{Text: "chunk B", Score: 0.8},
	}

	result, err := reranker.Rerank(context.Background(), "test", chunks, 2)
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	// On error, should return original chunks unchanged
	if len(result) != 2 {
		t.Fatalf("expected 2 results on fallback, got %d", len(result))
	}
}

func TestReranker_Rerank_TopN(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: `{"score":5.0}`}}},
		})
	}))
	defer server.Close()

	client := New(server.URL, "", "model")
	reranker := NewReranker(client)

	chunks := []domain.RetrievedChunk{
		{Text: "A", Score: 0.9},
		{Text: "B", Score: 0.8},
		{Text: "C", Score: 0.7},
	}

	result, err := reranker.Rerank(context.Background(), "test", chunks, 2)
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	// Should have 3 results: 2 reranked + 1 original
	if len(result) != 3 {
		t.Fatalf("expected 3 results (2 reranked + 1 tail), got %d", len(result))
	}
}

func jsonFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}
