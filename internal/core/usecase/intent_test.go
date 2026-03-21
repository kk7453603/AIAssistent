package usecase

import (
	"strings"
	"testing"
)

func TestClassifyIntentByKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected Intent
	}{
		// Code intent
		{"выполни Python скрипт", IntentCode},
		{"запусти код на bash", IntentCode},
		{"execute this python code", IntentCode},
		{"вычисли 2+2", IntentCode},
		{"напиши и выполни скрипт", IntentCode},

		// File intent
		{"прочитай файл Progress.md", IntentFile},
		{"покажи содержимое каталога ML", IntentFile},
		{"list directory /vaults", IntentFile},
		{"открой файл README", IntentFile},

		// Task intent
		{"создай задачу купить молоко", IntentTask},
		{"покажи мои задачи", IntentTask},
		{"напомни мне завтра", IntentTask},

		// Web intent
		{"найди в интернете рецепт борща", IntentWeb},
		{"поищи онлайн информацию", IntentWeb},

		// General (no keyword match)
		{"что такое transformer?", IntentGeneral},
		{"расскажи про attention mechanism", IntentGeneral},
		{"привет", IntentGeneral},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := classifyIntentByKeywords(tt.input)
			if got != tt.expected {
				t.Errorf("classifyIntentByKeywords(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSystemPromptForIntent(t *testing.T) {
	// Ensure each intent produces a non-empty system prompt addition
	for _, intent := range []Intent{IntentKnowledge, IntentCode, IntentFile, IntentTask, IntentWeb, IntentGeneral} {
		prompt := systemPromptForIntent(intent)
		if prompt == "" {
			t.Errorf("systemPromptForIntent(%q) returned empty string", intent)
		}
	}
}

func TestClassifyIntentLLMPrompt(t *testing.T) {
	prompt := classifyIntentLLMPrompt("сравни мой прогресс по модулям")
	if !strings.Contains(prompt, "knowledge") || !strings.Contains(prompt, "code") {
		t.Error("prompt should list all categories")
	}
}

func TestParseIntent(t *testing.T) {
	tests := []struct {
		input    string
		expected Intent
	}{
		{"knowledge", IntentKnowledge},
		{"  CODE  ", IntentCode},
		{"file\n", IntentFile},
		{"task something", IntentTask},
		{"web", IntentWeb},
		{"garbage", IntentGeneral},
		{"", IntentGeneral},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseIntent(tt.input); got != tt.expected {
				t.Errorf("parseIntent(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
