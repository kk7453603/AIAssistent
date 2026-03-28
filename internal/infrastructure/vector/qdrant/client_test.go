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
			defer func() { _ = r.Body.Close() }()
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

func TestSearch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/points/query"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"result": {
					"points": [
						{
							"score": 0.95,
							"payload": {
								"doc_id": "doc-42",
								"filename": "notes.md",
								"category": "personal",
								"chunk_index": 2,
								"text": "relevant chunk text"
							}
						},
						{
							"score": 0.80,
							"payload": {
								"doc_id": "doc-43",
								"filename": "readme.txt",
								"category": "tech",
								"chunk_index": 0,
								"text": "another chunk"
							}
						}
					]
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL, "docs")
	results, err := client.Search(context.Background(), []float32{0.1, 0.2, 0.3}, 5, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	r0 := results[0]
	if r0.DocumentID != "doc-42" {
		t.Errorf("expected doc_id=doc-42, got %q", r0.DocumentID)
	}
	if r0.Filename != "notes.md" {
		t.Errorf("expected filename=notes.md, got %q", r0.Filename)
	}
	if r0.Category != "personal" {
		t.Errorf("expected category=personal, got %q", r0.Category)
	}
	if r0.ChunkIndex != 2 {
		t.Errorf("expected chunk_index=2, got %d", r0.ChunkIndex)
	}
	if r0.Text != "relevant chunk text" {
		t.Errorf("expected text='relevant chunk text', got %q", r0.Text)
	}
	if r0.Score != 0.95 {
		t.Errorf("expected score=0.95, got %f", r0.Score)
	}

	r1 := results[1]
	if r1.DocumentID != "doc-43" {
		t.Errorf("expected doc_id=doc-43, got %q", r1.DocumentID)
	}
	if r1.Score != 0.80 {
		t.Errorf("expected score=0.80, got %f", r1.Score)
	}
}

func TestSearch_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/points/query") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"points":[]}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := New(server.URL, "docs")
	results, err := client.Search(context.Background(), []float32{0.1, 0.2}, 5, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestSearchLexical_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/points/query") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"result": {
					"points": [
						{
							"score": 0.75,
							"payload": {
								"doc_id": "doc-10",
								"filename": "report.pdf",
								"category": "work",
								"chunk_index": 1,
								"text": "matched text content"
							}
						}
					]
				}
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := New(server.URL, "docs")
	results, err := client.SearchLexical(context.Background(), "matched text", 5, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("SearchLexical() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].DocumentID != "doc-10" {
		t.Errorf("expected doc_id=doc-10, got %q", results[0].DocumentID)
	}
	if results[0].Text != "matched text content" {
		t.Errorf("expected text='matched text content', got %q", results[0].Text)
	}
	if results[0].Score != 0.75 {
		t.Errorf("expected score=0.75, got %f", results[0].Score)
	}
}

func TestUpdateChunksPayload_Success(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/points/payload") {
			defer func() { _ = r.Body.Close() }()
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				t.Fatalf("decode set_payload body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := New(server.URL, "docs")
	err := client.UpdateChunksPayload(context.Background(), "doc-99", "upload", map[string]any{
		"category": "updated",
		"tags":     []string{"new-tag"},
	})
	if err != nil {
		t.Fatalf("UpdateChunksPayload() error = %v", err)
	}

	// Verify the request body structure
	payload, ok := capturedBody["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload in request body, got %#v", capturedBody)
	}
	if payload["category"] != "updated" {
		t.Errorf("expected category=updated in payload, got %v", payload["category"])
	}

	filter, ok := capturedBody["filter"].(map[string]any)
	if !ok {
		t.Fatalf("expected filter in request body, got %#v", capturedBody)
	}
	must, ok := filter["must"].([]any)
	if !ok || len(must) == 0 {
		t.Fatalf("expected must filter with doc_id condition, got %#v", filter)
	}
}

func TestSearchLexicalUsesSparseQueryAndVectorName(t *testing.T) {
	var queryBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/collections/docs/points/query":
			defer func() { _ = r.Body.Close() }()
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
