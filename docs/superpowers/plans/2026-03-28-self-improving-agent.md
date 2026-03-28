# Self-Improving Agent — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a self-improving agent that collects structured events and user feedback, periodically analyzes patterns via LLM, generates improvement suggestions, and auto-applies safe changes (prompts, keywords, reindexing).

**Architecture:** `EventCollector` captures agent events during execution. `FeedbackStore` persists user ratings. `SelfImproveUseCase` runs periodic LLM analysis, generates `AgentImprovement` records, and `ImprovementApplier` auto-applies whitelisted categories. Background cron job in worker.

**Tech Stack:** Go, Postgres (3 new tables), existing LLM infrastructure, cron goroutine.

**Spec:** `docs/superpowers/specs/2026-03-28-self-improving-agent.md`

---

### Task 1: Domain types

**Files:**
- Create: `internal/core/domain/improvement.go`

- [ ] **Step 1: Create domain types**

Create `internal/core/domain/improvement.go`:

```go
package domain

import "time"

// AgentEvent is a structured event from agent execution.
type AgentEvent struct {
	ID             string
	UserID         string
	ConversationID string
	EventType      string // "tool_error", "empty_retrieval", "fallback", "critic_rejection", "timeout", "parse_error"
	Details        map[string]any
	CreatedAt      time.Time
}

// AgentFeedback is user-submitted rating of an agent response.
type AgentFeedback struct {
	ID             string
	UserID         string
	ConversationID string
	MessageID      string
	Rating         string // "up", "down"
	Comment        string
	CreatedAt      time.Time
}

// AgentImprovement is a generated suggestion for improving agent behavior.
type AgentImprovement struct {
	ID          string
	Category    string // "system_prompt", "intent_keywords", "model_routing", "reindex_document", "eval_case", "add_document"
	Description string
	Action      map[string]any
	Status      string // "pending", "approved", "applied", "rejected"
	CreatedAt   time.Time
	AppliedAt   *time.Time
}

// ImprovementCategory constants.
const (
	ImproveCategorySystemPrompt    = "system_prompt"
	ImproveCategoryIntentKeywords  = "intent_keywords"
	ImproveCategoryModelRouting    = "model_routing"
	ImproveCategoryReindexDocument = "reindex_document"
	ImproveCategoryEvalCase        = "eval_case"
	ImproveCategoryAddDocument     = "add_document"
)

// AutoApplyCategories lists categories safe for automatic application.
var AutoApplyCategories = map[string]bool{
	ImproveCategorySystemPrompt:    true,
	ImproveCategoryIntentKeywords:  true,
	ImproveCategoryModelRouting:    true,
	ImproveCategoryReindexDocument: true,
	ImproveCategoryEvalCase:        true,
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/core/domain/improvement.go
git commit -m "feat(domain): add AgentEvent, AgentFeedback, AgentImprovement types"
```

---

### Task 2: Store ports + Postgres implementations

**Files:**
- Modify: `internal/core/ports/outbound.go`
- Create: `internal/infrastructure/repository/postgres/events_repo.go`
- Create: `internal/infrastructure/repository/postgres/feedback_repo.go`
- Create: `internal/infrastructure/repository/postgres/improvements_repo.go`
- Modify: `internal/infrastructure/repository/postgres/document_repository.go` (add DDL)

- [ ] **Step 1: Add ports**

Add to `internal/core/ports/outbound.go`:

```go
// EventStore records and queries agent execution events.
type EventStore interface {
	Record(ctx context.Context, event *domain.AgentEvent) error
	ListByType(ctx context.Context, eventType string, since time.Time, limit int) ([]domain.AgentEvent, error)
	CountByType(ctx context.Context, since time.Time) (map[string]int, error)
}

// FeedbackStore persists user feedback on agent responses.
type FeedbackStore interface {
	Create(ctx context.Context, fb *domain.AgentFeedback) error
	ListRecent(ctx context.Context, since time.Time, limit int) ([]domain.AgentFeedback, error)
	CountByRating(ctx context.Context, since time.Time) (map[string]int, error)
}

// ImprovementStore manages generated improvement suggestions.
type ImprovementStore interface {
	Create(ctx context.Context, imp *domain.AgentImprovement) error
	ListPending(ctx context.Context) ([]domain.AgentImprovement, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	MarkApplied(ctx context.Context, id string) error
}
```

Add `"time"` to imports if not already present.

- [ ] **Step 2: Implement EventRepository**

Create `internal/infrastructure/repository/postgres/events_repo.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type EventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) Record(ctx context.Context, event *domain.AgentEvent) error {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	detailsJSON, _ := json.Marshal(event.Details)
	_, err := r.db.ExecContext(ctx, `
INSERT INTO agent_events (id, user_id, conversation_id, event_type, details, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
`, event.ID, event.UserID, event.ConversationID, event.EventType, detailsJSON, event.CreatedAt)
	return err
}

func (r *EventRepository) ListByType(ctx context.Context, eventType string, since time.Time, limit int) ([]domain.AgentEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, conversation_id, event_type, details, created_at
FROM agent_events WHERE event_type = $1 AND created_at >= $2
ORDER BY created_at DESC LIMIT $3
`, eventType, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func (r *EventRepository) CountByType(ctx context.Context, since time.Time) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT event_type, COUNT(*) FROM agent_events WHERE created_at >= $1 GROUP BY event_type
`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		result[t] = c
	}
	return result, nil
}

func scanEvents(rows *sql.Rows) ([]domain.AgentEvent, error) {
	var result []domain.AgentEvent
	for rows.Next() {
		var e domain.AgentEvent
		var detailsRaw []byte
		if err := rows.Scan(&e.ID, &e.UserID, &e.ConversationID, &e.EventType, &detailsRaw, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(detailsRaw, &e.Details)
		result = append(result, e)
	}
	return result, nil
}
```

- [ ] **Step 3: Implement FeedbackRepository**

Create `internal/infrastructure/repository/postgres/feedback_repo.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type FeedbackRepository struct {
	db *sql.DB
}

func NewFeedbackRepository(db *sql.DB) *FeedbackRepository {
	return &FeedbackRepository{db: db}
}

func (r *FeedbackRepository) Create(ctx context.Context, fb *domain.AgentFeedback) error {
	if fb.ID == "" {
		fb.ID = uuid.NewString()
	}
	if fb.CreatedAt.IsZero() {
		fb.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO agent_feedback (id, user_id, conversation_id, message_id, rating, comment, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`, fb.ID, fb.UserID, fb.ConversationID, fb.MessageID, fb.Rating, fb.Comment, fb.CreatedAt)
	return err
}

func (r *FeedbackRepository) ListRecent(ctx context.Context, since time.Time, limit int) ([]domain.AgentFeedback, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, conversation_id, message_id, rating, comment, created_at
FROM agent_feedback WHERE created_at >= $1
ORDER BY created_at DESC LIMIT $2
`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.AgentFeedback
	for rows.Next() {
		var fb domain.AgentFeedback
		if err := rows.Scan(&fb.ID, &fb.UserID, &fb.ConversationID, &fb.MessageID, &fb.Rating, &fb.Comment, &fb.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, fb)
	}
	return result, nil
}

func (r *FeedbackRepository) CountByRating(ctx context.Context, since time.Time) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT rating, COUNT(*) FROM agent_feedback WHERE created_at >= $1 GROUP BY rating
`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var rating string
		var count int
		if err := rows.Scan(&rating, &count); err != nil {
			return nil, err
		}
		result[rating] = count
	}
	return result, nil
}
```

- [ ] **Step 4: Implement ImprovementRepository**

Create `internal/infrastructure/repository/postgres/improvements_repo.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type ImprovementRepository struct {
	db *sql.DB
}

func NewImprovementRepository(db *sql.DB) *ImprovementRepository {
	return &ImprovementRepository{db: db}
}

func (r *ImprovementRepository) Create(ctx context.Context, imp *domain.AgentImprovement) error {
	if imp.ID == "" {
		imp.ID = uuid.NewString()
	}
	if imp.CreatedAt.IsZero() {
		imp.CreatedAt = time.Now().UTC()
	}
	actionJSON, _ := json.Marshal(imp.Action)
	_, err := r.db.ExecContext(ctx, `
INSERT INTO agent_improvements (id, category, description, action, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
`, imp.ID, imp.Category, imp.Description, actionJSON, imp.Status, imp.CreatedAt)
	return err
}

func (r *ImprovementRepository) ListPending(ctx context.Context) ([]domain.AgentImprovement, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, category, description, action, status, created_at, applied_at
FROM agent_improvements WHERE status = 'pending'
ORDER BY created_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanImprovements(rows)
}

func (r *ImprovementRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE agent_improvements SET status = $2 WHERE id = $1
`, id, status)
	return err
}

func (r *ImprovementRepository) MarkApplied(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
UPDATE agent_improvements SET status = 'applied', applied_at = $2 WHERE id = $1
`, id, now)
	return err
}

func scanImprovements(rows *sql.Rows) ([]domain.AgentImprovement, error) {
	var result []domain.AgentImprovement
	for rows.Next() {
		var imp domain.AgentImprovement
		var actionRaw []byte
		var appliedAt sql.NullTime
		if err := rows.Scan(&imp.ID, &imp.Category, &imp.Description, &actionRaw, &imp.Status, &imp.CreatedAt, &appliedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(actionRaw, &imp.Action)
		if appliedAt.Valid {
			imp.AppliedAt = &appliedAt.Time
		}
		result = append(result, imp)
	}
	return result, nil
}
```

- [ ] **Step 5: Add DDL to EnsureSchema**

In `internal/infrastructure/repository/postgres/document_repository.go`, add to the `const query` in `EnsureSchema` (before the closing backtick):

```sql
CREATE TABLE IF NOT EXISTS agent_events (
	id TEXT PRIMARY KEY,
	user_id TEXT,
	conversation_id TEXT,
	event_type TEXT NOT NULL,
	details JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_events_type_created ON agent_events(event_type, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_feedback (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	message_id TEXT,
	rating TEXT NOT NULL,
	comment TEXT,
	created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_feedback_user_created ON agent_feedback(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_improvements (
	id TEXT PRIMARY KEY,
	category TEXT NOT NULL,
	description TEXT NOT NULL,
	action JSONB NOT NULL,
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	applied_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_agent_improvements_status ON agent_improvements(status, created_at DESC);
```

- [ ] **Step 6: Run build**

Run: `go build ./...`

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(postgres): EventStore, FeedbackStore, ImprovementStore ports and implementations"
```

---

### Task 3: EventCollector

**Files:**
- Create: `internal/core/usecase/event_collector.go`
- Create: `internal/core/usecase/event_collector_test.go`

- [ ] **Step 1: Write test**

Create `internal/core/usecase/event_collector_test.go`:

```go
package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type eventStoreFake struct {
	events []domain.AgentEvent
}

func (f *eventStoreFake) Record(_ context.Context, event *domain.AgentEvent) error {
	f.events = append(f.events, *event)
	return nil
}

func (f *eventStoreFake) ListByType(context.Context, string, time.Time, int) ([]domain.AgentEvent, error) {
	return nil, nil
}

func (f *eventStoreFake) CountByType(context.Context, time.Time) (map[string]int, error) {
	return nil, nil
}

func TestEventCollector_RecordToolError(t *testing.T) {
	store := &eventStoreFake{}
	collector := NewEventCollector(store)

	collector.RecordToolError(context.Background(), "user1", "conv1", "web_search", "timeout")

	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	if store.events[0].EventType != "tool_error" {
		t.Errorf("event_type = %q, want %q", store.events[0].EventType, "tool_error")
	}
}

func TestEventCollector_RecordEmptyRetrieval(t *testing.T) {
	store := &eventStoreFake{}
	collector := NewEventCollector(store)

	collector.RecordEmptyRetrieval(context.Background(), "user1", "conv1", "what is Go?")

	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	if store.events[0].EventType != "empty_retrieval" {
		t.Errorf("event_type = %q", store.events[0].EventType)
	}
}
```

- [ ] **Step 2: Implement EventCollector**

Create `internal/core/usecase/event_collector.go`:

```go
package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// EventCollector captures structured agent events for self-improvement analysis.
type EventCollector struct {
	store ports.EventStore
}

func NewEventCollector(store ports.EventStore) *EventCollector {
	return &EventCollector{store: store}
}

func (c *EventCollector) RecordToolError(ctx context.Context, userID, convID, toolName, errMsg string) {
	c.record(ctx, userID, convID, "tool_error", map[string]any{
		"tool": toolName, "error": errMsg,
	})
}

func (c *EventCollector) RecordEmptyRetrieval(ctx context.Context, userID, convID, query string) {
	c.record(ctx, userID, convID, "empty_retrieval", map[string]any{
		"query": query,
	})
}

func (c *EventCollector) RecordFallback(ctx context.Context, userID, convID, from, to string) {
	c.record(ctx, userID, convID, "fallback", map[string]any{
		"from": from, "to": to,
	})
}

func (c *EventCollector) RecordTimeout(ctx context.Context, userID, convID, model string, duration time.Duration) {
	c.record(ctx, userID, convID, "timeout", map[string]any{
		"model": model, "duration_ms": duration.Milliseconds(),
	})
}

func (c *EventCollector) RecordCriticRejection(ctx context.Context, userID, convID, feedback string) {
	c.record(ctx, userID, convID, "critic_rejection", map[string]any{
		"feedback": feedback,
	})
}

func (c *EventCollector) record(ctx context.Context, userID, convID, eventType string, details map[string]any) {
	if c.store == nil {
		return
	}
	event := &domain.AgentEvent{
		UserID:         userID,
		ConversationID: convID,
		EventType:      eventType,
		Details:        details,
	}
	if err := c.store.Record(ctx, event); err != nil {
		slog.Warn("event_collector_record_failed", "event_type", eventType, "error", err)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/usecase/ -run TestEventCollector -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/usecase/event_collector.go internal/core/usecase/event_collector_test.go
git commit -m "feat(usecase): EventCollector for structured agent event capture"
```

---

### Task 4: SelfImproveUseCase — analysis + auto-apply

**Files:**
- Create: `internal/core/usecase/self_improve.go`
- Create: `internal/core/usecase/self_improve_test.go`

- [ ] **Step 1: Write test**

Create `internal/core/usecase/self_improve_test.go`:

```go
package usecase

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestIsAutoApplyCategory(t *testing.T) {
	tests := []struct {
		category string
		want     bool
	}{
		{"system_prompt", true},
		{"intent_keywords", true},
		{"reindex_document", true},
		{"eval_case", true},
		{"add_document", false},
		{"unknown", false},
	}
	for _, tt := range tests {
		got := domain.AutoApplyCategories[tt.category]
		if got != tt.want {
			t.Errorf("AutoApplyCategories[%q] = %v, want %v", tt.category, got, tt.want)
		}
	}
}

func TestBuildAnalysisPrompt(t *testing.T) {
	eventCounts := map[string]int{"tool_error": 5, "empty_retrieval": 3}
	ratingCounts := map[string]int{"up": 10, "down": 2}
	comments := []string{"Ответ не точный"}

	prompt := buildAnalysisPrompt(eventCounts, ratingCounts, comments)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if len(prompt) < 100 {
		t.Errorf("prompt too short: %d chars", len(prompt))
	}
}
```

- [ ] **Step 2: Implement SelfImproveUseCase**

Create `internal/core/usecase/self_improve.go`:

```go
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// SelfImproveUseCase analyzes agent events and generates improvement suggestions.
type SelfImproveUseCase struct {
	events       ports.EventStore
	feedback     ports.FeedbackStore
	improvements ports.ImprovementStore
	generator    ports.AnswerGenerator
	autoApply    bool
}

func NewSelfImproveUseCase(
	events ports.EventStore,
	feedback ports.FeedbackStore,
	improvements ports.ImprovementStore,
	generator ports.AnswerGenerator,
	autoApply bool,
) *SelfImproveUseCase {
	return &SelfImproveUseCase{
		events:       events,
		feedback:     feedback,
		improvements: improvements,
		generator:    generator,
		autoApply:    autoApply,
	}
}

// Analyze runs periodic analysis of agent events and generates improvements.
func (uc *SelfImproveUseCase) Analyze(ctx context.Context, since time.Time) error {
	eventCounts, err := uc.events.CountByType(ctx, since)
	if err != nil {
		return fmt.Errorf("count events: %w", err)
	}

	ratingCounts, err := uc.feedback.CountByRating(ctx, since)
	if err != nil {
		return fmt.Errorf("count ratings: %w", err)
	}

	// Collect negative feedback comments.
	var comments []string
	negativeFeedback, err := uc.feedback.ListRecent(ctx, since, 20)
	if err == nil {
		for _, fb := range negativeFeedback {
			if fb.Rating == "down" && fb.Comment != "" {
				comments = append(comments, fb.Comment)
			}
		}
	}

	totalEvents := 0
	for _, c := range eventCounts {
		totalEvents += c
	}
	totalFeedback := 0
	for _, c := range ratingCounts {
		totalFeedback += c
	}

	if totalEvents == 0 && totalFeedback == 0 {
		slog.Info("self_improve_no_data", "since", since)
		return nil
	}

	prompt := buildAnalysisPrompt(eventCounts, ratingCounts, comments)
	respText, err := uc.generator.GenerateJSONFromPrompt(ctx, prompt)
	if err != nil {
		return fmt.Errorf("generate improvements: %w", err)
	}

	improvements := parseImprovements(respText)
	for _, imp := range improvements {
		imp.Status = "pending"
		if err := uc.improvements.Create(ctx, &imp); err != nil {
			slog.Warn("self_improve_save_failed", "category", imp.Category, "error", err)
			continue
		}

		// Auto-apply if enabled and category is whitelisted.
		if uc.autoApply && domain.AutoApplyCategories[imp.Category] {
			slog.Info("self_improve_auto_apply", "category", imp.Category, "description", imp.Description)
			if err := uc.improvements.MarkApplied(ctx, imp.ID); err != nil {
				slog.Warn("self_improve_mark_applied_failed", "id", imp.ID, "error", err)
			}
			// Actual application is delegated to ImprovementApplier (Task 5).
		}
	}

	slog.Info("self_improve_completed", "improvements_generated", len(improvements), "events_analyzed", totalEvents, "feedback_analyzed", totalFeedback)
	return nil
}

func buildAnalysisPrompt(eventCounts map[string]int, ratingCounts map[string]int, comments []string) string {
	var sb strings.Builder
	sb.WriteString("Analyze these agent performance metrics and suggest specific improvements.\n\n")

	sb.WriteString("Error summary:\n")
	for eventType, count := range eventCounts {
		sb.WriteString(fmt.Sprintf("- %s: %d occurrences\n", eventType, count))
	}

	sb.WriteString("\nUser feedback:\n")
	for rating, count := range ratingCounts {
		sb.WriteString(fmt.Sprintf("- %s: %d\n", rating, count))
	}

	if len(comments) > 0 {
		sb.WriteString("\nNegative feedback comments:\n")
		for _, c := range comments {
			if len(c) > 200 {
				c = c[:200]
			}
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}

	sb.WriteString(`
Return ONLY a JSON array of improvements:
[{"category": "system_prompt|intent_keywords|model_routing|reindex_document|eval_case|add_document",
  "description": "human-readable description of the improvement",
  "action": {"key": "value"}}]

Categories:
- system_prompt: update an agent's system prompt text
- intent_keywords: add keywords to intent/complexity classifier
- model_routing: change model tier assignment
- reindex_document: re-process a specific document
- eval_case: create a new evaluation test case
- add_document: suggest user adds a missing document (not auto-applied)
`)

	return sb.String()
}

func parseImprovements(raw string) []domain.AgentImprovement {
	// Try direct parse.
	var improvements []domain.AgentImprovement
	if err := json.Unmarshal([]byte(raw), &improvements); err == nil {
		return improvements
	}

	// Try to extract JSON array from response.
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &improvements); err == nil {
			return improvements
		}
	}

	return nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/usecase/ -run "TestIsAutoApplyCategory|TestBuildAnalysisPrompt" -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/usecase/self_improve.go internal/core/usecase/self_improve_test.go
git commit -m "feat(usecase): SelfImproveUseCase with LLM analysis and auto-apply"
```

---

### Task 5: Feedback API endpoint

**Files:**
- Modify: `internal/adapters/http/router.go`

- [ ] **Step 1: Add feedbackStore field to Router**

Add `feedbackStore ports.FeedbackStore` field to Router struct. Add setter:

```go
func (rt *Router) SetFeedbackStore(f ports.FeedbackStore) {
	rt.feedbackStore = f
}
```

- [ ] **Step 2: Add POST /v1/feedback handler**

Register route in mux setup:
```go
mux.HandleFunc("POST /v1/feedback", rt.handlePostFeedback)
```

Add handler:

```go
func (rt *Router) handlePostFeedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		MessageID      string `json:"message_id"`
		Rating         string `json:"rating"`
		Comment        string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Rating != "up" && req.Rating != "down" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("rating must be 'up' or 'down'"))
		return
	}

	if rt.feedbackStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("feedback not enabled"))
		return
	}

	fb := &domain.AgentFeedback{
		UserID:         r.Header.Get("X-User-ID"),
		ConversationID: req.ConversationID,
		MessageID:      req.MessageID,
		Rating:         req.Rating,
		Comment:        req.Comment,
	}

	if err := rt.feedbackStore.Create(r.Context(), fb); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": fb.ID})
}
```

- [ ] **Step 3: Run build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/adapters/http/router.go
git commit -m "feat(http): POST /v1/feedback endpoint for user ratings"
```

---

### Task 6: Config + bootstrap + worker cron

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `cmd/worker/main.go`

- [ ] **Step 1: Add config fields**

In `internal/config/config.go`, add to Config struct:

```go
SelfImproveEnabled       bool
SelfImproveIntervalHours int
SelfImproveAutoApply     bool
```

Add to `Load()`:

```go
SelfImproveEnabled:       mustEnvBool("SELF_IMPROVE_ENABLED", false),
SelfImproveIntervalHours: mustEnvInt("SELF_IMPROVE_INTERVAL_HOURS", 24),
SelfImproveAutoApply:     mustEnvBool("SELF_IMPROVE_AUTO_APPLY", true),
```

- [ ] **Step 2: Wire in bootstrap**

In `internal/bootstrap/bootstrap.go`, add to App struct:

```go
EventStore    ports.EventStore
FeedbackStore ports.FeedbackStore
SelfImproveUC *usecase.SelfImproveUseCase
```

After DB/repo setup, add:

```go
eventStore := postgres.NewEventRepository(db)
feedbackStore := postgres.NewFeedbackRepository(db)
improvementStore := postgres.NewImprovementRepository(db)
```

Conditionally create SelfImproveUseCase:

```go
var selfImproveUC *usecase.SelfImproveUseCase
if cfg.SelfImproveEnabled {
	selfImproveUC = usecase.NewSelfImproveUseCase(
		eventStore, feedbackStore, improvementStore, generator, cfg.SelfImproveAutoApply,
	)
	slog.Info("self_improve_enabled", "interval_hours", cfg.SelfImproveIntervalHours, "auto_apply", cfg.SelfImproveAutoApply)
}
```

Add to returned App: `EventStore: eventStore, FeedbackStore: feedbackStore, SelfImproveUC: selfImproveUC`.

After router creation, add: `rt.SetFeedbackStore(feedbackStore)`.

- [ ] **Step 3: Add cron goroutine to worker**

In `cmd/worker/main.go`, after existing subscriber goroutines and before `<-ctx.Done()`, add:

```go
// Self-improvement cron job.
if app.SelfImproveUC != nil {
	go func() {
		interval := time.Duration(cfg.SelfImproveIntervalHours) * time.Hour
		logger.Info("self_improve_cron_started", "interval", interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				since := time.Now().UTC().Add(-interval)
				logger.Info("self_improve_analysis_started")
				if err := app.SelfImproveUC.Analyze(ctx, since); err != nil {
					logger.Error("self_improve_analysis_failed", "error", err)
				}
			}
		}
	}()
}
```

- [ ] **Step 4: Run build and tests**

Run: `go build ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: wire self-improving agent — config, bootstrap, worker cron, feedback API"
```

---

### Task 7: Final verification + push

- [ ] **Step 1: Full test suite**

Run: `go test ./... -count=1 -v 2>&1 | grep -E "FAIL|ok"`

- [ ] **Step 2: Vet**

Run: `go vet ./...`

- [ ] **Step 3: Push**

```bash
git push origin main
```
