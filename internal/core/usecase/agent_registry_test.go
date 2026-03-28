package usecase

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestAgentRegistry_GetExisting(t *testing.T) {
	reg := NewAgentRegistry([]domain.AgentSpec{
		{Name: "researcher", SystemPrompt: "You are a researcher", Tools: []string{"knowledge_search"}, MaxIterations: 5},
	})
	spec, ok := reg.Get("researcher")
	if !ok {
		t.Fatal("expected to find researcher")
	}
	if spec.SystemPrompt != "You are a researcher" {
		t.Errorf("prompt = %q", spec.SystemPrompt)
	}
}

func TestAgentRegistry_GetMissing(t *testing.T) {
	reg := NewAgentRegistry(nil)
	_, ok := reg.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestAgentRegistry_DefaultSpecs(t *testing.T) {
	reg := NewAgentRegistry(nil)
	specs := reg.List()
	if len(specs) < 4 {
		t.Fatalf("expected at least 4 default specs, got %d", len(specs))
	}
	names := make(map[string]bool)
	for _, s := range specs {
		names[s.Name] = true
	}
	for _, expected := range []string{"researcher", "coder", "writer", "critic"} {
		if !names[expected] {
			t.Errorf("missing default spec: %s", expected)
		}
	}
}
