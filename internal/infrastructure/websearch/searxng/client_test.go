package searxng

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query().Get("q")
		if q == "" {
			t.Error("expected query parameter")
		}
		resp := searchResponse{
			Results: []rawResult{
				{Title: "Result 1", URL: "http://example.com/1", Content: "snippet 1"},
				{Title: "Result 2", URL: "http://example.com/2", Content: "snippet 2"},
				{Title: "Result 3", URL: "http://example.com/3", Content: "snippet 3"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New(server.URL, 5)
	results, err := client.Search(context.Background(), "test query", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Title != "Result 1" {
		t.Fatalf("expected title 'Result 1', got %q", results[0].Title)
	}
	if results[0].URL != "http://example.com/1" {
		t.Fatalf("expected URL 'http://example.com/1', got %q", results[0].URL)
	}
	if results[0].Snippet != "snippet 1" {
		t.Fatalf("expected snippet 'snippet 1', got %q", results[0].Snippet)
	}
}

func TestSearch_LimitTruncation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := searchResponse{
			Results: []rawResult{
				{Title: "R1", URL: "http://1.com", Content: "s1"},
				{Title: "R2", URL: "http://2.com", Content: "s2"},
				{Title: "R3", URL: "http://3.com", Content: "s3"},
				{Title: "R4", URL: "http://4.com", Content: "s4"},
				{Title: "R5", URL: "http://5.com", Content: "s5"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := New(server.URL, 5)
	results, err := client.Search(context.Background(), "test", 2)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results after truncation, got %d", len(results))
	}
}

func TestSearch_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(server.URL, 5)
	_, err := client.Search(context.Background(), "test", 5)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error should mention status code, got: %v", err)
	}
}

func TestSearch_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := New(server.URL, 5)
	_, err := client.Search(context.Background(), "test", 5)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error, got: %v", err)
	}
}

func TestSearch_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(searchResponse{Results: []rawResult{}})
	}))
	defer server.Close()

	client := New(server.URL, 5)
	results, err := client.Search(context.Background(), "test", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestSearch_DefaultLimit(t *testing.T) {
	client := New("http://localhost:9999", 10)
	if client.limit != 10 {
		t.Fatalf("expected limit 10, got %d", client.limit)
	}

	client2 := New("http://localhost:9999", 0)
	if client2.limit != 5 {
		t.Fatalf("expected default limit 5 for 0, got %d", client2.limit)
	}

	client3 := New("http://localhost:9999", -1)
	if client3.limit != 5 {
		t.Fatalf("expected default limit 5 for -1, got %d", client3.limit)
	}
}
