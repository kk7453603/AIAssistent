package httpadapter

import (
	"fmt"
	"strings"
	"time"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func newCompletionID() string {
	return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
}

func buildTextChatCompletionResponse(completionID string, created int64, modelID string, promptText string, answerText string, debug *apigen.DebugInfo) apigen.ChatCompletionResponse {
	content := interface{}(answerText)
	finishReason := "stop"
	return apigen.ChatCompletionResponse{
		Id:      completionID,
		Object:  "chat.completion",
		Created: created,
		Model:   modelID,
		Choices: []apigen.ChatCompletionChoice{
			{
				Index: 0,
				Message: apigen.ChatMessage{
					Role:    apigen.Assistant,
					Content: &content,
				},
				FinishReason: &finishReason,
			},
		},
		Usage: estimateUsage(promptText, answerText),
		Debug: debug,
	}
}

func buildToolCallChatCompletionResponse(completionID string, created int64, modelID string, promptText string, toolCall apigen.ToolCall) apigen.ChatCompletionResponse {
	emptyContent := interface{}("")
	finishReason := "tool_calls"
	toolCalls := []apigen.ToolCall{toolCall}
	return apigen.ChatCompletionResponse{
		Id:      completionID,
		Object:  "chat.completion",
		Created: created,
		Model:   modelID,
		Choices: []apigen.ChatCompletionChoice{
			{
				Index: 0,
				Message: apigen.ChatMessage{
					Role:      apigen.Assistant,
					Content:   &emptyContent,
					ToolCalls: &toolCalls,
				},
				FinishReason: &finishReason,
			},
		},
		Usage: estimateUsage(promptText, toolCall.Function.Arguments),
	}
}

func estimateUsage(prompt string, completion string) *apigen.Usage {
	promptTokens := estimateTokenCount(prompt)
	completionTokens := estimateTokenCount(completion)
	return &apigen.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}

func estimateTokenCount(text string) int {
	return len(strings.Fields(text))
}

func toAPIDebugSources(chunks []domain.RetrievedChunk) []apigen.DebugSource {
	out := make([]apigen.DebugSource, 0, len(chunks))
	for _, chunk := range chunks {
		documentID := chunk.DocumentID
		filename := chunk.Filename
		category := chunk.Category
		score := float32(chunk.Score)
		out = append(out, apigen.DebugSource{
			DocumentId: &documentID,
			Filename:   &filename,
			Category:   &category,
			Score:      &score,
		})
	}
	return out
}
