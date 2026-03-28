package usecase

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestOrchContainsCriticIssues(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"The answer is correct and complete.", false},
		{"There is an error in the second paragraph.", true},
		{"Ответ содержит неточности.", true},
		{"Missing information about the deadline.", true},
		{"Everything looks good.", false},
	}
	for _, tt := range tests {
		got := orchContainsCriticIssues(tt.input)
		if got != tt.want {
			t.Errorf("orchContainsCriticIssues(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestOrchTruncate(t *testing.T) {
	if orchTruncate("hello world", 5) != "hello..." {
		t.Errorf("expected truncation")
	}
	if orchTruncate("short", 100) != "short" {
		t.Errorf("expected no truncation")
	}
}

func TestOrchLastUserMsg(t *testing.T) {
	msgs := []domain.AgentInputMessage{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "answer"},
		{Role: "user", Content: "second question"},
	}
	got := orchLastUserMsg(msgs)
	if got != "second question" {
		t.Errorf("orchLastUserMsg = %q, want %q", got, "second question")
	}
}
