package routing

import (
	"context"
	"log/slog"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// Generator implements ports.AnswerGenerator — routes to the appropriate provider
// based on the provider name in context. Falls back to default if not found.
type Generator struct {
	providers       map[string]ports.AnswerGenerator
	defaultProvider string
	logger          *slog.Logger
}

func NewGenerator(providers map[string]ports.AnswerGenerator, defaultProvider string, logger *slog.Logger) *Generator {
	return &Generator{
		providers:       providers,
		defaultProvider: defaultProvider,
		logger:          logger,
	}
}

func (g *Generator) resolve(ctx context.Context) ports.AnswerGenerator {
	if name := ProviderFrom(ctx); name != "" {
		if gen, ok := g.providers[name]; ok {
			return gen
		}
		g.logger.Warn("unknown provider in context, using default", "requested", name, "default", g.defaultProvider)
	}
	return g.providers[g.defaultProvider]
}

func (g *Generator) GenerateAnswer(ctx context.Context, question string, chunks []domain.RetrievedChunk) (string, error) {
	return g.resolve(ctx).GenerateAnswer(ctx, question, chunks)
}

func (g *Generator) GenerateFromPrompt(ctx context.Context, prompt string) (string, error) {
	return g.resolve(ctx).GenerateFromPrompt(ctx, prompt)
}

func (g *Generator) GenerateJSONFromPrompt(ctx context.Context, prompt string) (string, error) {
	return g.resolve(ctx).GenerateJSONFromPrompt(ctx, prompt)
}

// Classifier implements ports.DocumentClassifier — routes to the appropriate provider
// based on the provider name in context.
type Classifier struct {
	providers       map[string]ports.DocumentClassifier
	defaultProvider string
	logger          *slog.Logger
}

func NewClassifier(providers map[string]ports.DocumentClassifier, defaultProvider string, logger *slog.Logger) *Classifier {
	return &Classifier{
		providers:       providers,
		defaultProvider: defaultProvider,
		logger:          logger,
	}
}

func (c *Classifier) resolve(ctx context.Context) ports.DocumentClassifier {
	if name := ProviderFrom(ctx); name != "" {
		if cls, ok := c.providers[name]; ok {
			return cls
		}
		c.logger.Warn("unknown provider in context, using default", "requested", name, "default", c.defaultProvider)
	}
	return c.providers[c.defaultProvider]
}

func (c *Classifier) Classify(ctx context.Context, text string) (domain.Classification, error) {
	return c.resolve(ctx).Classify(ctx, text)
}
