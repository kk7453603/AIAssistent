package ollama

import (
	"context"
	"fmt"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func (g *Generator) ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	return g.client.chatWithTools(ctx, messages, tools)
}

func (c *Client) chatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	model := c.plannerModel
	if model == "" {
		model = c.genModel
	}

	ollamaMessages := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		msg := map[string]any{"role": m.Role, "content": m.Content}
		if len(m.ToolCalls) > 0 {
			msg["tool_calls"] = m.ToolCalls
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		ollamaMessages = append(ollamaMessages, msg)
	}

	ollamaTools := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		ollamaTools = append(ollamaTools, map[string]any{
			"type": t.Type,
			"function": map[string]any{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  t.Function.Parameters,
			},
		})
	}

	reqBody := map[string]any{
		"model":    model,
		"messages": ollamaMessages,
		"stream":   false,
		"think":    false,
	}
	if len(ollamaTools) > 0 {
		reqBody["tools"] = ollamaTools
	}

	var response struct {
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	}

	if err := c.postJSON(ctx, "/api/chat", reqBody, &response, "chat"); err != nil {
		return nil, fmt.Errorf("ollama chat: %w", err)
	}

	result := &domain.ChatToolsResult{
		Content: strings.TrimSpace(response.Message.Content),
	}
	for _, tc := range response.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, domain.ToolCall{
			Function: domain.ToolCallFunc{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return result, nil
}
