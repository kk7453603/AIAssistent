# Scheduled Tasks — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a cron-based task scheduler that executes agent prompts on schedule, with conditional execution, webhook notifications, natural language scheduling via chat, and CRUD API.

**Architecture:** `SchedulerUseCase` runs a ticker loop in worker, checks cron expressions against current time using `robfig/cron/v3`, executes tasks via `AgentChatService.Complete()`. `ScheduleStore` persists tasks in Postgres. Chat integration extends existing `task_tool`. REST API for programmatic access.

**Tech Stack:** Go, `github.com/robfig/cron/v3`, Postgres, existing AgentChatService.

**Spec:** `docs/superpowers/specs/2026-03-28-scheduled-tasks.md`

---

### Task 1: Domain type + ScheduleStore port + Postgres implementation

**Files:**
- Create: `internal/core/domain/schedule.go`
- Modify: `internal/core/ports/outbound.go`
- Create: `internal/infrastructure/repository/postgres/schedule_repo.go`
- Modify: `internal/infrastructure/repository/postgres/document_repository.go` (DDL)

- [ ] **Step 1: Create domain type**

Create `internal/core/domain/schedule.go`:

```go
package domain

import "time"

// ScheduledTask represents a recurring agent task.
type ScheduledTask struct {
	ID         string
	UserID     string
	CronExpr   string
	Prompt     string
	Condition  string
	WebhookURL string
	Enabled    bool
	LastRunAt  *time.Time
	LastResult string
	LastStatus string // "success", "failed", "skipped"
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
```

- [ ] **Step 2: Add ScheduleStore port**

Add to `internal/core/ports/outbound.go`:

```go
// ScheduleStore persists and queries scheduled tasks.
type ScheduleStore interface {
	Create(ctx context.Context, task *domain.ScheduledTask) error
	ListByUser(ctx context.Context, userID string) ([]domain.ScheduledTask, error)
	ListEnabled(ctx context.Context) ([]domain.ScheduledTask, error)
	GetByID(ctx context.Context, id string) (*domain.ScheduledTask, error)
	Update(ctx context.Context, task *domain.ScheduledTask) error
	Delete(ctx context.Context, id string) error
	RecordRun(ctx context.Context, id string, result string, status string) error
}
```

- [ ] **Step 3: Implement Postgres ScheduleRepository**

Create `internal/infrastructure/repository/postgres/schedule_repo.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type ScheduleRepository struct {
	db *sql.DB
}

func NewScheduleRepository(db *sql.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

func (r *ScheduleRepository) Create(ctx context.Context, task *domain.ScheduledTask) error {
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
INSERT INTO scheduled_tasks (id, user_id, cron_expr, prompt, condition, webhook_url, enabled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`, task.ID, task.UserID, task.CronExpr, task.Prompt, task.Condition, task.WebhookURL, task.Enabled, task.CreatedAt, task.UpdatedAt)
	return err
}

func (r *ScheduleRepository) ListByUser(ctx context.Context, userID string) ([]domain.ScheduledTask, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, cron_expr, prompt, condition, webhook_url, enabled, last_run_at, last_result, last_status, created_at, updated_at
FROM scheduled_tasks WHERE user_id = $1 ORDER BY created_at DESC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSchedules(rows)
}

func (r *ScheduleRepository) ListEnabled(ctx context.Context) ([]domain.ScheduledTask, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, cron_expr, prompt, condition, webhook_url, enabled, last_run_at, last_result, last_status, created_at, updated_at
FROM scheduled_tasks WHERE enabled = true
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSchedules(rows)
}

func (r *ScheduleRepository) GetByID(ctx context.Context, id string) (*domain.ScheduledTask, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, user_id, cron_expr, prompt, condition, webhook_url, enabled, last_run_at, last_result, last_status, created_at, updated_at
FROM scheduled_tasks WHERE id = $1
`, id)
	var t domain.ScheduledTask
	var lastRunAt sql.NullTime
	var lastResult, lastStatus, condition, webhookURL sql.NullString
	err := row.Scan(&t.ID, &t.UserID, &t.CronExpr, &t.Prompt, &condition, &webhookURL,
		&t.Enabled, &lastRunAt, &lastResult, &lastStatus, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if lastRunAt.Valid {
		t.LastRunAt = &lastRunAt.Time
	}
	t.LastResult = lastResult.String
	t.LastStatus = lastStatus.String
	t.Condition = condition.String
	t.WebhookURL = webhookURL.String
	return &t, nil
}

func (r *ScheduleRepository) Update(ctx context.Context, task *domain.ScheduledTask) error {
	task.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
UPDATE scheduled_tasks
SET cron_expr = $2, prompt = $3, condition = $4, webhook_url = $5, enabled = $6, updated_at = $7
WHERE id = $1
`, task.ID, task.CronExpr, task.Prompt, task.Condition, task.WebhookURL, task.Enabled, task.UpdatedAt)
	return err
}

func (r *ScheduleRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM scheduled_tasks WHERE id = $1`, id)
	return err
}

func (r *ScheduleRepository) RecordRun(ctx context.Context, id, result, status string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
UPDATE scheduled_tasks SET last_run_at = $2, last_result = $3, last_status = $4, updated_at = $2 WHERE id = $1
`, id, now, result, status)
	return err
}

func scanSchedules(rows *sql.Rows) ([]domain.ScheduledTask, error) {
	var result []domain.ScheduledTask
	for rows.Next() {
		var t domain.ScheduledTask
		var lastRunAt sql.NullTime
		var lastResult, lastStatus, condition, webhookURL sql.NullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.CronExpr, &t.Prompt, &condition, &webhookURL,
			&t.Enabled, &lastRunAt, &lastResult, &lastStatus, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if lastRunAt.Valid {
			t.LastRunAt = &lastRunAt.Time
		}
		t.LastResult = lastResult.String
		t.LastStatus = lastStatus.String
		t.Condition = condition.String
		t.WebhookURL = webhookURL.String
		result = append(result, t)
	}
	return result, nil
}
```

- [ ] **Step 4: Add DDL to EnsureSchema**

In `document_repository.go`, add to `EnsureSchema` query:

```sql
CREATE TABLE IF NOT EXISTS scheduled_tasks (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	cron_expr TEXT NOT NULL,
	prompt TEXT NOT NULL,
	condition TEXT,
	webhook_url TEXT,
	enabled BOOLEAN NOT NULL DEFAULT true,
	last_run_at TIMESTAMPTZ,
	last_result TEXT,
	last_status TEXT,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_user_enabled
	ON scheduled_tasks(user_id, enabled, updated_at DESC);
```

- [ ] **Step 5: Run build**

Run: `go get github.com/robfig/cron/v3 && go build ./...`

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: ScheduledTask domain, ScheduleStore port, Postgres repo + DDL"
```

---

### Task 2: SchedulerUseCase — cron loop, execution, conditional, webhook

**Files:**
- Create: `internal/core/usecase/scheduler.go`
- Create: `internal/core/usecase/scheduler_test.go`

- [ ] **Step 1: Write tests**

Create `internal/core/usecase/scheduler_test.go`:

```go
package usecase

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestCronShouldRun(t *testing.T) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	sched, err := parser.Parse("* * * * *") // every minute
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	lastRun := time.Now().Add(-2 * time.Minute)
	next := sched.Next(lastRun)
	if !next.Before(time.Now()) {
		t.Error("expected every-minute cron to be due")
	}
}

func TestCronShouldNotRun(t *testing.T) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	sched, err := parser.Parse("0 0 1 1 *") // Jan 1st midnight
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	lastRun := time.Now().Add(-1 * time.Minute)
	next := sched.Next(lastRun)
	if next.Before(time.Now()) {
		t.Error("expected yearly cron to NOT be due")
	}
}

func TestTruncateScheduleResult(t *testing.T) {
	long := make([]byte, 5000)
	for i := range long {
		long[i] = 'a'
	}
	result := truncateScheduleResult(string(long), 1000)
	if len(result) > 1003 { // 1000 + "..."
		t.Errorf("expected truncated, got len %d", len(result))
	}
}
```

- [ ] **Step 2: Implement SchedulerUseCase**

Create `internal/core/usecase/scheduler.go`:

```go
package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// SchedulerUseCase runs scheduled tasks on a cron-like loop.
type SchedulerUseCase struct {
	store     ports.ScheduleStore
	agentChat ports.AgentChatService
	generator ports.AnswerGenerator
	parser    cron.Parser
}

func NewSchedulerUseCase(
	store ports.ScheduleStore,
	agentChat ports.AgentChatService,
	generator ports.AnswerGenerator,
) *SchedulerUseCase {
	return &SchedulerUseCase{
		store:     store,
		agentChat: agentChat,
		generator: generator,
		parser:    cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
	}
}

// Tick checks all enabled tasks and runs those that are due.
func (uc *SchedulerUseCase) Tick(ctx context.Context) {
	tasks, err := uc.store.ListEnabled(ctx)
	if err != nil {
		slog.Warn("scheduler_list_failed", "error", err)
		return
	}

	now := time.Now().UTC()
	for _, task := range tasks {
		if !uc.isDue(task, now) {
			continue
		}
		go uc.executeTask(ctx, task)
	}
}

func (uc *SchedulerUseCase) isDue(task domain.ScheduledTask, now time.Time) bool {
	sched, err := uc.parser.Parse(task.CronExpr)
	if err != nil {
		slog.Warn("scheduler_bad_cron", "task_id", task.ID, "cron", task.CronExpr, "error", err)
		return false
	}

	lastRun := task.CreatedAt
	if task.LastRunAt != nil {
		lastRun = *task.LastRunAt
	}

	next := sched.Next(lastRun)
	return !next.After(now)
}

func (uc *SchedulerUseCase) executeTask(ctx context.Context, task domain.ScheduledTask) {
	slog.Info("scheduler_task_started", "task_id", task.ID, "prompt", task.Prompt[:min(len(task.Prompt), 50)])

	// Conditional execution.
	if task.Condition != "" {
		should, err := uc.evaluateCondition(ctx, task.Condition)
		if err != nil {
			slog.Warn("scheduler_condition_error", "task_id", task.ID, "error", err)
		}
		if !should {
			_ = uc.store.RecordRun(ctx, task.ID, "", "skipped")
			slog.Info("scheduler_task_skipped", "task_id", task.ID, "condition", task.Condition)
			return
		}
	}

	// Execute via agent.
	req := domain.AgentChatRequest{
		UserID:         task.UserID,
		ConversationID: fmt.Sprintf("schedule_%s_%d", task.ID, time.Now().Unix()),
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: task.Prompt},
		},
	}

	result, err := uc.agentChat.Complete(ctx, req, nil)
	if err != nil {
		_ = uc.store.RecordRun(ctx, task.ID, err.Error(), "failed")
		slog.Error("scheduler_task_failed", "task_id", task.ID, "error", err)
		return
	}

	truncated := truncateScheduleResult(result.Answer, 2000)
	_ = uc.store.RecordRun(ctx, task.ID, truncated, "success")
	slog.Info("scheduler_task_completed", "task_id", task.ID)

	// Webhook notification.
	if task.WebhookURL != "" {
		uc.sendWebhook(task, result.Answer)
	}
}

func (uc *SchedulerUseCase) evaluateCondition(ctx context.Context, condition string) (bool, error) {
	prompt := fmt.Sprintf("Evaluate this condition and answer exactly 'yes' or 'no'. Nothing else.\n\nCondition: %s", condition)
	resp, err := uc.generator.GenerateFromPrompt(ctx, prompt)
	if err != nil {
		return true, err // on error, execute anyway
	}
	lower := strings.ToLower(strings.TrimSpace(resp))
	return lower == "yes" || lower == "да", nil
}

func (uc *SchedulerUseCase) sendWebhook(task domain.ScheduledTask, result string) {
	payload, _ := json.Marshal(map[string]string{
		"task_id":     task.ID,
		"prompt":      task.Prompt,
		"result":      result,
		"status":      "success",
		"executed_at": time.Now().UTC().Format(time.RFC3339),
	})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(task.WebhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		slog.Warn("scheduler_webhook_failed", "task_id", task.ID, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("scheduler_webhook_error", "task_id", task.ID, "status", resp.StatusCode)
	}
}

func truncateScheduleResult(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}

// ParseNaturalSchedule asks LLM to convert natural language to cron expression.
func (uc *SchedulerUseCase) ParseNaturalSchedule(ctx context.Context, input string) (cronExpr, prompt string, err error) {
	llmPrompt := fmt.Sprintf(`Convert this scheduling request to a cron expression and task prompt.
Return ONLY a JSON object: {"cron": "0 9 * * *", "prompt": "task description"}

Request: %s`, input)

	resp, err := uc.generator.GenerateJSONFromPrompt(ctx, llmPrompt)
	if err != nil {
		return "", "", fmt.Errorf("parse schedule: %w", err)
	}

	var result struct {
		Cron   string `json:"cron"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		// Try extract JSON.
		start := strings.Index(resp, "{")
		end := strings.LastIndex(resp, "}")
		if start >= 0 && end > start {
			_ = json.Unmarshal([]byte(resp[start:end+1]), &result)
		}
	}

	if result.Cron == "" {
		return "", "", fmt.Errorf("could not parse cron expression from: %s", input)
	}

	// Validate cron.
	if _, err := uc.parser.Parse(result.Cron); err != nil {
		return "", "", fmt.Errorf("invalid cron expression %q: %w", result.Cron, err)
	}

	return result.Cron, result.Prompt, nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/usecase/ -run "TestCron|TestTruncateSchedule" -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/usecase/scheduler.go internal/core/usecase/scheduler_test.go
git commit -m "feat(usecase): SchedulerUseCase with cron loop, conditional execution, webhooks"
```

---

### Task 3: REST API endpoints

**Files:**
- Modify: `internal/adapters/http/router.go`

- [ ] **Step 1: Add scheduleStore field and setter to Router**

```go
scheduleStore ports.ScheduleStore
```

```go
func (rt *Router) SetScheduleStore(s ports.ScheduleStore) {
	rt.scheduleStore = s
}
```

- [ ] **Step 2: Register routes**

Add to mux setup:

```go
mux.HandleFunc("POST /v1/schedules", rt.handleCreateSchedule)
mux.HandleFunc("GET /v1/schedules", rt.handleListSchedules)
mux.HandleFunc("DELETE /v1/schedules/{id}", rt.handleDeleteSchedule)
mux.HandleFunc("PATCH /v1/schedules/{id}", rt.handleUpdateSchedule)
```

- [ ] **Step 3: Add handlers**

```go
func (rt *Router) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CronExpr   string `json:"cron_expr"`
		Prompt     string `json:"prompt"`
		Condition  string `json:"condition"`
		WebhookURL string `json:"webhook_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.CronExpr == "" || req.Prompt == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("cron_expr and prompt are required"))
		return
	}
	if rt.scheduleStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("scheduler not enabled"))
		return
	}

	task := &domain.ScheduledTask{
		UserID:     r.Header.Get("X-User-ID"),
		CronExpr:   req.CronExpr,
		Prompt:     req.Prompt,
		Condition:  req.Condition,
		WebhookURL: req.WebhookURL,
		Enabled:    true,
	}
	if err := rt.scheduleStore.Create(r.Context(), task); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(task)
}

func (rt *Router) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	if rt.scheduleStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("scheduler not enabled"))
		return
	}
	userID := r.Header.Get("X-User-ID")
	tasks, err := rt.scheduleStore.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if tasks == nil {
		tasks = []domain.ScheduledTask{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tasks)
}

func (rt *Router) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	if rt.scheduleStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("scheduler not enabled"))
		return
	}
	id := r.PathValue("id")
	if err := rt.scheduleStore.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (rt *Router) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	if rt.scheduleStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("scheduler not enabled"))
		return
	}
	id := r.PathValue("id")
	existing, err := rt.scheduleStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	var req struct {
		CronExpr   *string `json:"cron_expr"`
		Prompt     *string `json:"prompt"`
		Condition  *string `json:"condition"`
		WebhookURL *string `json:"webhook_url"`
		Enabled    *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.CronExpr != nil {
		existing.CronExpr = *req.CronExpr
	}
	if req.Prompt != nil {
		existing.Prompt = *req.Prompt
	}
	if req.Condition != nil {
		existing.Condition = *req.Condition
	}
	if req.WebhookURL != nil {
		existing.WebhookURL = *req.WebhookURL
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := rt.scheduleStore.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(existing)
}
```

- [ ] **Step 4: Run build**

Run: `go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/http/router.go
git commit -m "feat(http): CRUD API endpoints for scheduled tasks"
```

---

### Task 4: Config + bootstrap + worker cron

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `cmd/worker/main.go`

- [ ] **Step 1: Add config fields**

In Config struct:

```go
SchedulerEnabled              bool
SchedulerCheckIntervalSeconds int
```

In Load():

```go
SchedulerEnabled:              mustEnvBool("SCHEDULER_ENABLED", false),
SchedulerCheckIntervalSeconds: mustEnvInt("SCHEDULER_CHECK_INTERVAL_SECONDS", 60),
```

- [ ] **Step 2: Wire in bootstrap**

Add `ScheduleStore ports.ScheduleStore` and `SchedulerUC *usecase.SchedulerUseCase` to App struct.

After repo setup:

```go
scheduleStore := postgres.NewScheduleRepository(db)
```

Conditionally create SchedulerUseCase:

```go
var schedulerUC *usecase.SchedulerUseCase
if cfg.SchedulerEnabled {
	schedulerUC = usecase.NewSchedulerUseCase(scheduleStore, agentUC, generator)
	slog.Info("scheduler_enabled", "interval_seconds", cfg.SchedulerCheckIntervalSeconds)
}
```

Add to App: `ScheduleStore: scheduleStore, SchedulerUC: schedulerUC`.

After router creation: `rt.SetScheduleStore(scheduleStore)`.

- [ ] **Step 3: Add scheduler goroutine to worker**

In `cmd/worker/main.go`, before `<-ctx.Done()`:

```go
// Scheduler cron loop.
if app.SchedulerUC != nil {
	go func() {
		interval := time.Duration(cfg.SchedulerCheckIntervalSeconds) * time.Second
		logger.Info("scheduler_started", "interval", interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				app.SchedulerUC.Tick(ctx)
			}
		}
	}()
}
```

- [ ] **Step 4: Run build and tests**

Run: `go build ./... && go test ./... -count=1`

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: wire scheduled tasks — config, bootstrap, worker cron loop, API"
```

---

### Task 5: Final verification + push

- [ ] **Step 1: Full test suite**

Run: `go test ./... -count=1 -v 2>&1 | grep -E "FAIL|ok"`

- [ ] **Step 2: Vet**

Run: `go vet ./...`

- [ ] **Step 3: Push**

```bash
git push origin main
```
