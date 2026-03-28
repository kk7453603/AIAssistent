package openaicompat

import (
	"context"
	"fmt"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// Generator implements ports.AnswerGenerator via an OpenAI-compatible API.
type Generator struct {
	client *Client
}

func NewGenerator(client *Client) *Generator {
	return &Generator{client: client}
}

func (g *Generator) GenerateAnswer(ctx context.Context, question string, chunks []domain.RetrievedChunk) (string, error) {
	prompt := buildAnswerPrompt(question, chunks)
	return g.client.chatCompletion(ctx, []chatMessage{
		{Role: "system", Content: "Answer user question only from context below. If context is insufficient, say it directly."},
		{Role: "user", Content: prompt},
	}, false)
}

func (g *Generator) GenerateFromPrompt(ctx context.Context, prompt string) (string, error) {
	return g.client.chatCompletion(ctx, []chatMessage{
		{Role: "user", Content: prompt},
	}, false)
}

func (g *Generator) GenerateJSONFromPrompt(ctx context.Context, prompt string) (string, error) {
	return g.client.chatCompletion(ctx, []chatMessage{
		{Role: "user", Content: prompt},
	}, true)
}

func buildAnswerPrompt(question string, chunks []domain.RetrievedChunk) string {
	var b strings.Builder
	b.WriteString("Question:\n")
	b.WriteString(question)
	b.WriteString("\n\nContext:\n")
	for idx, chunk := range chunks {
		fmt.Fprintf(&b, "[%d] file=%s category=%s score=%.3f\n%s\n\n",
			idx+1, chunk.Filename, chunk.Category, chunk.Score, chunk.Text)
	}
	return b.String()
}
