package httpadapter

import "testing"

func TestShouldTriggerToolCall(t *testing.T) {
	keywords := []string{"file", "document", "upload", "attach"}

	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{
			name:    "explicit upload request",
			message: "please upload this file and summarize it",
			want:    true,
		},
		{
			name:    "attached files request",
			message: "summarize attached files",
			want:    true,
		},
		{
			name:    "russian attachment request",
			message: "Сделай summary по вложенному файлу",
			want:    true,
		},
		{
			name:    "informational question about concept",
			message: "what is a document database?",
			want:    false,
		},
		{
			name:    "informational api question",
			message: "how to upload files via API endpoint?",
			want:    false,
		},
		{
			name:    "statement with document reference",
			message: "this document explains architecture",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldTriggerToolCall(tt.message, keywords); got != tt.want {
				t.Fatalf("shouldTriggerToolCall(%q) = %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}
