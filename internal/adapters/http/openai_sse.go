package httpadapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
)

type chatCompletionsSSEResponse struct {
	Chunks []apigen.ChatCompletionChunk
}

func (response chatCompletionsSSEResponse) VisitChatCompletionsResponse(w http.ResponseWriter) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming is not supported by response writer")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for _, chunk := range response.Chunks {
		payload, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
		flusher.Flush()
	}

	if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func buildTextStreamChunks(completionID string, created int64, modelID string, text string, chunkChars int) []apigen.ChatCompletionChunk {
	if chunkChars <= 0 {
		chunkChars = 120
	}

	parts := splitByRunes(text, chunkChars)
	chunks := make([]apigen.ChatCompletionChunk, 0, len(parts)+1)
	for idx, part := range parts {
		delta := apigen.ChatMessageDelta{}
		if idx == 0 {
			role := "assistant"
			delta.Role = &role
		}
		if part != "" {
			delta.Content = &part
		}
		chunks = append(chunks, apigen.ChatCompletionChunk{
			Id:      completionID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   modelID,
			Choices: []apigen.ChatCompletionChunkChoice{{
				Index:        0,
				Delta:        delta,
				FinishReason: nil,
			}},
		})
	}

	finishReason := "stop"
	chunks = append(chunks, apigen.ChatCompletionChunk{
		Id:      completionID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   modelID,
		Choices: []apigen.ChatCompletionChunkChoice{{
			Index:        0,
			Delta:        apigen.ChatMessageDelta{},
			FinishReason: &finishReason,
		}},
	})

	return chunks
}

func buildToolCallStreamChunks(completionID string, created int64, modelID string, toolCall apigen.ToolCall) []apigen.ChatCompletionChunk {
	role := "assistant"
	toolCalls := []apigen.ToolCall{toolCall}
	finishReason := "tool_calls"
	return []apigen.ChatCompletionChunk{
		{
			Id:      completionID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   modelID,
			Choices: []apigen.ChatCompletionChunkChoice{{
				Index: 0,
				Delta: apigen.ChatMessageDelta{
					Role:      &role,
					ToolCalls: &toolCalls,
				},
			}},
		},
		{
			Id:      completionID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   modelID,
			Choices: []apigen.ChatCompletionChunkChoice{{
				Index:        0,
				Delta:        apigen.ChatMessageDelta{},
				FinishReason: &finishReason,
			}},
		},
	}
}

func splitByRunes(text string, chunkChars int) []string {
	if strings.TrimSpace(text) == "" {
		return []string{""}
	}
	if chunkChars <= 0 || utf8.RuneCountInString(text) <= chunkChars {
		return []string{text}
	}

	parts := make([]string, 0, utf8.RuneCountInString(text)/chunkChars+1)
	runes := []rune(text)
	for start := 0; start < len(runes); start += chunkChars {
		end := start + chunkChars
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}
