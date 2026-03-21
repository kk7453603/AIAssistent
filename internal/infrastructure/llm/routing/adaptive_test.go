package routing

import (
	"context"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// mockGenerator is a simple test double for ports.AnswerGenerator.
type mockGenerator struct {
	name string
}

func (m *mockGenerator) GenerateAnswer(_ context.Context, _ string, _ []domain.RetrievedChunk) (string, error) {
	return m.name, nil
}

func (m *mockGenerator) GenerateFromPrompt(_ context.Context, _ string) (string, error) {
	return m.name, nil
}

func (m *mockGenerator) GenerateJSONFromPrompt(_ context.Context, _ string) (string, error) {
	return m.name, nil
}

func (m *mockGenerator) ChatWithTools(_ context.Context, _ []domain.ChatMessage, _ []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	return &domain.ChatToolsResult{Content: m.name}, nil
}

var _ ports.AnswerGenerator = (*mockGenerator)(nil)

func buildTestAdaptive() *AdaptiveGenerator {
	providers := map[string]ports.AnswerGenerator{
		"fast":    &mockGenerator{name: "fast"},
		"large":   &mockGenerator{name: "large"},
		"default": &mockGenerator{name: "default"},
	}
	return NewAdaptiveGenerator(providers, DefaultRules(), NewPerformanceTracker(100), "default", nil)
}

func TestAdaptiveGenerator_FallbackWhenNoContext(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := context.Background()

	result, err := ag.GenerateFromPrompt(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "default" {
		t.Errorf("expected fallback 'default', got %q", result)
	}
}

func TestAdaptiveGenerator_ExplicitProviderOverrides(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := WithProvider(context.Background(), "large")

	result, err := ag.GenerateFromPrompt(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "large" {
		t.Errorf("expected explicit 'large', got %q", result)
	}
}

func TestAdaptiveGenerator_IntentCodeHighComplexity(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := WithIntent(context.Background(), "code")
	ctx = WithComplexity(ctx, 9)

	result, err := ag.GenerateFromPrompt(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "large" {
		t.Errorf("expected 'large' for code/9, got %q", result)
	}
}

func TestAdaptiveGenerator_IntentCodeLowComplexity(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := WithIntent(context.Background(), "code")
	ctx = WithComplexity(ctx, 3)

	result, err := ag.GenerateFromPrompt(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "fast" {
		t.Errorf("expected 'fast' for code/3, got %q", result)
	}
}

func TestAdaptiveGenerator_IntentWeb(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := WithIntent(context.Background(), "web")
	ctx = WithComplexity(ctx, 5)

	result, err := ag.GenerateFromPrompt(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "fast" {
		t.Errorf("expected 'fast' for web, got %q", result)
	}
}

func TestAdaptiveGenerator_IntentKnowledge(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := WithIntent(context.Background(), "knowledge")
	ctx = WithComplexity(ctx, 5)

	result, err := ag.GenerateFromPrompt(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "default" {
		t.Errorf("expected 'default' for knowledge, got %q", result)
	}
}

func TestAdaptiveGenerator_IntentGeneralLow(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := WithIntent(context.Background(), "general")
	ctx = WithComplexity(ctx, 2)

	result, err := ag.GenerateFromPrompt(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "fast" {
		t.Errorf("expected 'fast' for general/2, got %q", result)
	}
}

func TestAdaptiveGenerator_IntentGeneralHigh(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := WithIntent(context.Background(), "general")
	ctx = WithComplexity(ctx, 6)

	result, err := ag.GenerateFromPrompt(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "default" {
		t.Errorf("expected 'default' for general/6, got %q", result)
	}
}

func TestAdaptiveGenerator_RecordsPerformance(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := WithIntent(context.Background(), "task")
	ctx = WithComplexity(ctx, 1)

	_, _ = ag.GenerateFromPrompt(ctx, "hello")

	stats := ag.tracker.Stats("fast")
	if stats.TotalCalls != 1 {
		t.Errorf("expected 1 call recorded for 'fast', got %d", stats.TotalCalls)
	}
}

func TestAdaptiveGenerator_ChatWithToolsRoutes(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := WithIntent(context.Background(), "code")
	ctx = WithComplexity(ctx, 8)

	result, err := ag.ChatWithTools(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "large" {
		t.Errorf("expected 'large' for ChatWithTools code/8, got %q", result.Content)
	}
}

func TestAdaptiveGenerator_UnknownExplicitProviderFallsThrough(t *testing.T) {
	ag := buildTestAdaptive()
	ctx := WithProvider(context.Background(), "nonexistent")
	ctx = WithIntent(ctx, "web")
	ctx = WithComplexity(ctx, 5)

	result, err := ag.GenerateFromPrompt(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	// Should fall through to rule-based routing since provider doesn't exist.
	if result != "fast" {
		t.Errorf("expected rule-based 'fast' after unknown provider, got %q", result)
	}
}

// --- EstimateComplexity tests ---

func TestEstimateComplexity_Short(t *testing.T) {
	c := EstimateComplexity("hi", nil)
	if c < 1 {
		t.Errorf("short message should have complexity >= 1, got %d", c)
	}
}

func TestEstimateComplexity_LongWithKeywords(t *testing.T) {
	msg := strings.Repeat("word ", 80) + " explain in detail and also compare"
	c := EstimateComplexity(msg, nil)
	if c < 7 {
		t.Errorf("long complex message should have high complexity, got %d", c)
	}
}

func TestEstimateComplexity_CodeBlock(t *testing.T) {
	msg := "Please fix this:\n```go\nfunc main() {}\n```"
	c := EstimateComplexity(msg, nil)
	// Should include code block bonus.
	if c < 2 {
		t.Errorf("code block message should have complexity >= 2, got %d", c)
	}
}

func TestEstimateComplexity_DeepConversation(t *testing.T) {
	history := make([]domain.ChatMessage, 10)
	c := EstimateComplexity("ok", history)
	// Short message (1) + deep conversation (1) = 2
	if c < 2 {
		t.Errorf("deep conversation should add complexity, got %d", c)
	}
}

func TestEstimateComplexity_Clamped(t *testing.T) {
	// Create a message that triggers all bonuses.
	msg := strings.Repeat("word ", 100) + " explain in detail and also 1. first 2. second ```code```"
	history := make([]domain.ChatMessage, 10)
	c := EstimateComplexity(msg, history)
	if c > 10 {
		t.Errorf("complexity should be clamped to 10, got %d", c)
	}
}

// --- Context helpers tests ---

func TestContextIntentRoundTrip(t *testing.T) {
	ctx := WithIntent(context.Background(), "code")
	if got := IntentFrom(ctx); got != "code" {
		t.Errorf("IntentFrom = %q, want 'code'", got)
	}
}

func TestContextIntentDefault(t *testing.T) {
	if got := IntentFrom(context.Background()); got != "" {
		t.Errorf("IntentFrom on empty ctx = %q, want ''", got)
	}
}

func TestContextComplexityRoundTrip(t *testing.T) {
	ctx := WithComplexity(context.Background(), 7)
	if got := ComplexityFrom(ctx); got != 7 {
		t.Errorf("ComplexityFrom = %d, want 7", got)
	}
}

func TestContextComplexityDefault(t *testing.T) {
	if got := ComplexityFrom(context.Background()); got != 0 {
		t.Errorf("ComplexityFrom on empty ctx = %d, want 0", got)
	}
}
