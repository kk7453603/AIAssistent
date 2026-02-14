package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestIndexChunksEnsuresCollectionOncePerVectorSize(t *testing.T) {
	var ensureCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/collections/docs":
			atomic.AddInt32(&ensureCalls, 1)
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut && r.URL.Path == "/collections/docs/points":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL, "docs")
	doc := &domain.Document{ID: "doc-1", Filename: "a.txt"}
	chunks := []string{"a", "b"}
	vectors := [][]float32{{0.1, 0.2}, {0.3, 0.4}}

	if err := client.IndexChunks(context.Background(), doc, chunks, vectors); err != nil {
		t.Fatalf("first IndexChunks() error = %v", err)
	}
	if err := client.IndexChunks(context.Background(), doc, chunks, vectors); err != nil {
		t.Fatalf("second IndexChunks() error = %v", err)
	}
	if got := atomic.LoadInt32(&ensureCalls); got != 1 {
		t.Fatalf("expected ensure collection called once, got %d", got)
	}
}

func TestEnsureCollectionIncludesResponseBodyInError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/collections/docs" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := New(server.URL, "docs")
	doc := &domain.Document{ID: "doc-1", Filename: "a.txt"}
	err := client.IndexChunks(context.Background(), doc, []string{"a"}, [][]float32{{0.1, 0.2}})
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "boom") {
		t.Fatalf("expected error to include body, got %v", err)
	}
}

func TestIndexChunksUpsertContainsDenseAndSparseVectors(t *testing.T) {
	var upsertBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/collections/docs":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut && r.URL.Path == "/collections/docs/points":
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&upsertBody); err != nil {
				t.Fatalf("decode upsert body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL, "docs")
	doc := &domain.Document{ID: "doc-1", Filename: "doc_0001.txt"}
	err := client.IndexChunks(context.Background(), doc, []string{"risk level high"}, [][]float32{{0.1, 0.2}})
	if err != nil {
		t.Fatalf("IndexChunks() error = %v", err)
	}

	points, ok := upsertBody["points"].([]any)
	if !ok || len(points) != 1 {
		t.Fatalf("unexpected points payload: %#v", upsertBody["points"])
	}
	point, ok := points[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected point shape: %#v", points[0])
	}
	vector, ok := point["vector"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected vector shape: %#v", point["vector"])
	}
	if _, ok := vector["dense"]; !ok {
		t.Fatalf("expected dense vector in upsert body: %#v", vector)
	}
	sparse, ok := vector["text"].(map[string]any)
	if !ok {
		t.Fatalf("expected sparse vector in upsert body: %#v", vector["text"])
	}
	if _, ok := sparse["indices"]; !ok {
		t.Fatalf("expected sparse.indices in upsert body: %#v", sparse)
	}
	if _, ok := sparse["values"]; !ok {
		t.Fatalf("expected sparse.values in upsert body: %#v", sparse)
	}
}

func TestSearchLexicalUsesSparseQueryAndVectorName(t *testing.T) {
	var queryBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/collections/docs/points/query":
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&queryBody); err != nil {
				t.Fatalf("decode query body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"points":[]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL, "docs")
	_, err := client.SearchLexical(context.Background(), "risk for doc_0001", 5, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("SearchLexical() error = %v", err)
	}

	if got, _ := queryBody["using"].(string); got != "text" {
		t.Fatalf("expected using=text, got %#v", queryBody["using"])
	}
	query, ok := queryBody["query"].(map[string]any)
	if !ok {
		t.Fatalf("expected sparse query object, got %#v", queryBody["query"])
	}
	if _, ok := query["indices"]; !ok {
		t.Fatalf("expected sparse query indices, got %#v", query)
	}
	if _, ok := query["values"]; !ok {
		t.Fatalf("expected sparse query values, got %#v", query)
	}
}
