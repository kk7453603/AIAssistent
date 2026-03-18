package openaicompat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// Classifier implements ports.DocumentClassifier via an OpenAI-compatible API.
type Classifier struct {
	client *Client
}

func NewClassifier(client *Client) *Classifier {
	return &Classifier{client: client}
}

func (c *Classifier) Classify(ctx context.Context, text string) (domain.Classification, error) {
	const maxSnippet = 4000
	snippet := text
	if len(snippet) > maxSnippet {
		snippet = snippet[:maxSnippet]
	}

	prompt := `You are a document classifier.
Return strict JSON object with keys:
category (string), subcategory (string), tags (array of strings), confidence (number from 0 to 1), summary (string).
No markdown, no extra keys.

Document:
` + snippet

	respText, err := c.client.chatCompletion(ctx, []chatMessage{
		{Role: "user", Content: prompt},
	}, true)
	if err != nil {
		return domain.Classification{}, err
	}

	jsonStr := extractJSONObject(respText)
	var result domain.Classification
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return domain.Classification{}, fmt.Errorf("parse classification json: %w", err)
	}
	if result.Tags == nil {
		result.Tags = []string{}
	}
	return result, nil
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}
	return raw
}
