package httpadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
)

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

	return rt.querySvc.GenerateFromPrompt(ctx, prompt)
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
