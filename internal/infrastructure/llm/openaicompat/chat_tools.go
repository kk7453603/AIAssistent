package openaicompat

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
	// Build messages in OpenAI wire format.
	wireMessages := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		msg := map[string]any{"role": m.Role, "content": m.Content}
		if len(m.ToolCalls) > 0 {
			msg["tool_calls"] = m.ToolCalls
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		wireMessages = append(wireMessages, msg)
	}

	// Build tools in OpenAI wire format.
	wireTools := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		wireTools = append(wireTools, map[string]any{
			"type": t.Type,
			"function": map[string]any{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  t.Function.Parameters,
			},
		})
	}

	reqBody := map[string]any{
		"model":    c.model,
		"messages": wireMessages,
	}
	if len(wireTools) > 0 {
		reqBody["tools"] = wireTools
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string         `json:"name"`
						Arguments map[string]any `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := c.postJSON(ctx, "/v1/chat/completions", reqBody, &response, "chat_tools"); err != nil {
		return nil, fmt.Errorf("openaicompat chat_tools: %w", err)
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("openaicompat chat_tools: empty choices")
	}

	msg := response.Choices[0].Message
	result := &domain.ChatToolsResult{
		Content: strings.TrimSpace(msg.Content),
	}
	for _, tc := range msg.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, domain.ToolCall{
			ID: tc.ID,
			Function: domain.ToolCallFunc{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return result, nil
}
