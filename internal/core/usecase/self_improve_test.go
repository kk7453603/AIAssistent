package usecase

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestAutoApplyCategories(t *testing.T) {
	tests := []struct {
		cat  string
		want bool
	}{
		{"system_prompt", true},
		{"intent_keywords", true},
		{"reindex_document", true},
		{"eval_case", true},
		{"add_document", false},
		{"unknown", false},
	}
	for _, tt := range tests {
		if got := domain.AutoApplyCategories[tt.cat]; got != tt.want {
			t.Errorf("AutoApplyCategories[%q] = %v, want %v", tt.cat, got, tt.want)
		}
	}
}

func TestBuildSelfImprovePrompt(t *testing.T) {
	p := buildSelfImprovePrompt(
		map[string]int{"tool_error": 5, "empty_retrieval": 3},
		map[string]int{"up": 10, "down": 2},
		[]string{"Not accurate"},
	)
	if p == "" || len(p) < 50 {
		t.Errorf("prompt too short: %d", len(p))
	}
}

func TestParseSelfImprovements(t *testing.T) {
	raw := `[{"category":"system_prompt","description":"Update researcher prompt","action":{"prompt":"new prompt"}}]`
	imps := parseSelfImprovements(raw)
	if len(imps) != 1 {
		t.Fatalf("expected 1, got %d", len(imps))
	}
	if imps[0].Category != "system_prompt" {
		t.Errorf("category = %q", imps[0].Category)
	}
}

func TestParseSelfImprovements_WithWrapper(t *testing.T) {
	raw := `Here are improvements:\n[{"category":"eval_case","description":"Add test case","action":{}}]`
	imps := parseSelfImprovements(raw)
	if len(imps) != 1 {
		t.Fatalf("expected 1, got %d", len(imps))
	}
}
