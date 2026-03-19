package fallback

import (
	"context"
	"log/slog"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/openaicompat"
)

// Generator wraps ports.AnswerGenerator with automatic fallback on retryable errors.
type Generator struct {
	primary  ports.AnswerGenerator
	fallback ports.AnswerGenerator
	logger   *slog.Logger
}

func NewGenerator(primary, fallback ports.AnswerGenerator, logger *slog.Logger) *Generator {
	return &Generator{primary: primary, fallback: fallback, logger: logger}
}

func (g *Generator) GenerateAnswer(ctx context.Context, question string, chunks []domain.RetrievedChunk) (string, error) {
	ans, err := g.primary.GenerateAnswer(ctx, question, chunks)
	if err != nil && openaicompat.IsRetryable(err) {
		g.logger.Warn("primary LLM failed, falling back", "op", "GenerateAnswer", "error", err)
		return g.fallback.GenerateAnswer(ctx, question, chunks)
	}
	return ans, err
}

func (g *Generator) GenerateFromPrompt(ctx context.Context, prompt string) (string, error) {
	ans, err := g.primary.GenerateFromPrompt(ctx, prompt)
	if err != nil && openaicompat.IsRetryable(err) {
		g.logger.Warn("primary LLM failed, falling back", "op", "GenerateFromPrompt", "error", err)
		return g.fallback.GenerateFromPrompt(ctx, prompt)
	}
	return ans, err
}

func (g *Generator) GenerateJSONFromPrompt(ctx context.Context, prompt string) (string, error) {
	ans, err := g.primary.GenerateJSONFromPrompt(ctx, prompt)
	if err != nil && openaicompat.IsRetryable(err) {
		g.logger.Warn("primary LLM failed, falling back", "op", "GenerateJSONFromPrompt", "error", err)
		return g.fallback.GenerateJSONFromPrompt(ctx, prompt)
	}
	return ans, err
}

// Classifier wraps ports.DocumentClassifier with automatic fallback on retryable errors.
type Classifier struct {
	primary  ports.DocumentClassifier
	fallback ports.DocumentClassifier
	logger   *slog.Logger
}

func NewClassifier(primary, fallback ports.DocumentClassifier, logger *slog.Logger) *Classifier {
	return &Classifier{primary: primary, fallback: fallback, logger: logger}
}

func (c *Classifier) Classify(ctx context.Context, text string) (domain.Classification, error) {
	cls, err := c.primary.Classify(ctx, text)
	if err != nil && openaicompat.IsRetryable(err) {
		c.logger.Warn("primary LLM failed, falling back", "op", "Classify", "error", err)
		return c.fallback.Classify(ctx, text)
	}
	return cls, err
}
