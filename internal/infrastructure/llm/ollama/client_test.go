package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestGeneratorBuildsContextPrompt(t *testing.T) {
	var capturedPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		capturedPrompt, _ = payload["prompt"].(string)
		_, _ = w.Write([]byte(`{"response":"ok"}`))
	}))
	defer server.Close()

	client := New(server.URL, "gen", "embed")
	gen := NewGenerator(client)
	_, err := gen.GenerateAnswer(context.Background(), "question?", []domain.RetrievedChunk{{Filename: "a.txt", Category: "general", Text: "chunk text", Score: 0.99}})
	if err != nil {
		t.Fatalf("GenerateAnswer() error = %v", err)
	}
	if !strings.Contains(capturedPrompt, "question?") || !strings.Contains(capturedPrompt, "chunk text") {
		t.Fatalf("unexpected prompt: %s", capturedPrompt)
	}
}

func TestEmbedIncludesHTTPBodyInError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	client := New(server.URL, "gen", "embed")
	embedder := NewEmbedder(client)
	_, err := embedder.Embed(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "model unavailable") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestOllamaChatWithTools_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": {
				"role": "assistant",
				"content": "",
				"tool_calls": [
					{
						"function": {
							"name": "knowledge_search",
							"arguments": {"query": "golang concurrency"}
						}
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client := New(server.URL, "gen", "embed")
	gen := NewGenerator(client)

	messages := []domain.ChatMessage{
		{Role: "user", Content: "search for golang concurrency"},
	}
	tools := []domain.ToolSchema{
		{
			Type: "function",
			Function: domain.FunctionSchema{
				Name:        "knowledge_search",
				Description: "Search knowledge base",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}},
			},
		},
	}

	result, err := gen.ChatWithTools(context.Background(), messages, tools)
	if err != nil {
		t.Fatalf("ChatWithTools() error = %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	tc := result.ToolCalls[0]
	if tc.Function.Name != "knowledge_search" {
		t.Errorf("expected tool name=knowledge_search, got %q", tc.Function.Name)
	}
	query, ok := tc.Function.Arguments["query"].(string)
	if !ok || query != "golang concurrency" {
		t.Errorf("expected query=golang concurrency, got %v", tc.Function.Arguments["query"])
	}
	if result.Content != "" {
		t.Errorf("expected empty content for tool call response, got %q", result.Content)
	}
}

func TestOllamaChatWithTools_NoToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": {
				"role": "assistant",
				"content": "Here is the answer to your question."
			}
		}`))
	}))
	defer server.Close()

	client := New(server.URL, "gen", "embed")
	gen := NewGenerator(client)

	messages := []domain.ChatMessage{
		{Role: "user", Content: "what is 2+2?"},
	}

	result, err := gen.ChatWithTools(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("ChatWithTools() error = %v", err)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected 0 tool calls, got %d", len(result.ToolCalls))
	}
	if result.Content != "Here is the answer to your question." {
		t.Errorf("expected content text, got %q", result.Content)
	}
}

func TestOllamaReranker_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		// Return a score JSON for reranking
		_, _ = w.Write([]byte(`{"response":"{\"score\": 8}"}`))
	}))
	defer server.Close()

	client := New(server.URL, "gen", "embed")
	reranker := NewReranker(client)

	chunks := []domain.RetrievedChunk{
		{DocumentID: "d1", Filename: "a.txt", Text: "first chunk", Score: 0.5},
		{DocumentID: "d2", Filename: "b.txt", Text: "second chunk", Score: 0.6},
	}

	results, err := reranker.Rerank(context.Background(), "test query", chunks, 2)
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// All chunks should have score 0.8 (8/10) since mock returns same score
	for i, r := range results {
		if r.Score != 0.8 {
			t.Errorf("result[%d]: expected score=0.8, got %f", i, r.Score)
		}
	}
}

func TestOllamaReranker_Empty(t *testing.T) {
	client := New("http://localhost:11434", "gen", "embed")
	reranker := NewReranker(client)

	results, err := reranker.Rerank(context.Background(), "query", nil, 5)
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty input, got %d", len(results))
	}
}

func TestGenerateJSONFromPromptUsesPlannerModelAndJSONFormat(t *testing.T) {
	var capturedModel string
	var capturedFormat string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		capturedModel, _ = payload["model"].(string)
		capturedFormat, _ = payload["format"].(string)
		_, _ = w.Write([]byte(`{"response":"{\"type\":\"final\",\"answer\":\"ok\"}"}`))
	}))
	defer server.Close()

	client := NewWithOptions(server.URL, "gen-model", "embed-model", Options{PlannerModel: "planner-model"})
	gen := NewGenerator(client)
	out, err := gen.GenerateJSONFromPrompt(context.Background(), "return json")
	if err != nil {
		t.Fatalf("GenerateJSONFromPrompt() error = %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty response")
	}
	if capturedModel != "planner-model" {
		t.Fatalf("expected planner model, got %q", capturedModel)
	}
	if capturedFormat != "json" {
		t.Fatalf("expected format=json, got %q", capturedFormat)
	}
}
