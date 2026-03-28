package upload

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestUploadAdapter_SourceType(t *testing.T) {
	a := New()
	if a.SourceType() != "upload" {
		t.Fatalf("expected 'upload', got %q", a.SourceType())
	}
}

func TestUploadAdapter_Ingest_Success(t *testing.T) {
	a := New()
	req := domain.SourceRequest{
		SourceType: "upload",
		Filename:   "report.txt",
		MimeType:   "text/plain",
		Body:       bytes.NewBufferString("hello world"),
	}

	result, err := a.Ingest(context.Background(), req)
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if result.Filename != "report.txt" {
		t.Errorf("Filename = %q, want %q", result.Filename, "report.txt")
	}
	if result.MimeType != "text/plain" {
		t.Errorf("MimeType = %q, want %q", result.MimeType, "text/plain")
	}
	if result.SourceType != "upload" {
		t.Errorf("SourceType = %q, want %q", result.SourceType, "upload")
	}
	body, _ := io.ReadAll(result.Body)
	if string(body) != "hello world" {
		t.Errorf("Body = %q, want %q", string(body), "hello world")
	}
}

func TestUploadAdapter_Ingest_NilBody(t *testing.T) {
	a := New()
	req := domain.SourceRequest{SourceType: "upload", Filename: "report.txt"}
	_, err := a.Ingest(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nil body")
	}
}
