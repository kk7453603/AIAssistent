package routing

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// --- stub generator ---

type stubGenerator struct {
	name string
	err  error
}

func (s *stubGenerator) GenerateAnswer(_ context.Context, _ string, _ []domain.RetrievedChunk) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return "answer-from-" + s.name, nil
}

func (s *stubGenerator) GenerateFromPrompt(_ context.Context, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return "prompt-from-" + s.name, nil
}

func (s *stubGenerator) GenerateJSONFromPrompt(_ context.Context, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return "json-from-" + s.name, nil
}

// --- stub classifier ---

type stubClassifier struct {
	name string
	err  error
}

func (s *stubClassifier) Classify(_ context.Context, _ string) (domain.Classification, error) {
	if s.err != nil {
		return domain.Classification{}, s.err
	}
	return domain.Classification{Category: "cat-" + s.name}, nil
}

// --- context tests ---

func TestWithProviderRoundTrip(t *testing.T) {
	ctx := context.Background()
	if got := ProviderFrom(ctx); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	ctx = WithProvider(ctx, "huggingface")
	if got := ProviderFrom(ctx); got != "huggingface" {
		t.Fatalf("expected huggingface, got %q", got)
	}
}

// --- generator tests ---

func makeGenerators(names ...string) map[string]ports.AnswerGenerator {
	m := make(map[string]ports.AnswerGenerator, len(names))
	for _, n := range names {
		m[n] = &stubGenerator{name: n}
	}
	return m
}

func makeClassifiers(names ...string) map[string]ports.DocumentClassifier {
	m := make(map[string]ports.DocumentClassifier, len(names))
	for _, n := range names {
		m[n] = &stubClassifier{name: n}
	}
	return m
}

func TestGeneratorRoutesToRequestedProvider(t *testing.T) {
	gen := NewGenerator(makeGenerators("ollama", "huggingface"), "ollama", slog.Default())

	ctx := WithProvider(context.Background(), "huggingface")
	ans, err := gen.GenerateAnswer(ctx, "q", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ans != "answer-from-huggingface" {
		t.Fatalf("expected answer-from-huggingface, got %q", ans)
	}
}

func TestGeneratorFallsBackToDefault(t *testing.T) {
	gen := NewGenerator(makeGenerators("ollama", "huggingface"), "ollama", slog.Default())

	ans, err := gen.GenerateAnswer(context.Background(), "q", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ans != "answer-from-ollama" {
		t.Fatalf("expected answer-from-ollama, got %q", ans)
	}
}

func TestGeneratorUnknownProviderUsesDefault(t *testing.T) {
	gen := NewGenerator(makeGenerators("ollama"), "ollama", slog.Default())

	ctx := WithProvider(context.Background(), "nonexistent")
	ans, err := gen.GenerateAnswer(ctx, "q", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ans != "answer-from-ollama" {
		t.Fatalf("expected answer-from-ollama, got %q", ans)
	}
}

func TestGeneratorPropagatesError(t *testing.T) {
	providers := makeGenerators("ollama")
	providers["broken"] = &stubGenerator{name: "broken", err: errors.New("fail")}
	gen := NewGenerator(providers, "ollama", slog.Default())

	ctx := WithProvider(context.Background(), "broken")
	_, err := gen.GenerateAnswer(ctx, "q", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGeneratorFromPromptRoutes(t *testing.T) {
	gen := NewGenerator(makeGenerators("a", "b"), "a", slog.Default())

	ctx := WithProvider(context.Background(), "b")
	ans, err := gen.GenerateFromPrompt(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if ans != "prompt-from-b" {
		t.Fatalf("expected prompt-from-b, got %q", ans)
	}
}

func TestGeneratorJSONFromPromptRoutes(t *testing.T) {
	gen := NewGenerator(makeGenerators("a", "b"), "a", slog.Default())

	ctx := WithProvider(context.Background(), "b")
	ans, err := gen.GenerateJSONFromPrompt(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if ans != "json-from-b" {
		t.Fatalf("expected json-from-b, got %q", ans)
	}
}

// --- classifier tests ---

func TestClassifierRoutesToRequestedProvider(t *testing.T) {
	cls := NewClassifier(makeClassifiers("ollama", "hf"), "ollama", slog.Default())

	ctx := WithProvider(context.Background(), "hf")
	c, err := cls.Classify(ctx, "text")
	if err != nil {
		t.Fatal(err)
	}
	if c.Category != "cat-hf" {
		t.Fatalf("expected cat-hf, got %q", c.Category)
	}
}

func TestClassifierFallsBackToDefault(t *testing.T) {
	cls := NewClassifier(makeClassifiers("ollama", "hf"), "ollama", slog.Default())

	c, err := cls.Classify(context.Background(), "text")
	if err != nil {
		t.Fatal(err)
	}
	if c.Category != "cat-ollama" {
		t.Fatalf("expected cat-ollama, got %q", c.Category)
	}
}

func TestClassifierUnknownProviderUsesDefault(t *testing.T) {
	cls := NewClassifier(makeClassifiers("ollama"), "ollama", slog.Default())

	ctx := WithProvider(context.Background(), "nonexistent")
	c, err := cls.Classify(ctx, "text")
	if err != nil {
		t.Fatal(err)
	}
	if c.Category != "cat-ollama" {
		t.Fatalf("expected cat-ollama, got %q", c.Category)
	}
}
