package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func (g *Generator) ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	return g.client.chatWithTools(ctx, messages, tools)
}

func (c *Client) chatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	genModel, plannerModel, _, thinkEnabled := c.runtimeSnapshot()
	model := plannerModel
	if model == "" {
		model = genModel
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
	useStreaming := thinkEnabled && onThinking != nil

	if useStreaming {
		result, err := c.chatWithToolsStreaming(ctx, model, ollamaMessages, ollamaTools, onThinking)
		if err != nil {
			// Fallback to content-streaming with <think> tag detection
			slog.Warn("chat_think_api_fallback", "error", err)
			result, err = c.chatWithToolsContentStreaming(ctx, model, ollamaMessages, ollamaTools, onThinking)
			if err != nil {
				slog.Warn("chat_content_stream_fallback", "error", err)
				return c.chatWithToolsSync(ctx, model, ollamaMessages, ollamaTools)
			}
			return result, nil
		}
		return result, nil
	}
	return c.chatWithToolsSync(ctx, model, ollamaMessages, ollamaTools)
}

// chatWithToolsSync is the original non-streaming path.
func (c *Client) chatWithToolsSync(ctx context.Context, model string, messages, tools []map[string]any) (*domain.ChatToolsResult, error) {
	_, _, _, thinkEnabled := c.runtimeSnapshot()
	reqBody := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}
	if thinkEnabled {
		reqBody["think"] = true
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
		// Retry without think if model doesn't support it
		if thinkEnabled {
			delete(reqBody, "think")
			if err2 := c.postJSON(ctx, "/api/chat", reqBody, &response, "chat"); err2 != nil {
				return nil, fmt.Errorf("ollama chat: %w", err)
			}
			slog.Warn("think_not_supported_fallback", "model", reqBody["model"])
		} else {
			return nil, fmt.Errorf("ollama chat: %w", err)
		}
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

// chatWithToolsContentStreaming streams content tokens and detects <think> tags inline.
// This works with models that don't support the native think API but output <think> tags in content.
func (c *Client) chatWithToolsContentStreaming(ctx context.Context, model string, messages, tools []map[string]any, onThinking domain.ThinkingDeltaCallback) (*domain.ChatToolsResult, error) {
	reqBody := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   true,
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}

	var thinkingBuf strings.Builder
	var contentBuf strings.Builder
	var toolCalls []domain.ToolCall

	// State machine for detecting <think> tags in content stream.
	const (
		stateInit     = 0
		stateThinking = 1
		stateContent  = 2
	)
	state := stateInit
	var tagBuf strings.Builder // buffers potential partial tags

	type streamChunk struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		Done bool `json:"done"`
	}

	err := c.postStreamJSON(ctx, "/api/chat", reqBody, "chat-content-stream", func(raw json.RawMessage) error {
		var chunk streamChunk
		if err := json.Unmarshal(raw, &chunk); err != nil {
			return nil
		}

		text := chunk.Message.Content
		if text != "" {
			switch state {
			case stateInit:
				tagBuf.WriteString(text)
				accumulated := tagBuf.String()
				// Wait until we have enough to determine if <think> is present
				if len(accumulated) < 7 {
					// Could still be a partial "<think>" — wait for more
					if strings.HasPrefix("<think>", accumulated) {
						break
					}
					// Not a prefix of <think> — flush as content
					contentBuf.WriteString(accumulated)
					tagBuf.Reset()
					state = stateContent
					break
				}
				if strings.HasPrefix(accumulated, "<think>") {
					// We have <think> — switch to thinking state
					state = stateThinking
					remainder := accumulated[7:]
					tagBuf.Reset()
					if remainder != "" {
						thinkingBuf.WriteString(remainder)
						onThinking(remainder)
					}
				} else {
					// No <think> tag — flush as content
					contentBuf.WriteString(accumulated)
					tagBuf.Reset()
					state = stateContent
				}

			case stateThinking:
				// Check for </think> close tag
				if idx := strings.Index(text, "</think>"); idx >= 0 {
					before := text[:idx]
					after := text[idx+8:]
					if before != "" {
						thinkingBuf.WriteString(before)
						onThinking(before)
					}
					state = stateContent
					if after != "" {
						contentBuf.WriteString(after)
					}
				} else {
					thinkingBuf.WriteString(text)
					onThinking(text)
				}

			case stateContent:
				contentBuf.WriteString(text)
			}
		}

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
		return nil, fmt.Errorf("ollama chat content stream: %w", err)
	}

	// Flush any remaining tag buffer (e.g. stream ended without <think>)
	if tagBuf.Len() > 0 {
		contentBuf.WriteString(tagBuf.String())
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
