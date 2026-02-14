package httpadapter

import (
	"encoding/json"
	"strings"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
)

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
				if segment := strings.TrimSpace(typed); segment != "" {
					parts = append(parts, segment)
				}
			case map[string]interface{}:
				if text, ok := typed["text"].(string); ok {
					if segment := strings.TrimSpace(text); segment != "" {
						parts = append(parts, segment)
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
