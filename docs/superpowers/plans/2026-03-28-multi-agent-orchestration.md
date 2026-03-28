# Multi-Agent Orchestration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a dynamic multi-agent orchestrator with configurable specialist agents (researcher, coder, writer, critic), shared Qdrant memory, streaming status, and persistent Postgres history.

**Architecture:** `OrchestratorUseCase` plans which agents to call, executes them sequentially via `AgentChatService`, shares context through Qdrant memory. `AgentRegistry` holds configurable specs. `OrchestrationStore` persists history to Postgres. Orchestrator integrates into existing `AgentChatUseCase` transparently — triggered for complex tasks.

**Tech Stack:** Go, existing AgentChatService, Qdrant MemoryVectorStore, Postgres, SSE streaming.

**Spec:** `docs/superpowers/specs/2026-03-28-multi-agent-orchestration.md`

---

### Task 1: Domain types for orchestration

**Files:**
- Create: `internal/core/domain/orchestration.go`

- [ ] **Step 1: Create orchestration domain types**

Create `internal/core/domain/orchestration.go`:

```go
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
	Index     int       `json:"index"`
	Agent     string    `json:"agent"`
	Task      string    `json:"task"`
	Result    string    `json:"result"`
	Status    string    `json:"status"` // "completed", "failed"
	StartedAt time.Time `json:"started_at"`
	DurationMS float64  `json:"duration_ms"`
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
	Status          string `json:"status"` // "started", "completed", "failed"
	Result          string `json:"result,omitempty"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/core/domain/orchestration.go
git commit -m "feat(domain): add orchestration types — AgentSpec, Orchestration, OrchestrationStep"
```

---

### Task 2: OrchestrationStore port + Postgres implementation

**Files:**
- Modify: `internal/core/ports/outbound.go`
- Create: `internal/infrastructure/repository/postgres/orch_repo.go`

- [ ] **Step 1: Add OrchestrationStore port**

Add to `internal/core/ports/outbound.go`:

```go
// OrchestrationStore persists multi-agent orchestration history.
type OrchestrationStore interface {
	Create(ctx context.Context, orch *domain.Orchestration) error
	AddStep(ctx context.Context, orchID string, step domain.OrchestrationStep) error
	Complete(ctx context.Context, orchID string, status string) error
	GetByID(ctx context.Context, orchID string) (*domain.Orchestration, error)
	ListByUser(ctx context.Context, userID string, limit int) ([]domain.Orchestration, error)
}
```

- [ ] **Step 2: Implement Postgres OrchestrationRepository**

Create `internal/infrastructure/repository/postgres/orch_repo.go`:

```go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type OrchestrationRepository struct {
	db *sql.DB
}

func NewOrchestrationRepository(db *sql.DB) *OrchestrationRepository {
	return &OrchestrationRepository{db: db}
}

func (r *OrchestrationRepository) EnsureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS orchestrations (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	request TEXT NOT NULL,
	plan JSONB NOT NULL DEFAULT '[]'::jsonb,
	steps JSONB NOT NULL DEFAULT '[]'::jsonb,
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_orchestrations_user_conv
	ON orchestrations(user_id, conversation_id, created_at DESC);
`)
	return err
}

func (r *OrchestrationRepository) Create(ctx context.Context, orch *domain.Orchestration) error {
	planJSON, _ := json.Marshal(orch.Plan)
	stepsJSON, _ := json.Marshal(orch.Steps)

	_, err := r.db.ExecContext(ctx, `
INSERT INTO orchestrations (id, user_id, conversation_id, request, plan, steps, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`, orch.ID, orch.UserID, orch.ConversationID, orch.Request,
		planJSON, stepsJSON, orch.Status, orch.CreatedAt)
	return err
}

func (r *OrchestrationRepository) AddStep(ctx context.Context, orchID string, step domain.OrchestrationStep) error {
	stepJSON, _ := json.Marshal(step)
	_, err := r.db.ExecContext(ctx, `
UPDATE orchestrations
SET steps = steps || $2::jsonb, updated_at = $3
WHERE id = $1
`, orchID, fmt.Sprintf("[%s]", stepJSON), time.Now().UTC())
	return err
}

func (r *OrchestrationRepository) Complete(ctx context.Context, orchID string, status string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
UPDATE orchestrations SET status = $2, completed_at = $3 WHERE id = $1
`, orchID, status, now)
	return err
}

func (r *OrchestrationRepository) GetByID(ctx context.Context, orchID string) (*domain.Orchestration, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, user_id, conversation_id, request, plan, steps, status, created_at, completed_at
FROM orchestrations WHERE id = $1
`, orchID)

	var orch domain.Orchestration
	var planRaw, stepsRaw []byte
	var completedAt sql.NullTime

	err := row.Scan(&orch.ID, &orch.UserID, &orch.ConversationID, &orch.Request,
		&planRaw, &stepsRaw, &orch.Status, &orch.CreatedAt, &completedAt)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(planRaw, &orch.Plan)
	_ = json.Unmarshal(stepsRaw, &orch.Steps)
	if completedAt.Valid {
		orch.CompletedAt = &completedAt.Time
	}
	return &orch, nil
}

func (r *OrchestrationRepository) ListByUser(ctx context.Context, userID string, limit int) ([]domain.Orchestration, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, conversation_id, request, plan, steps, status, created_at, completed_at
FROM orchestrations WHERE user_id = $1
ORDER BY created_at DESC LIMIT $2
`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Orchestration
	for rows.Next() {
		var o domain.Orchestration
		var planRaw, stepsRaw []byte
		var completedAt sql.NullTime
		if err := rows.Scan(&o.ID, &o.UserID, &o.ConversationID, &o.Request,
			&planRaw, &stepsRaw, &o.Status, &o.CreatedAt, &completedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(planRaw, &o.Plan)
		_ = json.Unmarshal(stepsRaw, &o.Steps)
		if completedAt.Valid {
			o.CompletedAt = &completedAt.Time
		}
		result = append(result, o)
	}
	return result, nil
}
```

- [ ] **Step 3: Add schema to EnsureSchema in document_repository.go**

In `internal/infrastructure/repository/postgres/document_repository.go`, add the orchestrations table DDL to the existing `EnsureSchema` const query (before the closing backtick):

```sql
CREATE TABLE IF NOT EXISTS orchestrations (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	request TEXT NOT NULL,
	plan JSONB NOT NULL DEFAULT '[]'::jsonb,
	steps JSONB NOT NULL DEFAULT '[]'::jsonb,
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_orchestrations_user_conv
	ON orchestrations(user_id, conversation_id, created_at DESC);
```

- [ ] **Step 4: Run build**

Run: `go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/core/ports/outbound.go internal/infrastructure/repository/postgres/orch_repo.go internal/infrastructure/repository/postgres/document_repository.go
git commit -m "feat(postgres): OrchestrationStore port and Postgres implementation"
```

---

### Task 3: AgentRegistry

**Files:**
- Create: `internal/core/usecase/agent_registry.go`
- Create: `internal/core/usecase/agent_registry_test.go`

- [ ] **Step 1: Write tests**

Create `internal/core/usecase/agent_registry_test.go`:

```go
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
	reg := NewAgentRegistry(nil) // nil → use defaults
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
```

- [ ] **Step 2: Implement AgentRegistry**

Create `internal/core/usecase/agent_registry.go`:

```go
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

// AgentRegistry holds configurable specialist agent definitions.
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
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/usecase/ -run TestAgentRegistry -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/usecase/agent_registry.go internal/core/usecase/agent_registry_test.go
git commit -m "feat(usecase): AgentRegistry with configurable and default specialist specs"
```

---

### Task 4: OrchestratorUseCase — core logic

**Files:**
- Create: `internal/core/usecase/orchestrator.go`
- Create: `internal/core/usecase/orchestrator_test.go`

- [ ] **Step 1: Implement OrchestratorUseCase**

Create `internal/core/usecase/orchestrator.go`:

```go
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// OrchestratorUseCase coordinates multiple specialist agents for complex tasks.
type OrchestratorUseCase struct {
	agentChat    ports.AgentChatService
	registry     *AgentRegistry
	memoryVector ports.MemoryVectorStore
	embedder     ports.Embedder
	generator    ports.AnswerGenerator
	orchStore    ports.OrchestrationStore
	maxSteps     int
}

func NewOrchestratorUseCase(
	agentChat ports.AgentChatService,
	registry *AgentRegistry,
	memoryVector ports.MemoryVectorStore,
	embedder ports.Embedder,
	generator ports.AnswerGenerator,
	orchStore ports.OrchestrationStore,
	maxSteps int,
) *OrchestratorUseCase {
	if maxSteps <= 0 {
		maxSteps = 8
	}
	return &OrchestratorUseCase{
		agentChat:    agentChat,
		registry:     registry,
		memoryVector: memoryVector,
		embedder:     embedder,
		generator:    generator,
		orchStore:    orchStore,
		maxSteps:     maxSteps,
	}
}

func (uc *OrchestratorUseCase) Execute(
	ctx context.Context,
	req domain.AgentChatRequest,
	onToolStatus domain.ToolStatusCallback,
) (*domain.AgentRunResult, error) {
	orchID := uuid.NewString()
	now := time.Now().UTC()
	lastMessage := lastUserMsg(req.Messages)

	// 1. Plan — determine which agents to call.
	plan, err := uc.planSteps(ctx, lastMessage)
	if err != nil {
		slog.Warn("orchestrator_plan_failed", "error", err)
		// Fallback to single-agent.
		return uc.agentChat.Complete(ctx, req, onToolStatus)
	}

	// 2. Persist orchestration.
	orch := &domain.Orchestration{
		ID:             orchID,
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		Request:        lastMessage,
		Plan:           plan,
		Status:         "running",
		CreatedAt:      now,
	}
	if uc.orchStore != nil {
		_ = uc.orchStore.Create(ctx, orch)
	}

	// 3. Execute steps.
	var lastResult string
	stepIndex := 0

	for stepIndex < len(plan) && stepIndex < uc.maxSteps {
		step := plan[stepIndex]
		spec, ok := uc.registry.Get(step.Agent)
		if !ok {
			slog.Warn("orchestrator_unknown_agent", "agent", step.Agent)
			stepIndex++
			continue
		}

		// Notify streaming.
		if onToolStatus != nil {
			onToolStatus(fmt.Sprintf("orchestrator:%s", step.Agent), "started")
		}

		startTime := time.Now()

		// Build context from shared memory.
		memoryContext := uc.gatherMemoryContext(ctx, req.UserID, req.ConversationID, step.Task)

		// Build agent request with custom system prompt.
		agentPrompt := fmt.Sprintf("%s\n\nTask: %s", spec.SystemPrompt, step.Task)
		if memoryContext != "" {
			agentPrompt += fmt.Sprintf("\n\nContext from previous steps:\n%s", memoryContext)
		}
		if lastResult != "" {
			agentPrompt += fmt.Sprintf("\n\nPrevious agent result:\n%s", truncate(lastResult, 2000))
		}

		agentReq := domain.AgentChatRequest{
			UserID:         req.UserID,
			ConversationID: req.ConversationID + "_orch_" + orchID,
			Messages: []domain.ChatMessage{
				{Role: "system", Content: agentPrompt},
				{Role: "user", Content: lastMessage},
			},
			Model: req.Model,
		}

		result, err := uc.agentChat.Complete(ctx, agentReq, nil)

		duration := float64(time.Since(startTime).Microseconds()) / 1000.0
		orchStep := domain.OrchestrationStep{
			Index:     stepIndex,
			Agent:     step.Agent,
			Task:      step.Task,
			Status:    "completed",
			StartedAt: startTime,
			DurationMS: duration,
		}

		if err != nil {
			orchStep.Status = "failed"
			orchStep.Result = err.Error()
			slog.Warn("orchestrator_step_failed", "agent", step.Agent, "error", err)
		} else {
			orchStep.Result = result.Answer
			lastResult = result.Answer

			// Save to shared memory.
			uc.saveToMemory(ctx, req.UserID, req.ConversationID, orchID, step.Agent, result.Answer)
		}

		if uc.orchStore != nil {
			_ = uc.orchStore.AddStep(ctx, orchID, orchStep)
		}

		if onToolStatus != nil {
			status := "completed"
			if orchStep.Status == "failed" {
				status = "failed"
			}
			onToolStatus(fmt.Sprintf("orchestrator:%s", step.Agent), status)
		}

		// Dynamic routing: if critic found issues, add writer fix step.
		if step.Agent == "critic" && orchStep.Status == "completed" {
			if containsCriticIssues(orchStep.Result) && stepIndex+2 < uc.maxSteps {
				plan = append(plan,
					domain.OrchestrationPlanStep{Agent: "writer", Task: "Fix issues found by critic: " + truncate(orchStep.Result, 500)},
					domain.OrchestrationPlanStep{Agent: "critic", Task: "Re-check the fixed answer"},
				)
			}
		}

		stepIndex++
	}

	// 4. Complete orchestration.
	if uc.orchStore != nil {
		_ = uc.orchStore.Complete(ctx, orchID, "completed")
	}

	slog.Info("orchestration_completed", "id", orchID, "steps", stepIndex)

	return &domain.AgentRunResult{
		Answer:     lastResult,
		ToolEvents: nil,
	}, nil
}

func (uc *OrchestratorUseCase) planSteps(ctx context.Context, userMessage string) ([]domain.OrchestrationPlanStep, error) {
	agentNames := uc.registry.Names()
	prompt := fmt.Sprintf(`You are an orchestrator. Given the user request, decide which specialist agents to call and in what order.

Available specialists: %s

Return ONLY a JSON object: {"steps": [{"agent": "<name>", "task": "<specific task for this agent>"}]}
Do not include any explanation, only the JSON.

User request: %s`, strings.Join(agentNames, ", "), userMessage)

	respText, err := uc.generator.GenerateJSONFromPrompt(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("plan generation: %w", err)
	}

	var plan struct {
		Steps []domain.OrchestrationPlanStep `json:"steps"`
	}
	if err := json.Unmarshal([]byte(respText), &plan); err != nil {
		// Try to extract JSON.
		start := strings.Index(respText, "{")
		end := strings.LastIndex(respText, "}")
		if start >= 0 && end > start {
			if err2 := json.Unmarshal([]byte(respText[start:end+1]), &plan); err2 != nil {
				return nil, fmt.Errorf("parse plan: %w", err)
			}
		} else {
			return nil, fmt.Errorf("parse plan: %w", err)
		}
	}

	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("empty plan")
	}

	return plan.Steps, nil
}

func (uc *OrchestratorUseCase) gatherMemoryContext(ctx context.Context, userID, conversationID, task string) string {
	vec, err := uc.embedder.EmbedQuery(ctx, task)
	if err != nil {
		return ""
	}
	hits, err := uc.memoryVector.SearchSummaries(ctx, userID, conversationID, vec, 3)
	if err != nil || len(hits) == 0 {
		return ""
	}
	var parts []string
	for _, h := range hits {
		parts = append(parts, h.Summary)
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func (uc *OrchestratorUseCase) saveToMemory(ctx context.Context, userID, conversationID, orchID, agentName, result string) {
	summary := domain.MemorySummary{
		ID:             uuid.NewString(),
		UserID:         userID,
		ConversationID: conversationID,
		TurnFrom:       0,
		TurnTo:         0,
		Summary:        fmt.Sprintf("[%s] %s", agentName, truncate(result, 1000)),
		CreatedAt:      time.Now().UTC(),
	}
	vec, err := uc.embedder.EmbedQuery(ctx, result)
	if err != nil {
		return
	}
	_ = uc.memoryVector.IndexSummary(ctx, summary, vec)
}

func lastUserMsg(messages []domain.ChatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func containsCriticIssues(result string) bool {
	lower := strings.ToLower(result)
	issueKeywords := []string{"error", "incorrect", "missing", "hallucination", "ошибк", "неточн", "пропущен", "галлюцинац", "не хватает"}
	for _, kw := range issueKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}
```

- [ ] **Step 2: Write basic test**

Create `internal/core/usecase/orchestrator_test.go`:

```go
package usecase

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestContainsCriticIssues(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"The answer is correct and complete.", false},
		{"There is an error in the second paragraph.", true},
		{"Ответ содержит неточности.", true},
		{"Missing information about the deadline.", true},
		{"Everything looks good.", false},
	}
	for _, tt := range tests {
		got := containsCriticIssues(tt.input)
		if got != tt.want {
			t.Errorf("containsCriticIssues(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello world", 5) != "hello..." {
		t.Errorf("expected truncation")
	}
	if truncate("short", 100) != "short" {
		t.Errorf("expected no truncation")
	}
}

func TestLastUserMsg(t *testing.T) {
	msgs := []domain.ChatMessage{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "answer"},
		{Role: "user", Content: "second question"},
	}
	got := lastUserMsg(msgs)
	if got != "second question" {
		t.Errorf("lastUserMsg = %q, want %q", got, "second question")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/usecase/ -run "TestContainsCriticIssues|TestTruncate|TestLastUserMsg" -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/usecase/orchestrator.go internal/core/usecase/orchestrator_test.go
git commit -m "feat(usecase): OrchestratorUseCase with dynamic planning and shared memory"
```

---

### Task 5: shouldOrchestrate + integration into AgentChatUseCase

**Files:**
- Modify: `internal/core/usecase/agent_chat.go`

- [ ] **Step 1: Add orchestrator field and setter**

Add to `AgentChatUseCase` struct:

```go
orchestrator *OrchestratorUseCase
```

Add setter:

```go
func (uc *AgentChatUseCase) SetOrchestrator(o *OrchestratorUseCase) {
	uc.orchestrator = o
}
```

- [ ] **Step 2: Add shouldOrchestrate function**

```go
var orchestrateKeywords = []string{
	"исследуй подробно", "проанализируй детально", "deep research",
	"подробный анализ", "detailed analysis", "thorough investigation",
	"исследуй и напиши", "research and write",
}

func shouldOrchestrate(intent Intent, complexity domain.ComplexityTier, message string) bool {
	if complexity != domain.TierComplex {
		return false
	}
	if intent == IntentGeneral || intent == IntentCode {
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
```

- [ ] **Step 3: Add orchestration call in Complete**

In the `Complete` method, after the complexity routing block (after `slog.Info("adaptive_routing", ...)`), add:

```go
// Multi-agent orchestration for complex tasks.
if uc.orchestrator != nil {
	if shouldOrchestrate(intent, tier, lastUserMessage) {
		slog.Info("orchestrating_multi_agent", "intent", intent)
		return uc.orchestrator.Execute(ctx, req, onToolStatus)
	}
}
```

Note: `tier` variable needs to be available. If it's scoped inside the `if uc.modelRouting != nil` block, extract it to outer scope. Declare `var tier domain.ComplexityTier = domain.TierSimple` before the routing block and assign inside.

- [ ] **Step 4: Run build and tests**

Run: `go build ./... && go test ./internal/core/usecase/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/usecase/agent_chat.go
git commit -m "feat(agent): integrate orchestrator — shouldOrchestrate trigger for complex tasks"
```

---

### Task 6: Config + bootstrap wiring

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/bootstrap/bootstrap.go`

- [ ] **Step 1: Add config fields**

In `internal/config/config.go`, add to Config struct:

```go
AgentSpecs           string // JSON array of AgentSpec
OrchestratorEnabled  bool
OrchestratorMaxSteps int
```

Add to `Load()`:

```go
AgentSpecs:           os.Getenv("AGENT_SPECS"),
OrchestratorEnabled:  mustEnvBool("ORCHESTRATOR_ENABLED", false),
OrchestratorMaxSteps: mustEnvInt("ORCHESTRATOR_MAX_STEPS", 8),
```

Add parser:

```go
func ParseAgentSpecs(raw string) []domain.AgentSpec {
	if raw == "" {
		return nil
	}
	var result []domain.AgentSpec
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}
```

- [ ] **Step 2: Wire in bootstrap**

In `internal/bootstrap/bootstrap.go`, after `agentUC` creation and after the existing setter calls, add:

```go
// Multi-agent orchestration.
if cfg.OrchestratorEnabled {
	agentSpecs := config.ParseAgentSpecs(cfg.AgentSpecs)
	agentRegistry := usecase.NewAgentRegistry(agentSpecs)
	orchRepo := postgres.NewOrchestrationRepository(db)
	orchestrator := usecase.NewOrchestratorUseCase(
		agentUC,
		agentRegistry,
		memoryVector,
		embedder,
		generator,
		orchRepo,
		cfg.OrchestratorMaxSteps,
	)
	agentUC.SetOrchestrator(orchestrator)
	slog.Info("orchestrator_enabled", "agents", agentRegistry.Names(), "max_steps", cfg.OrchestratorMaxSteps)
}
```

- [ ] **Step 3: Run build and tests**

Run: `go build ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go internal/bootstrap/bootstrap.go
git commit -m "feat(bootstrap): wire multi-agent orchestrator with configurable agent specs"
```

---

### Task 7: Final verification + push

- [ ] **Step 1: Full test suite**

Run: `go test ./... -count=1 -v 2>&1 | grep -E "FAIL|ok"`
Expected: All PASS

- [ ] **Step 2: Vet**

Run: `go vet ./...`

- [ ] **Step 3: Push**

```bash
git push origin main
```
