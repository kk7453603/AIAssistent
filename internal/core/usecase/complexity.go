package usecase

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// TierUncertain means rule-based classifier is not confident.
const TierUncertain domain.ComplexityTier = "uncertain"

var complexityKeywords = []string{
	"сравни", "объясни разницу", "проанализируй", "напиши план",
	"compare", "analyze", "explain why", "explain the difference",
	"evaluate", "design", "разработай", "спроектируй", "оцени",
}

// classifyComplexityRules uses heuristics to determine request complexity.
// Returns TierUncertain if no rule matches with high confidence.
func classifyComplexityRules(message string, intent Intent) domain.ComplexityTier {
	if intent == IntentCode {
		return domain.TierCode
	}

	lower := strings.ToLower(message)
	runeCount := utf8.RuneCountInString(message)

	if runeCount < 30 {
		return domain.TierSimple
	}

	for _, kw := range complexityKeywords {
		if strings.Contains(lower, kw) {
			return domain.TierComplex
		}
	}

	return TierUncertain
}

// AutoAssignTiers automatically maps discovered models to complexity tiers.
func AutoAssignTiers(models []domain.ModelInfo, defaultModel string) domain.ModelRouting {
	routing := domain.ModelRouting{
		Simple:  defaultModel,
		Complex: defaultModel,
		Code:    defaultModel,
	}

	if len(models) == 0 {
		return routing
	}

	sorted := make([]domain.ModelInfo, len(models))
	copy(sorted, models)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SizeBytes > sorted[j].SizeBytes
	})

	for _, m := range sorted {
		nameLower := strings.ToLower(m.Name)
		if strings.Contains(nameLower, "code") {
			routing.Code = m.Name
			break
		}
	}

	routing.Complex = sorted[0].Name
	routing.Simple = sorted[len(sorted)-1].Name

	if routing.Code == defaultModel {
		routing.Code = routing.Complex
	}

	return routing
}
