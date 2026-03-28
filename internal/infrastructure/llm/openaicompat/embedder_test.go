package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedder_Embed_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embedResponse{
			Data: []struct {
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{
				{Index: 0, Embedding: []float32{0.1, 0.2, 0.3}},
			},
		})
	}))
	defer server.Close()

	client := New(server.URL, "", "model")
	embedder := NewEmbedder(client, "embed-model")

	vectors, err := embedder.Embed(context.Background(), []string{"test"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(vectors) != 1 || len(vectors[0]) != 3 {
		t.Fatalf("expected 1 vector of dim 3, got %d vectors", len(vectors))
	}
}

func TestEmbedder_Embed_Empty(t *testing.T) {
	embedder := NewEmbedder(New("http://unused", "", "m"), "m")
	vectors, err := embedder.Embed(context.Background(), nil)
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if vectors != nil {
		t.Fatalf("expected nil for empty input, got %v", vectors)
	}
}

func TestEmbedder_EmbedQuery_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embedResponse{
			Data: []struct {
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{
				{Index: 0, Embedding: []float32{0.5, 0.6}},
			},
		})
	}))
	defer server.Close()

	client := New(server.URL, "", "model")
	embedder := NewEmbedder(client, "embed-model")

	vec, err := embedder.EmbedQuery(context.Background(), "test query")
	if err != nil {
		t.Fatalf("EmbedQuery error: %v", err)
	}
	if len(vec) != 2 {
		t.Fatalf("expected vector dim 2, got %d", len(vec))
	}
}

func TestEmbedder_EmbedQuery_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embedResponse{})
	}))
	defer server.Close()

	client := New(server.URL, "", "model")
	embedder := NewEmbedder(client, "embed-model")

	_, err := embedder.EmbedQuery(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for empty result")
	}
}
