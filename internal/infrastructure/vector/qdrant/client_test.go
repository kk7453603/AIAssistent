package qdrant

import (
	"context"
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
