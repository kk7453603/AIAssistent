package domain

import "time"

type Conversation struct {
	UserID             string    `json:"user_id"`
	ConversationID     string    `json:"conversation_id"`
	CurrentUserTurn    int       `json:"current_user_turn"`
	LastSummaryEndTurn int       `json:"last_summary_end_turn"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ConversationMessage struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	ToolName       string    `json:"tool_name,omitempty"`
	UserTurn       int       `json:"user_turn"`
	CreatedAt      time.Time `json:"created_at"`
}

type TaskStatus string

const (
	TaskStatusOpen      TaskStatus = "open"
	TaskStatusCompleted TaskStatus = "completed"
)

type Task struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Title     string     `json:"title"`
	Details   string     `json:"details,omitempty"`
	Status    TaskStatus `json:"status"`
	DueAt     *time.Time `json:"due_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type MemorySummary struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ConversationID string    `json:"conversation_id"`
	TurnFrom       int       `json:"turn_from"`
	TurnTo         int       `json:"turn_to"`
	Summary        string    `json:"summary"`
	CreatedAt      time.Time `json:"created_at"`
}

type MemoryHit struct {
	Summary MemorySummary `json:"summary"`
	Score   float64       `json:"score"`
}

type AgentLimits struct {
	MaxIterations       int           `json:"max_iterations"`
	Timeout             time.Duration `json:"timeout"`
	PlannerTimeout      time.Duration `json:"planner_timeout"`
	ToolTimeout         time.Duration `json:"tool_timeout"`
	ShortMemoryMessages int           `json:"short_memory_messages"`
	SummaryEveryTurns   int           `json:"summary_every_turns"`
	MemoryTopK          int           `json:"memory_top_k"`
	KnowledgeTopK       int           `json:"knowledge_top_k"`
}

type AgentInputMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AgentChatRequest struct {
	UserID         string              `json:"user_id"`
	ConversationID string              `json:"conversation_id,omitempty"`
	SessionEnd     bool                `json:"session_end"`
	Messages       []AgentInputMessage `json:"messages"`
}

type AgentToolEvent struct {
	Tool   string `json:"tool"`
	Status string `json:"status"`
	Output string `json:"output"`
}

type AgentRunResult struct {
	ConversationID string           `json:"conversation_id"`
	Answer         string           `json:"answer"`
	Iterations     int              `json:"iterations"`
	MemoryHits     int              `json:"memory_hits"`
	SummaryCreated bool             `json:"summary_created"`
	ToolsInvoked   []string         `json:"tools_invoked,omitempty"`
	FallbackReason string           `json:"fallback_reason,omitempty"`
	ToolEvents     []AgentToolEvent `json:"tool_events,omitempty"`
}

type AgentPlanStep struct {
	Type   string                 `json:"type"`
	Tool   string                 `json:"tool,omitempty"`
	Action string                 `json:"action,omitempty"`
	Answer string                 `json:"answer,omitempty"`
	Input  map[string]interface{} `json:"input,omitempty"`
}
