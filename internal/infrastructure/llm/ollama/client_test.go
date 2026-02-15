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
