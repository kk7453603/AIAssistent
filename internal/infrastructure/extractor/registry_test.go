package extractor

import (
	"context"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type fakeExtractor struct {
	text string
}

func (f *fakeExtractor) Extract(context.Context, *domain.Document) (string, error) {
	return f.text, nil
}

func TestRegistry_ForMimeType_Registered(t *testing.T) {
	fallback := &fakeExtractor{text: "fallback"}
	pdfExt := &fakeExtractor{text: "pdf-text"}

	reg := NewRegistry(fallback)
	reg.Register("application/pdf", pdfExt)

	got := reg.ForMimeType("application/pdf")
	text, _ := got.Extract(context.Background(), &domain.Document{})
	if text != "pdf-text" {
		t.Errorf("expected pdf-text, got %q", text)
	}
}

func TestRegistry_ForMimeType_Fallback(t *testing.T) {
	fallback := &fakeExtractor{text: "fallback"}
	reg := NewRegistry(fallback)

	got := reg.ForMimeType("application/unknown")
	text, _ := got.Extract(context.Background(), &domain.Document{})
	if text != "fallback" {
		t.Errorf("expected fallback, got %q", text)
	}
}

func TestRegistry_ForMimeType_WithCharset(t *testing.T) {
	fallback := &fakeExtractor{text: "fallback"}
	pdfExt := &fakeExtractor{text: "pdf-text"}

	reg := NewRegistry(fallback)
	reg.Register("application/pdf", pdfExt)

	got := reg.ForMimeType("application/pdf; charset=utf-8")
	text, _ := got.Extract(context.Background(), &domain.Document{})
	if text != "pdf-text" {
		t.Errorf("expected pdf-text with charset suffix, got %q", text)
	}
}

var _ ports.ExtractorRegistry = (*Registry)(nil)
