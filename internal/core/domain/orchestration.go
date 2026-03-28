package domain

import "time"

// AgentSpec defines a specialist agent's configuration.
type AgentSpec struct {
	Name          string   `json:"name"`
	SystemPrompt  string   `json:"system_prompt"`
	Tools         []string `json:"tools"`
	MaxIterations int      `json:"max_iterations"`
}

// OrchestrationPlanStep is a planned agent invocation.
type OrchestrationPlanStep struct {
	Agent string `json:"agent"`
	Task  string `json:"task"`
}

// OrchestrationStep is a completed agent execution.
type OrchestrationStep struct {
	Index      int       `json:"index"`
	Agent      string    `json:"agent"`
	Task       string    `json:"task"`
	Result     string    `json:"result"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	DurationMS float64   `json:"duration_ms"`
}

// Orchestration represents a full multi-agent execution.
type Orchestration struct {
	ID             string
	UserID         string
	ConversationID string
	Request        string
	Plan           []OrchestrationPlanStep
	Steps          []OrchestrationStep
	Status         string // "running", "completed", "failed"
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

// OrchestrationStatus is sent via SSE during orchestration.
type OrchestrationStatus struct {
	OrchestrationID string `json:"orchestration_id"`
	StepIndex       int    `json:"step_index"`
	AgentName       string `json:"agent_name"`
	Task            string `json:"task"`
	Status          string `json:"status"`
	Result          string `json:"result,omitempty"`
}
