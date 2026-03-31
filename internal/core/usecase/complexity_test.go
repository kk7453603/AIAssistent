package usecase

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestClassifyComplexityRules(t *testing.T) {
	tests := []struct {
		name    string
		message string
		intent  Intent
		want    domain.ComplexityTier
	}{
		{name: "code intent", message: "напиши код на Python", intent: IntentCode, want: domain.TierCode},
		{name: "short simple", message: "привет", intent: IntentGeneral, want: domain.TierSimple},
		{name: "short question", message: "как дела?", intent: IntentGeneral, want: domain.TierSimple},
		{name: "complex keyword RU", message: "сравни подходы к архитектуре микросервисов", intent: IntentGeneral, want: domain.TierComplex},
		{name: "complex keyword EN", message: "analyze the performance of this algorithm", intent: IntentGeneral, want: domain.TierComplex},
		{name: "explain why", message: "explain why transformers work better than RNNs", intent: IntentGeneral, want: domain.TierComplex},
		{name: "medium length general", message: "расскажи что такое dependency injection в Go", intent: IntentKnowledge, want: TierUncertain},
		{name: "write plan", message: "напиши план обучения машинному обучению", intent: IntentGeneral, want: domain.TierComplex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyComplexityRules(tt.message, tt.intent)
			if got != tt.want {
				t.Errorf("classifyComplexityRules(%q, %q) = %q, want %q", tt.message, tt.intent, got, tt.want)
			}
		})
	}
}

func TestAutoAssignTiers(t *testing.T) {
	models := []domain.ModelInfo{
		{Name: "llama3.1:8b", SizeBytes: 4_000_000_000},
		{Name: "qwen3.5:9b", SizeBytes: 9_000_000_000},
		{Name: "qwen-coder:7b", SizeBytes: 4_500_000_000},
	}

	routing := AutoAssignTiers(models, "llama3.1:8b")

	if routing.Code != "qwen-coder:7b" {
		t.Errorf("Code = %q, want %q", routing.Code, "qwen-coder:7b")
	}
	if routing.Complex != "qwen3.5:9b" {
		t.Errorf("Complex = %q, want %q", routing.Complex, "qwen3.5:9b")
	}
	if routing.Simple != "llama3.1:8b" {
		t.Errorf("Simple = %q, want %q", routing.Simple, "llama3.1:8b")
	}
}

func TestAutoAssignTiers_IgnoresEmbeddingModels(t *testing.T) {
	models := []domain.ModelInfo{
		{Name: "bge-m3:latest", SizeBytes: 1_100_000_000},
		{Name: "nomic-embed-text:latest", SizeBytes: 270_000_000},
		{Name: "qwen3:14b", SizeBytes: 9_200_000_000},
		{Name: "qwen3-coder:30b", SizeBytes: 18_500_000_000},
	}

	routing := AutoAssignTiers(models, "zfujicute/OmniCoder:latest")

	if routing.Complex != "qwen3-coder:30b" {
		t.Errorf("Complex = %q, want %q", routing.Complex, "qwen3-coder:30b")
	}
	if routing.Simple != "qwen3:14b" {
		t.Errorf("Simple = %q, want %q", routing.Simple, "qwen3:14b")
	}
	if routing.Code != "qwen3-coder:30b" {
		t.Errorf("Code = %q, want %q", routing.Code, "qwen3-coder:30b")
	}
}

func TestAutoAssignTiers_SingleModel(t *testing.T) {
	models := []domain.ModelInfo{
		{Name: "llama3.1:8b", SizeBytes: 4_000_000_000},
	}

	routing := AutoAssignTiers(models, "llama3.1:8b")

	if routing.Simple != "llama3.1:8b" || routing.Complex != "llama3.1:8b" || routing.Code != "llama3.1:8b" {
		t.Errorf("single model should fill all tiers, got %+v", routing)
	}
}

func TestAutoAssignTiers_Empty(t *testing.T) {
	routing := AutoAssignTiers(nil, "default-model")

	if routing.Simple != "default-model" || routing.Complex != "default-model" {
		t.Errorf("empty models should use default, got %+v", routing)
	}
}

func TestShouldOrchestrate(t *testing.T) {
	tests := []struct {
		name       string
		intent     Intent
		complexity domain.ComplexityTier
		message    string
		want       bool
	}{
		{name: "complex + keyword RU", intent: IntentKnowledge, complexity: domain.TierComplex, message: "исследуй подробно архитектуру системы", want: true},
		{name: "complex + keyword EN", intent: IntentKnowledge, complexity: domain.TierComplex, message: "deep research on microservices", want: true},
		{name: "complex but no keyword", intent: IntentKnowledge, complexity: domain.TierComplex, message: "расскажи про Docker", want: false},
		{name: "simple tier rejected", intent: IntentKnowledge, complexity: domain.TierSimple, message: "исследуй подробно что-то", want: false},
		{name: "general intent rejected", intent: IntentGeneral, complexity: domain.TierComplex, message: "исследуй подробно что-то", want: false},
		{name: "code tier rejected", intent: IntentCode, complexity: domain.TierCode, message: "deep research on algorithms", want: false},
		{name: "research and write EN", intent: IntentKnowledge, complexity: domain.TierComplex, message: "research and write a summary about RAG", want: true},
		{name: "detailed analysis RU", intent: IntentKnowledge, complexity: domain.TierComplex, message: "проанализируй детально производительность", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldOrchestrate(tt.intent, tt.complexity, tt.message)
			if got != tt.want {
				t.Errorf("shouldOrchestrate(%q, %q, %q) = %v, want %v", tt.intent, tt.complexity, tt.message, got, tt.want)
			}
		})
	}
}
