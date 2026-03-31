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

var orchestrateKeywords = []string{
	"исследуй подробно", "проанализируй детально", "deep research",
	"подробный анализ", "detailed analysis", "thorough investigation",
	"исследуй и напиши", "research and write",
}

// shouldOrchestrate returns true when the request warrants multi-agent orchestration.
func shouldOrchestrate(intent Intent, complexity domain.ComplexityTier, message string) bool {
	if complexity != domain.TierComplex {
		return false
	}
	if intent == IntentGeneral {
		return false
	}
	lower := strings.ToLower(message)
	for _, kw := range orchestrateKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
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

	candidates := make([]domain.ModelInfo, 0, len(models))
	for _, m := range models {
		if isLikelyEmbeddingModel(m.Name) {
			continue
		}
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		return routing
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].SizeBytes > candidates[j].SizeBytes
	})

	for _, m := range candidates {
		nameLower := strings.ToLower(m.Name)
		if strings.Contains(nameLower, "code") {
			routing.Code = m.Name
			break
		}
	}

	routing.Complex = candidates[0].Name
	routing.Simple = candidates[len(candidates)-1].Name

	if routing.Code == defaultModel {
		routing.Code = routing.Complex
	}

	return routing
}

func isLikelyEmbeddingModel(name string) bool {
	nameLower := strings.ToLower(strings.TrimSpace(name))
	if nameLower == "" {
		return false
	}
	markers := []string{
		"embed",
		"embedding",
		"nomic-bert",
		"nomic-embed",
		"bge",
		"gte",
		"e5",
	}
	for _, marker := range markers {
		if strings.Contains(nameLower, marker) {
			return true
		}
	}
	return false
}
