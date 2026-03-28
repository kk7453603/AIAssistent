package obsidian

import (
	"context"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestNew_ReturnsNonNil(t *testing.T) {
	a := New()
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
}

func TestSourceType(t *testing.T) {
	a := New()
	if got := a.SourceType(); got != "obsidian" {
		t.Fatalf("SourceType() = %q, want %q", got, "obsidian")
	}
}

func TestIngest_ReturnsNotImplemented(t *testing.T) {
	a := New()
	result, err := a.Ingest(context.Background(), domain.SourceRequest{})
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected 'not implemented' error, got: %v", err)
	}
}
