package usecase

import "github.com/kirillkom/personal-ai-assistant/internal/core/domain"

var defaultAgentSpecs = []domain.AgentSpec{
	{
		Name:          "researcher",
		SystemPrompt:  "You are a research specialist. Find facts, sources, and relevant context from the knowledge base and web. Be thorough and cite sources. Return structured findings.",
		Tools:         []string{"knowledge_search", "web_search"},
		MaxIterations: 5,
	},
	{
		Name:          "coder",
		SystemPrompt:  "You are a code specialist. Generate, analyze, debug, and explain code. Provide working examples with clear explanations.",
		Tools:         []string{"knowledge_search"},
		MaxIterations: 5,
	},
	{
		Name:          "writer",
		SystemPrompt:  "You are a writing specialist. Synthesize information from previous research into a clear, well-structured response. Use headings, bullet points, and examples where appropriate.",
		Tools:         []string{"knowledge_search"},
		MaxIterations: 3,
	},
	{
		Name:          "critic",
		SystemPrompt:  "You are a quality critic. Check the previous answer for factual errors, hallucinations, missing information, logical gaps, and unclear explanations. Be strict and specific about issues found. If the answer is good, say so explicitly.",
		Tools:         []string{"knowledge_search"},
		MaxIterations: 3,
	},
}

type AgentRegistry struct {
	specs map[string]domain.AgentSpec
}

func NewAgentRegistry(specs []domain.AgentSpec) *AgentRegistry {
	if len(specs) == 0 {
		specs = defaultAgentSpecs
	}
	m := make(map[string]domain.AgentSpec, len(specs))
	for _, s := range specs {
		m[s.Name] = s
	}
	return &AgentRegistry{specs: m}
}

func (r *AgentRegistry) Get(name string) (domain.AgentSpec, bool) {
	s, ok := r.specs[name]
	return s, ok
}

func (r *AgentRegistry) List() []domain.AgentSpec {
	result := make([]domain.AgentSpec, 0, len(r.specs))
	for _, s := range r.specs {
		result = append(result, s)
	}
	return result
}

func (r *AgentRegistry) Names() []string {
	names := make([]string, 0, len(r.specs))
	for name := range r.specs {
		names = append(names, name)
	}
	return names
}
