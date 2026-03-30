package ollama

import (
	"context"
	"encoding/json"
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

	// Check if we should stream thinking tokens
	onThinking := domain.ThinkingCallbackFromContext(ctx)
	useStreaming := c.thinkEnabled && onThinking != nil

	if useStreaming {
		return c.chatWithToolsStreaming(ctx, model, ollamaMessages, ollamaTools, onThinking)
	}
	return c.chatWithToolsSync(ctx, model, ollamaMessages, ollamaTools)
}

// chatWithToolsSync is the original non-streaming path.
func (c *Client) chatWithToolsSync(ctx context.Context, model string, messages, tools []map[string]any) (*domain.ChatToolsResult, error) {
	reqBody := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
		"think":    c.thinkEnabled,
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}

	var response struct {
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			Thinking  string `json:"thinking"`
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

	content := strings.TrimSpace(response.Message.Content)
	thinking := strings.TrimSpace(response.Message.Thinking)
	if thinking != "" {
		content = "<think>" + thinking + "</think>\n" + content
	}

	result := &domain.ChatToolsResult{
		Content: content,
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

// chatWithToolsStreaming uses Ollama streaming API to send thinking tokens in real-time.
func (c *Client) chatWithToolsStreaming(ctx context.Context, model string, messages, tools []map[string]any, onThinking domain.ThinkingDeltaCallback) (*domain.ChatToolsResult, error) {
	reqBody := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   true,
		"think":    true,
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}

	var thinkingBuf strings.Builder
	var contentBuf strings.Builder
	var toolCalls []domain.ToolCall

	type streamChunk struct {
		Message struct {
			Content   string `json:"content"`
			Thinking  string `json:"thinking"`
			ToolCalls []struct {
				Function struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		Done bool `json:"done"`
	}

	err := c.postStreamJSON(ctx, "/api/chat", reqBody, "chat-stream", func(raw json.RawMessage) error {
		var chunk streamChunk
		if err := json.Unmarshal(raw, &chunk); err != nil {
			return nil // skip malformed chunks
		}

		// Stream thinking tokens to the callback
		if chunk.Message.Thinking != "" {
			thinkingBuf.WriteString(chunk.Message.Thinking)
			onThinking(chunk.Message.Thinking)
		}

		// Accumulate content
		if chunk.Message.Content != "" {
			contentBuf.WriteString(chunk.Message.Content)
		}

		// Collect tool calls from final chunk
		for _, tc := range chunk.Message.ToolCalls {
			toolCalls = append(toolCalls, domain.ToolCall{
				Function: domain.ToolCallFunc{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ollama chat stream: %w", err)
	}

	content := strings.TrimSpace(contentBuf.String())
	thinking := strings.TrimSpace(thinkingBuf.String())
	if thinking != "" {
		content = "<think>" + thinking + "</think>\n" + content
	}

	return &domain.ChatToolsResult{
		Content:   content,
		ToolCalls: toolCalls,
	}, nil
}
