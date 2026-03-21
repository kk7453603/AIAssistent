package fallback

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/openaicompat"
)

// --- mock generator ---

type mockGenerator struct {
	answer string
	err    error
}

func (m *mockGenerator) GenerateAnswer(_ context.Context, _ string, _ []domain.RetrievedChunk) (string, error) {
	return m.answer, m.err
}
func (m *mockGenerator) GenerateFromPrompt(_ context.Context, _ string) (string, error) {
	return m.answer, m.err
}
func (m *mockGenerator) GenerateJSONFromPrompt(_ context.Context, _ string) (string, error) {
	return m.answer, m.err
}

func (m *mockGenerator) ChatWithTools(_ context.Context, _ []domain.ChatMessage, _ []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	return &domain.ChatToolsResult{Content: m.answer}, m.err
}

// --- mock classifier ---

type mockClassifier struct {
	cls domain.Classification
	err error
}

func (m *mockClassifier) Classify(_ context.Context, _ string) (domain.Classification, error) {
	return m.cls, m.err
}

func TestGenerator_FallbackOnRetryable(t *testing.T) {
	retryableErr := &openaicompat.ProviderError{StatusCode: 429, Body: "rate limited", Operation: "chat"}
	primary := &mockGenerator{err: retryableErr}
	fb := &mockGenerator{answer: "fallback answer"}
	logger := slog.Default()

	g := NewGenerator(primary, fb, logger)
	ans, err := g.GenerateAnswer(context.Background(), "q", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ans != "fallback answer" {
		t.Fatalf("expected fallback answer, got %q", ans)
	}
}

func TestGenerator_NoFallbackOnFatal(t *testing.T) {
	fatalErr := &openaicompat.ProviderError{StatusCode: 401, Body: "unauthorized", Operation: "chat"}
	primary := &mockGenerator{err: fatalErr}
	fb := &mockGenerator{answer: "should not reach"}
	logger := slog.Default()

	g := NewGenerator(primary, fb, logger)
	_, err := g.GenerateAnswer(context.Background(), "q", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pe *openaicompat.ProviderError
	if !errors.As(err, &pe) || pe.StatusCode != 401 {
		t.Fatalf("expected 401 ProviderError, got %v", err)
	}
}

func TestGenerator_PrimarySuccess(t *testing.T) {
	primary := &mockGenerator{answer: "primary answer"}
	fb := &mockGenerator{answer: "fallback answer"}
	logger := slog.Default()

	g := NewGenerator(primary, fb, logger)
	ans, err := g.GenerateAnswer(context.Background(), "q", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ans != "primary answer" {
		t.Fatalf("expected primary answer, got %q", ans)
	}
}

func TestGenerator_FallbackOnServerError(t *testing.T) {
	primary := &mockGenerator{err: &openaicompat.ProviderError{StatusCode: 503, Body: "cold start", Operation: "chat"}}
	fb := &mockGenerator{answer: "ok"}
	logger := slog.Default()

	g := NewGenerator(primary, fb, logger)
	ans, err := g.GenerateFromPrompt(context.Background(), "p")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ans != "ok" {
		t.Fatalf("expected ok, got %q", ans)
	}
}

func TestClassifier_FallbackOnRetryable(t *testing.T) {
	retryableErr := &openaicompat.ProviderError{StatusCode: 500, Body: "internal", Operation: "chat"}
	primary := &mockClassifier{err: retryableErr}
	fb := &mockClassifier{cls: domain.Classification{Category: "note"}}
	logger := slog.Default()

	c := NewClassifier(primary, fb, logger)
	cls, err := c.Classify(context.Background(), "text")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cls.Category != "note" {
		t.Fatalf("expected note, got %q", cls.Category)
	}
}

func TestClassifier_NoFallbackOnFatal(t *testing.T) {
	primary := &mockClassifier{err: &openaicompat.ProviderError{StatusCode: 400, Body: "bad", Operation: "chat"}}
	fb := &mockClassifier{cls: domain.Classification{Category: "should not reach"}}
	logger := slog.Default()

	c := NewClassifier(primary, fb, logger)
	_, err := c.Classify(context.Background(), "text")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
