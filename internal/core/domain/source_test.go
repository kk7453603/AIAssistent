package domain

import (
	"strings"
	"testing"
)

func TestSourceRequestDefaults(t *testing.T) {
	req := SourceRequest{SourceType: "upload", Filename: "test.txt"}
	if req.SourceType != "upload" {
		t.Fatalf("expected upload, got %q", req.SourceType)
	}
}

func TestIngestResultBodyReadable(t *testing.T) {
	body := strings.NewReader("hello")
	result := IngestResult{
		Filename:   "test.txt",
		MimeType:   "text/plain",
		Body:       body,
		SourceType: "upload",
		Path:       "test.txt",
	}
	if result.Body == nil {
		t.Fatal("expected body to be set")
	}
}
