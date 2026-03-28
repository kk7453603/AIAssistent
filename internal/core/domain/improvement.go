package domain

import "time"

type AgentEvent struct {
	ID             string         `json:"id"`
	UserID         string         `json:"user_id"`
	ConversationID string         `json:"conversation_id"`
	EventType      string         `json:"event_type"`
	Details        map[string]any `json:"details"`
	CreatedAt      time.Time      `json:"created_at"`
}

type AgentFeedback struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ConversationID string    `json:"conversation_id"`
	MessageID      string    `json:"message_id,omitempty"`
	Rating         string    `json:"rating"`
	Comment        string    `json:"comment,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type AgentImprovement struct {
	ID          string         `json:"id"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Action      map[string]any `json:"action"`
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	AppliedAt   *time.Time     `json:"applied_at,omitempty"`
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
