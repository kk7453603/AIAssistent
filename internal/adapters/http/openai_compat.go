package httpadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
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

func (rt *Router) openAICompatAuthMiddleware(f apigen.StrictHandlerFunc, operationID string) apigen.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
		if rt.openAICompatAPIKey == "" {
			return f(ctx, w, r, request)
		}
		if !isOpenAICompatOperation(operationID) {
			return f(ctx, w, r, request)
		}
		if isAuthorizedBearerHeader(r.Header.Get("Authorization"), rt.openAICompatAPIKey) {
			return f(ctx, w, r, request)
		}

		switch operationID {
		case "ListModels":
			return apigen.ListModels401JSONResponse{Error: "unauthorized"}, nil
		case "ChatCompletions":
			return apigen.ChatCompletions401JSONResponse{Error: "unauthorized"}, nil
		default:
			return f(ctx, w, r, request)
		}
	}
}

func isOpenAICompatOperation(operationID string) bool {
	switch operationID {
	case "ListModels", "ChatCompletions":
		return true
	default:
		return false
	}
}

func isAuthorizedBearerHeader(headerValue, expectedToken string) bool {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" || expectedToken == "" {
		return false
	}
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(headerValue, bearerPrefix) {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(headerValue, bearerPrefix))
	return token == expectedToken
}

func (rt *Router) ListModels(_ context.Context, _ apigen.ListModelsRequestObject) (apigen.ListModelsResponseObject, error) {
	created := time.Now().Unix()
	modelID := rt.openAICompatModelID
	if modelID == "" {
		modelID = "paa-rag-v1"
	}

	return apigen.ListModels200JSONResponse{
		Object: "list",
		Data: []apigen.ModelObject{
			{
				Id:      modelID,
				Object:  "model",
				OwnedBy: "personal-ai-assistant",
				Created: &created,
			},
		},
	}, nil
}

func (rt *Router) ChatCompletions(ctx context.Context, request apigen.ChatCompletionsRequestObject) (apigen.ChatCompletionsResponseObject, error) {
	if request.Body == nil {
		return apigen.ChatCompletions400JSONResponse{Error: "request body is required"}, nil
	}
	if len(request.Body.Messages) == 0 {
		return apigen.ChatCompletions400JSONResponse{Error: "messages are required"}, nil
	}

	modelID := strings.TrimSpace(request.Body.Model)
	if modelID == "" {
		modelID = rt.openAICompatModelID
	}
	if modelID == "" {
		modelID = "paa-rag-v1"
	}

	completionID := newCompletionID()
	created := time.Now().Unix()
	stream := request.Body.Stream != nil && *request.Body.Stream

	lastMessage := request.Body.Messages[len(request.Body.Messages)-1]
	if lastMessage.Role == apigen.Tool {
		toolContent := extractMessageText(lastMessage)
		if toolContent == "" {
			return apigen.ChatCompletions400JSONResponse{Error: "tool message content is required"}, nil
		}
		lastUser, _ := latestUserMessageContent(request.Body.Messages)
		answer, err := rt.postProcessToolOutput(ctx, request.Body.Messages, toolContent)
		if err != nil {
			return apigen.ChatCompletions500JSONResponse{Error: err.Error()}, nil
		}
		debugMode := "tool_postprocess"
		response := buildTextChatCompletionResponse(completionID, created, modelID, lastUser, answer, &apigen.DebugInfo{Mode: &debugMode})
		if stream {
			return chatCompletionsSSEResponse{Chunks: buildTextStreamChunks(completionID, created, modelID, answer, rt.openAICompatStreamChunkChars)}, nil
		}
		return apigen.ChatCompletions200JSONResponse(response), nil
	}

	lastUser, ok := latestUserMessageContent(request.Body.Messages)
	if !ok {
		return apigen.ChatCompletions400JSONResponse{Error: "at least one user message with text content is required"}, nil
	}

	if toolCall, ok := rt.buildToolCallIfTriggered(lastUser, request.Body.Tools); ok {
		response := buildToolCallChatCompletionResponse(completionID, created, modelID, lastUser, toolCall)
		if stream {
			return chatCompletionsSSEResponse{Chunks: buildToolCallStreamChunks(completionID, created, modelID, toolCall)}, nil
		}
		return apigen.ChatCompletions200JSONResponse(response), nil
	}

	ragQuestion := buildRAGQuestion(request.Body.Messages, lastUser, rt.openAICompatContextMessages)
	answer, err := rt.queryUC.Answer(ctx, ragQuestion, rt.ragTopK, domain.SearchFilter{})
	if err != nil {
		return apigen.ChatCompletions500JSONResponse{Error: err.Error()}, nil
	}

	debugMode := "rag"
	debug := &apigen.DebugInfo{Mode: &debugMode}
	if sources := toAPIDebugSources(answer.Sources); len(sources) > 0 {
		debug.Sources = &sources
	}

	response := buildTextChatCompletionResponse(completionID, created, modelID, ragQuestion, answer.Text, debug)
	if stream {
		return chatCompletionsSSEResponse{Chunks: buildTextStreamChunks(completionID, created, modelID, answer.Text, rt.openAICompatStreamChunkChars)}, nil
	}
	return apigen.ChatCompletions200JSONResponse(response), nil
}

func (rt *Router) postProcessToolOutput(ctx context.Context, messages []apigen.ChatMessage, toolOutput string) (string, error) {
	lastUser, _ := latestUserMessageContent(messages)
	prompt := fmt.Sprintf(`You are an assistant.
Rewrite the tool result into a concise helpful final answer.
If the result includes per-file statuses, summarize them clearly.

User request:
%s

Tool result:
%s
`, lastUser, toolOutput)

	return rt.queryUC.GenerateFromPrompt(ctx, prompt)
}

func (rt *Router) buildToolCallIfTriggered(lastUserMessage string, tools *[]apigen.ToolDefinition) (apigen.ToolCall, bool) {
	if tools == nil || len(*tools) == 0 {
		return apigen.ToolCall{}, false
	}
	if !containsAnyKeyword(strings.ToLower(lastUserMessage), rt.toolTriggerKeywords) {
		return apigen.ToolCall{}, false
	}

	selectedTool := (*tools)[0]
	if selectedTool.Function.Name == "" {
		return apigen.ToolCall{}, false
	}

	argumentsPayload := map[string]string{"question": strings.TrimSpace(lastUserMessage)}
	argumentsJSON, _ := json.Marshal(argumentsPayload)
	return apigen.ToolCall{
		Id:   fmt.Sprintf("call_%d", time.Now().UnixNano()),
		Type: apigen.ToolCallTypeFunction,
		Function: apigen.FunctionCall{
			Name:      selectedTool.Function.Name,
			Arguments: string(argumentsJSON),
		},
	}, true
}

func buildRAGQuestion(messages []apigen.ChatMessage, lastUserMessage string, contextMessages int) string {
	if contextMessages <= 1 {
		return lastUserMessage
	}

	start := len(messages) - contextMessages
	if start < 0 {
		start = 0
	}
	contextLines := make([]string, 0, contextMessages)
	for _, msg := range messages[start:] {
		if msg.Role == apigen.Tool {
			continue
		}
		text := extractMessageText(msg)
		if text == "" {
			continue
		}
		contextLines = append(contextLines, fmt.Sprintf("%s: %s", msg.Role, text))
	}
	if len(contextLines) == 0 {
		return lastUserMessage
	}

	return fmt.Sprintf("Conversation context:\n%s\n\nCurrent user question:\n%s", strings.Join(contextLines, "\n"), lastUserMessage)
}

func containsAnyKeyword(message string, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

func latestUserMessageContent(messages []apigen.ChatMessage) (string, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != apigen.User {
			continue
		}
		text := extractMessageText(messages[i])
		if text != "" {
			return text, true
		}
	}
	return "", false
}

func extractMessageText(message apigen.ChatMessage) string {
	if message.Content == nil {
		return ""
	}
	if *message.Content == nil {
		return ""
	}

	switch content := (*message.Content).(type) {
	case string:
		return strings.TrimSpace(content)
	case []interface{}:
		parts := make([]string, 0, len(content))
		for _, item := range content {
			switch typed := item.(type) {
			case string:
				if s := strings.TrimSpace(typed); s != "" {
					parts = append(parts, s)
				}
			case map[string]interface{}:
				if text, ok := typed["text"].(string); ok {
					if s := strings.TrimSpace(text); s != "" {
						parts = append(parts, s)
					}
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		payload, err := json.Marshal(content)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(payload))
	}
}

func newCompletionID() string {
	return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
}

func buildTextChatCompletionResponse(completionID string, created int64, modelID string, promptText string, answerText string, debug *apigen.DebugInfo) apigen.ChatCompletionResponse {
	content := interface{}(answerText)
	finishReason := "stop"
	response := apigen.ChatCompletionResponse{
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
	return response
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

func estimateUsage(prompt string, completion string) *apigen.Usage {
	promptTokens := len(strings.Fields(prompt))
	completionTokens := len(strings.Fields(completion))
	return &apigen.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
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
