package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscovery_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("expected /api/tags, got %s", r.URL.Path)
		}
		resp := map[string]any{
			"models": []map[string]any{
				{"name": "llama3.1:8b", "size": 4000000000},
				{"name": "qwen3.5:9b", "size": 6000000000},
				{"name": "qwen-coder:7b", "size": 4500000000},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	d := NewDiscovery(server.URL, server.Client())
	models, err := d.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}
	if models[0].Name != "llama3.1:8b" {
		t.Errorf("models[0].Name = %q, want %q", models[0].Name, "llama3.1:8b")
	}
}

func TestDiscovery_ListModels_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	d := NewDiscovery(server.URL, server.Client())
	_, err := d.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
