package routing

import "sort"

// RoutingRule maps an intent + complexity range to a preferred provider.
type RoutingRule struct {
	Intent        string `json:"intent"`
	ComplexityMin int    `json:"complexity_min"` // 0-10 scale, inclusive
	ComplexityMax int    `json:"complexity_max"` // 0-10 scale, inclusive
	Provider      string `json:"provider"`       // provider name
	Priority      int    `json:"priority"`       // lower = higher priority
}

// Match returns true when the rule applies to the given intent and complexity.
// An empty Intent means "match any intent".
func (r RoutingRule) Match(intent string, complexity int) bool {
	if r.Intent != "" && r.Intent != intent {
		return false
	}
	return complexity >= r.ComplexityMin && complexity <= r.ComplexityMax
}

// MatchRules returns all rules that match the given intent and complexity,
// sorted by priority (lowest first).
func MatchRules(rules []RoutingRule, intent string, complexity int) []RoutingRule {
	var matched []RoutingRule
	for _, r := range rules {
		if r.Match(intent, complexity) {
			matched = append(matched, r)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Priority < matched[j].Priority
	})
	return matched
}

// DefaultRules returns a sensible set of routing rules for the known intents.
// The provider names used here ("large", "fast", "default") are logical names
// that must be registered in the AdaptiveGenerator's provider map.
func DefaultRules() []RoutingRule {
	return []RoutingRule{
		{Intent: "code", ComplexityMin: 7, ComplexityMax: 10, Provider: "large", Priority: 10},
		{Intent: "code", ComplexityMin: 0, ComplexityMax: 6, Provider: "fast", Priority: 20},
		{Intent: "knowledge", ComplexityMin: 0, ComplexityMax: 10, Provider: "default", Priority: 30},
		{Intent: "general", ComplexityMin: 0, ComplexityMax: 3, Provider: "fast", Priority: 40},
		{Intent: "general", ComplexityMin: 4, ComplexityMax: 10, Provider: "default", Priority: 50},
		{Intent: "web", ComplexityMin: 0, ComplexityMax: 10, Provider: "fast", Priority: 60},
		{Intent: "task", ComplexityMin: 0, ComplexityMax: 10, Provider: "fast", Priority: 70},
		{Intent: "file", ComplexityMin: 0, ComplexityMax: 10, Provider: "fast", Priority: 80},
	}
}
