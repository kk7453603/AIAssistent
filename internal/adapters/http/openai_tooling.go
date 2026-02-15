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
	if !shouldTriggerToolCall(lastUserMessage, rt.toolTriggerKeywords) {
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

var (
	toolIntentActionKeywords = []string{
		"upload", "attach", "ingest", "index", "process", "parse", "summarize", "summarise", "analyze", "analyse", "extract", "scan", "read",
		"загруз", "прикреп", "влож", "проиндекс", "обработ", "суммар", "саммари", "проанализ", "извлеч", "прочита",
	}
	toolIntentFileKeywords = []string{
		"file", "files", "document", "documents", "attachment", "attachments", "pdf", "docx", "txt", "md",
		"файл", "файлы", "документ", "документы", "вложение", "вложения",
	}
	toolIntentAttachmentHints = []string{
		"this file", "these files", "this document", "these documents",
		"attached file", "attached files", "attached document", "attached documents",
		"этот файл", "эти файлы", "этот документ", "эти документы",
		"вложен", "прикрепл", "во вложении", "из вложения",
	}
	toolIntentRequestCues = []string{
		"?", "please", "can you", "could you", "tell me", "summarize", "analyse", "analyze", "extract",
		"пожалуйста", "можешь", "сделай", "расскажи", "проанализируй", "извлеки",
	}
	toolIntentInformationalPrefixes = []string{
		"what is ", "what are ", "how to ", "how do ", "how does ", "explain ", "tell me about ",
		"что такое ", "как ", "объясни ", "расскажи про ", "расскажи о ",
	}
	toolIntentInformationalHints = []string{
		" api ", " endpoint", " endpoints", "документац", "спецификац", "swagger", "openapi",
	}
)

func shouldTriggerToolCall(message string, configuredKeywords []string) bool {
	normalized := normalizeIntentText(message)
	if normalized == "" {
		return false
	}

	attachmentRef := containsAnyKeyword(normalized, toolIntentAttachmentHints)
	actionHits := countKeywordHits(normalized, toolIntentActionKeywords)
	fileHits := countKeywordHits(normalized, toolIntentFileKeywords)
	customHits := countKeywordHits(normalized, configuredKeywords)

	if attachmentRef && hasRequestCue(normalized, actionHits) {
		return true
	}
	if looksLikeInformationalQuestion(normalized) && !attachmentRef {
		return false
	}
	if actionHits > 0 && (fileHits > 0 || customHits > 0) {
		return true
	}

	return false
}

func normalizeIntentText(message string) string {
	if strings.TrimSpace(message) == "" {
		return ""
	}
	return strings.ToLower(strings.Join(strings.Fields(message), " "))
}

func countKeywordHits(message string, keywords []string) int {
	if len(keywords) == 0 || message == "" {
		return 0
	}
	hits := 0
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(message, strings.ToLower(strings.TrimSpace(keyword))) {
			hits++
		}
	}
	return hits
}

func hasRequestCue(message string, actionHits int) bool {
	if actionHits > 0 {
		return true
	}
	return containsAnyKeyword(message, toolIntentRequestCues)
}

func looksLikeInformationalQuestion(message string) bool {
	for _, prefix := range toolIntentInformationalPrefixes {
		if strings.HasPrefix(message, prefix) {
			return true
		}
	}
	for _, hint := range toolIntentInformationalHints {
		if strings.Contains(message, hint) {
			return true
		}
	}
	return false
}
