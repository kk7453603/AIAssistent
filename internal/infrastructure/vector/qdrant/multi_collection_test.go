package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestMultiCollectionStore_IndexChunksRoutesToCorrectCollection(t *testing.T) {
	var mu sync.Mutex
	requestedURLs := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestedURLs = append(requestedURLs, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"completed"}}`))
	}))
	defer server.Close()

	store := NewMultiCollectionStore(server.URL, "docs", []string{"upload", "web"}, []string{"upload", "web"}, Options{})

	doc := &domain.Document{ID: "d1", SourceType: "web", Filename: "page.html", Tags: []string{}}
	err := store.IndexChunks(context.Background(), doc, []string{"chunk1"}, [][]float32{{0.1, 0.2}})
	if err != nil {
		t.Fatalf("IndexChunks() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, u := range requestedURLs {
		if strings.Contains(u, "docs_web") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected request to docs_web collection, got URLs: %v", requestedURLs)
	}
}

func TestMultiCollectionStore_SearchCascadesInOrder(t *testing.T) {
	var mu sync.Mutex
	callOrder := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callOrder = append(callOrder, r.URL.Path)
		mu.Unlock()

		result := map[string]any{
			"result": map[string]any{
				"points": []map[string]any{
					{
						"score": 0.9,
						"payload": map[string]any{
							"doc_id": "d1", "filename": "f.txt", "category": "", "chunk_index": 0, "text": "chunk",
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	store := NewMultiCollectionStore(server.URL, "docs", []string{"obsidian", "upload", "web"}, []string{"obsidian", "upload", "web"}, Options{})

	results, err := store.Search(context.Background(), []float32{0.1, 0.2}, 1, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}

	mu.Lock()
	defer mu.Unlock()
	// With early stop and limit=1, first collection should satisfy
	if len(callOrder) == 0 {
		t.Fatal("expected at least one query call")
	}
	// First query should be to obsidian (highest priority)
	firstQuery := callOrder[0]
	for _, u := range callOrder {
		if strings.Contains(u, "/points/query") {
			firstQuery = u
			break
		}
	}
	if !strings.Contains(firstQuery, "docs_obsidian") {
		t.Errorf("expected first query to docs_obsidian, got %q", firstQuery)
	}
}

func TestMultiCollectionStore_SearchRespectsSourceTypeFilter(t *testing.T) {
	var mu sync.Mutex
	queriedCollections := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if strings.Contains(r.URL.Path, "/points/query") {
			queriedCollections = append(queriedCollections, r.URL.Path)
		}
		mu.Unlock()

		result := map[string]any{
			"result": map[string]any{
				"points": []map[string]any{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	store := NewMultiCollectionStore(server.URL, "docs", []string{"obsidian", "upload", "web"}, []string{"obsidian", "upload", "web"}, Options{})

	// Search only in "web" collection
	_, _ = store.Search(context.Background(), []float32{0.1, 0.2}, 5, domain.SearchFilter{
		SourceTypes: []string{"web"},
	})

	mu.Lock()
	defer mu.Unlock()
	for _, u := range queriedCollections {
		if !strings.Contains(u, "docs_web") {
			t.Errorf("expected only docs_web queries, got %q", u)
		}
	}
}

func TestMultiCollectionStore_IndexChunksUnknownSourceType(t *testing.T) {
	store := NewMultiCollectionStore("http://localhost", "docs", []string{"upload"}, []string{"upload"}, Options{})
	doc := &domain.Document{ID: "d1", SourceType: "unknown", Tags: []string{}}
	err := store.IndexChunks(context.Background(), doc, []string{"chunk"}, [][]float32{{0.1}})
	if err == nil {
		t.Fatal("expected error for unknown source_type")
	}
}
