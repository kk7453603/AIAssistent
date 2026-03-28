package tokenizer

import (
	"testing"
)

func TestTokenizeUnicode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "english text",
			input: "Hello World 123",
			want:  []string{"hello", "world", "123"},
		},
		{
			name:  "russian text",
			input: "Привет Мир 456",
			want:  []string{"привет", "мир", "456"},
		},
		{
			name:  "mixed RU/EN",
			input: "Go-паттерны для Clean Architecture",
			want:  []string{"go", "паттерны", "для", "clean", "architecture"},
		},
		{
			name:  "punctuation splitting",
			input: "file.txt, hello_world, test-case",
			want:  []string{"file", "txt", "hello", "world", "test", "case"},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "only punctuation",
			input: "!@#$%^&*()",
			want:  []string{},
		},
		{
			name:  "unicode digits",
			input: "version2 тест3",
			want:  []string{"version2", "тест3"},
		},
		{
			name:  "path-like input",
			input: "notes/programming/go-patterns.md",
			want:  []string{"notes", "programming", "go", "patterns", "md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TokenizeUnicode(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("TokenizeUnicode(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("TokenizeUnicode(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("TokenizeUnicode(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
