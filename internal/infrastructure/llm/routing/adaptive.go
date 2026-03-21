package routing

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// AdaptiveGenerator implements ports.AnswerGenerator.
// It selects the underlying provider based on intent, complexity and
// historical performance data.
type AdaptiveGenerator struct {
	providers map[string]ports.AnswerGenerator
	rules     []RoutingRule
	tracker   *PerformanceTracker
	fallback  string // default provider name
	logger    *slog.Logger
}

// NewAdaptiveGenerator creates an AdaptiveGenerator.
// providers maps logical names (e.g. "fast", "large", "default") to generators.
// fallback is the provider name used when no rule matches.
func NewAdaptiveGenerator(
	providers map[string]ports.AnswerGenerator,
	rules []RoutingRule,
	tracker *PerformanceTracker,
	fallback string,
	logger *slog.Logger,
) *AdaptiveGenerator {
	if logger == nil {
		logger = slog.Default()
	}
	return &AdaptiveGenerator{
		providers: providers,
		rules:     rules,
		tracker:   tracker,
		fallback:  fallback,
		logger:    logger,
	}
}

// resolve picks the best provider for the given context.
func (ag *AdaptiveGenerator) resolve(ctx context.Context) ports.AnswerGenerator {
	// 1. Explicit provider in context takes precedence (backward compat).
	if name := ProviderFrom(ctx); name != "" {
		if gen, ok := ag.providers[name]; ok {
			return gen
		}
		ag.logger.Warn("explicit provider not found, continuing with adaptive routing",
			"requested", name)
	}

	intent := IntentFrom(ctx)
	complexity := ComplexityFrom(ctx)

	// 2. Match rules.
	matched := MatchRules(ag.rules, intent, complexity)
	if len(matched) > 0 {
		// Collect unique candidate providers that we actually have.
		seen := make(map[string]bool)
		var candidates []string
		for _, r := range matched {
			if _, ok := ag.providers[r.Provider]; ok && !seen[r.Provider] {
				seen[r.Provider] = true
				candidates = append(candidates, r.Provider)
			}
		}

		// 3. Among candidates, prefer the one with best performance.
		if len(candidates) > 0 {
			best := ag.tracker.BestFor(candidates)
			if gen, ok := ag.providers[best]; ok {
				ag.logger.Debug("adaptive routing decision",
					"intent", intent,
					"complexity", complexity,
					"provider", best,
				)
				return gen
			}
		}
	}

	// 4. Fallback to default.
	ag.logger.Debug("adaptive routing fallback",
		"intent", intent,
		"complexity", complexity,
		"provider", ag.fallback,
	)
	return ag.providers[ag.fallback]
}

// providerName returns the logical name for a generator by reverse-looking it up.
func (ag *AdaptiveGenerator) providerName(gen ports.AnswerGenerator) string {
	for name, g := range ag.providers {
		if g == gen {
			return name
		}
	}
	return ag.fallback
}

// --- ports.AnswerGenerator implementation ---

func (ag *AdaptiveGenerator) GenerateAnswer(ctx context.Context, question string, chunks []domain.RetrievedChunk) (string, error) {
	gen := ag.resolve(ctx)
	name := ag.providerName(gen)

	start := time.Now()
	result, err := gen.GenerateAnswer(ctx, question, chunks)
	ag.tracker.Record(name, time.Since(start), err)

	return result, err
}

func (ag *AdaptiveGenerator) GenerateFromPrompt(ctx context.Context, prompt string) (string, error) {
	gen := ag.resolve(ctx)
	name := ag.providerName(gen)

	start := time.Now()
	result, err := gen.GenerateFromPrompt(ctx, prompt)
	ag.tracker.Record(name, time.Since(start), err)

	return result, err
}

func (ag *AdaptiveGenerator) GenerateJSONFromPrompt(ctx context.Context, prompt string) (string, error) {
	gen := ag.resolve(ctx)
	name := ag.providerName(gen)

	start := time.Now()
	result, err := gen.GenerateJSONFromPrompt(ctx, prompt)
	ag.tracker.Record(name, time.Since(start), err)

	return result, err
}

func (ag *AdaptiveGenerator) ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	gen := ag.resolve(ctx)
	name := ag.providerName(gen)

	start := time.Now()
	result, err := gen.ChatWithTools(ctx, messages, tools)
	ag.tracker.Record(name, time.Since(start), err)

	return result, err
}

// --- Complexity Estimation ---

// EstimateComplexity scores a user message on a 0-10 scale using simple heuristics.
func EstimateComplexity(message string, history []domain.ChatMessage) int {
	score := 0

	// Message length.
	runeCount := utf8.RuneCountInString(message)
	switch {
	case runeCount < 50:
		score += 1
	case runeCount < 200:
		score += 3
	default:
		score += 5
	}

	lower := strings.ToLower(message)

	// Keyword markers for complex requests.
	complexKeywords := []string{
		"explain in detail", "compare", "analyze", "analyse",
		"step by step", "in depth", "comprehensive", "подробно",
		"проанализируй", "сравни", "объясни детально",
	}
	for _, kw := range complexKeywords {
		if strings.Contains(lower, kw) {
			score += 2
			break
		}
	}

	// Multi-part question indicators.
	multiPartKeywords := []string{" and ", " also ", " а также ", " и ещё "}
	for _, kw := range multiPartKeywords {
		if strings.Contains(lower, kw) {
			score += 2
			break
		}
	}
	// Numbered list (e.g., "1.", "2.")
	if strings.Contains(message, "1.") && strings.Contains(message, "2.") {
		score += 2
	}

	// Code block presence.
	if strings.Contains(message, "```") {
		score += 1
	}

	// Conversation depth.
	if len(history) > 6 {
		score += 1
	}

	// Clamp to 0-10.
	if score > 10 {
		score = 10
	}
	return score
}
