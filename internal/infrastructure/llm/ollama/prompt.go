package ollama

import (
	"fmt"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func buildClassificationPrompt(text string) string {
	const maxSnippet = 4000
	snippet := text
	if len(snippet) > maxSnippet {
		snippet = snippet[:maxSnippet]
	}

	return `You are a document classifier.
Return strict JSON object with keys:
category (string), subcategory (string), tags (array of strings), confidence (number from 0 to 1), summary (string).
No markdown, no extra keys.

Document:
` + snippet
}

func buildAnswerPrompt(question string, chunks []domain.RetrievedChunk) string {
	var contextBuilder strings.Builder
	for idx, chunk := range chunks {
		contextBuilder.WriteString(fmt.Sprintf(
			"[%d] file=%s category=%s score=%.3f\n%s\n\n",
			idx+1,
			chunk.Filename,
			chunk.Category,
			chunk.Score,
			chunk.Text,
		))
	}

	return fmt.Sprintf(`Answer user question only from context below.
If context is insufficient, say it directly.

Question:
%s

Context:
%s
`, question, contextBuilder.String())
}
