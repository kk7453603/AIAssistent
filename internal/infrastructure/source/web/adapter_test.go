package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestWebAdapter_SourceType(t *testing.T) {
	a := New(nil)
	if a.SourceType() != "web" {
		t.Fatalf("expected 'web', got %q", a.SourceType())
	}
}

func TestWebAdapter_Ingest_HTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Test Page</title></head><body><p>Hello world</p></body></html>`))
	}))
	defer server.Close()

	a := New(server.Client())
	result, err := a.Ingest(context.Background(), domain.SourceRequest{
		SourceType: "web",
		URL:        server.URL + "/page.html",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if result.SourceType != "web" {
		t.Errorf("SourceType = %q, want %q", result.SourceType, "web")
	}
	if result.Filename != "Test Page" {
		t.Errorf("Filename = %q, want %q", result.Filename, "Test Page")
	}
	if result.Path != server.URL+"/page.html" {
		t.Errorf("Path = %q, want %q", result.Path, server.URL+"/page.html")
	}
	body, _ := io.ReadAll(result.Body)
	if string(body) == "" {
		t.Error("expected non-empty body")
	}
}

func TestWebAdapter_Ingest_Markdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte("# Title\n\nSome content"))
	}))
	defer server.Close()

	a := New(server.Client())
	result, err := a.Ingest(context.Background(), domain.SourceRequest{
		SourceType: "web",
		URL:        server.URL + "/doc.md",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	body, _ := io.ReadAll(result.Body)
	if string(body) != "# Title\n\nSome content" {
		t.Errorf("Body = %q, want markdown passthrough", string(body))
	}
}

func TestWebAdapter_Ingest_EmptyURL(t *testing.T) {
	a := New(nil)
	_, err := a.Ingest(context.Background(), domain.SourceRequest{SourceType: "web"})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestWebAdapter_Ingest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := New(server.Client())
	_, err := a.Ingest(context.Background(), domain.SourceRequest{
		SourceType: "web",
		URL:        server.URL,
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
