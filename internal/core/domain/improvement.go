package domain

import "time"

type AgentEvent struct {
	ID             string
	UserID         string
	ConversationID string
	EventType      string
	Details        map[string]any
	CreatedAt      time.Time
}

type AgentFeedback struct {
	ID             string
	UserID         string
	ConversationID string
	MessageID      string
	Rating         string
	Comment        string
	CreatedAt      time.Time
}

type AgentImprovement struct {
	ID          string
	Category    string
	Description string
	Action      map[string]any
	Status      string
	CreatedAt   time.Time
	AppliedAt   *time.Time
}

const (
	ImproveCategorySystemPrompt    = "system_prompt"
	ImproveCategoryIntentKeywords  = "intent_keywords"
	ImproveCategoryModelRouting    = "model_routing"
	ImproveCategoryReindexDocument = "reindex_document"
	ImproveCategoryEvalCase        = "eval_case"
	ImproveCategoryAddDocument     = "add_document"
)

var AutoApplyCategories = map[string]bool{
	ImproveCategorySystemPrompt:    true,
	ImproveCategoryIntentKeywords:  true,
	ImproveCategoryModelRouting:    true,
	ImproveCategoryReindexDocument: true,
	ImproveCategoryEvalCase:        true,
}
