package httpadapter

import (
	"testing"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
)

func ptrIface(v interface{}) *interface{} {
	return &v
}

func TestLatestUserMessageContent(t *testing.T) {
	tests := []struct {
		name     string
		messages []apigen.ChatMessage
		want     string
		wantOK   bool
	}{
		{
			name:     "no messages",
			messages: nil,
			want:     "",
			wantOK:   false,
		},
		{
			name: "no user messages",
			messages: []apigen.ChatMessage{
				{Role: apigen.System, Content: ptrIface("system prompt")},
				{Role: apigen.Assistant, Content: ptrIface("hello")},
			},
			want:   "",
			wantOK: false,
		},
		{
			name: "single user message",
			messages: []apigen.ChatMessage{
				{Role: apigen.User, Content: ptrIface("hello world")},
			},
			want:   "hello world",
			wantOK: true,
		},
		{
			name: "multiple user messages returns latest",
			messages: []apigen.ChatMessage{
				{Role: apigen.User, Content: ptrIface("first question")},
				{Role: apigen.Assistant, Content: ptrIface("answer")},
				{Role: apigen.User, Content: ptrIface("second question")},
			},
			want:   "second question",
			wantOK: true,
		},
		{
			name: "user message with nil content is skipped",
			messages: []apigen.ChatMessage{
				{Role: apigen.User, Content: ptrIface("real content")},
				{Role: apigen.User, Content: nil},
			},
			want:   "real content",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := latestUserMessageContent(tt.messages)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMessageText(t *testing.T) {
	tests := []struct {
		name    string
		message apigen.ChatMessage
		want    string
	}{
		{
			name:    "nil content pointer",
			message: apigen.ChatMessage{Role: apigen.User, Content: nil},
			want:    "",
		},
		{
			name: "nil interface value",
			message: apigen.ChatMessage{
				Role:    apigen.User,
				Content: func() *interface{} { var v interface{}; return &v }(),
			},
			want: "",
		},
		{
			name:    "string content",
			message: apigen.ChatMessage{Role: apigen.User, Content: ptrIface("hello")},
			want:    "hello",
		},
		{
			name:    "string content with whitespace",
			message: apigen.ChatMessage{Role: apigen.User, Content: ptrIface("  spaced  ")},
			want:    "spaced",
		},
		{
			name: "array of strings",
			message: apigen.ChatMessage{
				Role: apigen.User,
				Content: ptrIface([]interface{}{
					"part one",
					"part two",
				}),
			},
			want: "part one\npart two",
		},
		{
			name: "array of maps with text key",
			message: apigen.ChatMessage{
				Role: apigen.User,
				Content: ptrIface([]interface{}{
					map[string]interface{}{"type": "text", "text": "hello from map"},
				}),
			},
			want: "hello from map",
		},
		{
			name: "mixed array of strings and maps",
			message: apigen.ChatMessage{
				Role: apigen.User,
				Content: ptrIface([]interface{}{
					"plain string",
					map[string]interface{}{"type": "text", "text": "map text"},
				}),
			},
			want: "plain string\nmap text",
		},
		{
			name: "empty array",
			message: apigen.ChatMessage{
				Role:    apigen.User,
				Content: ptrIface([]interface{}{}),
			},
			want: "",
		},
		{
			name: "map with no text key returns empty from that item",
			message: apigen.ChatMessage{
				Role: apigen.User,
				Content: ptrIface([]interface{}{
					map[string]interface{}{"type": "image_url", "url": "https://example.com/img.png"},
				}),
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMessageText(tt.message)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
