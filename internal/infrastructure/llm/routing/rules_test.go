package routing

import (
	"testing"
)

func TestRoutingRule_Match(t *testing.T) {
	tests := []struct {
		name       string
		rule       RoutingRule
		intent     string
		complexity int
		want       bool
	}{
		{
			name:       "exact match",
			rule:       RoutingRule{Intent: "code", ComplexityMin: 5, ComplexityMax: 10},
			intent:     "code",
			complexity: 7,
			want:       true,
		},
		{
			name:       "intent mismatch",
			rule:       RoutingRule{Intent: "code", ComplexityMin: 0, ComplexityMax: 10},
			intent:     "web",
			complexity: 5,
			want:       false,
		},
		{
			name:       "complexity below min",
			rule:       RoutingRule{Intent: "code", ComplexityMin: 5, ComplexityMax: 10},
			intent:     "code",
			complexity: 3,
			want:       false,
		},
		{
			name:       "complexity above max",
			rule:       RoutingRule{Intent: "code", ComplexityMin: 0, ComplexityMax: 6},
			intent:     "code",
			complexity: 7,
			want:       false,
		},
		{
			name:       "boundary min inclusive",
			rule:       RoutingRule{Intent: "code", ComplexityMin: 5, ComplexityMax: 10},
			intent:     "code",
			complexity: 5,
			want:       true,
		},
		{
			name:       "boundary max inclusive",
			rule:       RoutingRule{Intent: "code", ComplexityMin: 0, ComplexityMax: 6},
			intent:     "code",
			complexity: 6,
			want:       true,
		},
		{
			name:       "empty intent matches any",
			rule:       RoutingRule{Intent: "", ComplexityMin: 0, ComplexityMax: 10},
			intent:     "web",
			complexity: 5,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.Match(tt.intent, tt.complexity)
			if got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchRules(t *testing.T) {
	rules := []RoutingRule{
		{Intent: "code", ComplexityMin: 7, ComplexityMax: 10, Provider: "large", Priority: 10},
		{Intent: "code", ComplexityMin: 0, ComplexityMax: 6, Provider: "fast", Priority: 20},
		{Intent: "general", ComplexityMin: 0, ComplexityMax: 10, Provider: "default", Priority: 50},
	}

	t.Run("matches code high complexity", func(t *testing.T) {
		matched := MatchRules(rules, "code", 8)
		if len(matched) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matched))
		}
		if matched[0].Provider != "large" {
			t.Errorf("expected provider 'large', got %q", matched[0].Provider)
		}
	})

	t.Run("matches code low complexity", func(t *testing.T) {
		matched := MatchRules(rules, "code", 3)
		if len(matched) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matched))
		}
		if matched[0].Provider != "fast" {
			t.Errorf("expected provider 'fast', got %q", matched[0].Provider)
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		matched := MatchRules(rules, "web", 5)
		if len(matched) != 0 {
			t.Fatalf("expected 0 matches, got %d", len(matched))
		}
	})

	t.Run("sorted by priority", func(t *testing.T) {
		overlapping := []RoutingRule{
			{Intent: "code", ComplexityMin: 0, ComplexityMax: 10, Provider: "b", Priority: 30},
			{Intent: "code", ComplexityMin: 0, ComplexityMax: 10, Provider: "a", Priority: 10},
		}
		matched := MatchRules(overlapping, "code", 5)
		if len(matched) != 2 {
			t.Fatalf("expected 2 matches, got %d", len(matched))
		}
		if matched[0].Provider != "a" {
			t.Errorf("first match should be 'a' (priority 10), got %q", matched[0].Provider)
		}
	})
}

func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()
	if len(rules) == 0 {
		t.Fatal("DefaultRules() returned empty slice")
	}

	// Verify code high-complexity maps to "large".
	matched := MatchRules(rules, "code", 9)
	if len(matched) == 0 || matched[0].Provider != "large" {
		t.Error("expected code/9 to match 'large' provider")
	}

	// Verify task maps to "fast".
	matched = MatchRules(rules, "task", 2)
	if len(matched) == 0 || matched[0].Provider != "fast" {
		t.Error("expected task/2 to match 'fast' provider")
	}
}
