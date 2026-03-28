# Multi-Agent Orchestration

**Дата:** 2026-03-28
**Этап:** 6 — Уровень 3.1: Автоматизация и агенты
**Статус:** approved

## Проблема

Текущий агент — single-loop: один system prompt, один набор tools, один проход. Сложные задачи (исследование + анализ + написание) требуют разных навыков и подходов, которые один промпт не может покрыть оптимально.

## Решение

Dynamic orchestrator координирует специализированных агентов (researcher, coder, writer, critic). Агенты конфигурируемые через JSON. Shared context через Qdrant memory. Streaming промежуточных результатов через SSE. Persistent history в Postgres.

## Configurable Agent Specs

### Конфигурация

```env
AGENT_SPECS=[
  {"name":"researcher","system_prompt":"You are a research specialist. Find facts, sources, and relevant context. Be thorough and cite sources.","tools":["knowledge_search","web_search"],"max_iterations":5},
  {"name":"coder","system_prompt":"You are a code specialist. Generate, analyze, debug, and explain code.","tools":["knowledge_search"],"max_iterations":5},
  {"name":"writer","system_prompt":"You are a writing specialist. Synthesize information into clear, structured responses.","tools":["knowledge_search"],"max_iterations":3},
  {"name":"critic","system_prompt":"You are a quality critic. Check the answer for errors, hallucinations, missing information, and logical gaps. Be strict.","tools":["knowledge_search"],"max_iterations":3}
]
```

Если `AGENT_SPECS` не задан — built-in defaults (researcher, coder, writer, critic).

### Domain type

```go
type AgentSpec struct {
    Name          string   `json:"name"`
    SystemPrompt  string   `json:"system_prompt"`
    Tools         []string `json:"tools"`
    MaxIterations int      `json:"max_iterations"`
}
```

### Agent Registry

```go
type AgentRegistry struct {
    specs map[string]domain.AgentSpec
}

func (r *AgentRegistry) Get(name string) (domain.AgentSpec, bool)
func (r *AgentRegistry) List() []domain.AgentSpec
```

## Dynamic Orchestrator

### Структура

```go
type OrchestratorUseCase struct {
    agentChat    ports.AgentChatService
    registry     *AgentRegistry
    memoryVector ports.MemoryVectorStore
    embedder     ports.Embedder
    generator    ports.AnswerGenerator
    orchStore    ports.OrchestrationStore
}
```

### Flow

1. **Plan step** — LLM call определяет порядок агентов:
```json
{"steps": [
  {"agent": "researcher", "task": "Find information about X"},
  {"agent": "writer", "task": "Synthesize findings"},
  {"agent": "critic", "task": "Check for errors"}
]}
```

2. **Execute loop** — для каждого шага:
   - Получить `AgentSpec` из registry
   - Собрать контекст из Qdrant memory (семантический поиск по задаче шага)
   - Вызвать `agentChat.Complete()` с кастомным system prompt
   - Сохранить результат в Qdrant memory + Postgres history
   - Отправить streaming status через SSE callback

3. **Dynamic routing** — после каждого шага:
   - Critic нашёл проблемы → добавить step writer fix → critic снова
   - Researcher не нашёл достаточно → добавить deeper search step
   - Max `ORCHESTRATOR_MAX_STEPS` (default 8) — guard against loops

4. **Final answer** — последний результат writer'а или последний непустой ответ

## Shared Memory через Qdrant

Переиспользуем существующий `MemoryVectorStore`:

- После каждого шага: embed результат → `IndexSummary` с metadata `{orchestration_id, agent_name, step_index}`
- Перед каждым шагом: `SearchSummaries` по задаче → релевантный контекст из предыдущих шагов
- Scope: `conversation_id` + `orchestration_id`

## Streaming промежуточных результатов

Уже есть `ToolStatusCallback` в `AgentChatService.Complete()`. Расширяем:

```go
type OrchestrationStatus struct {
    OrchestrationID string `json:"orchestration_id"`
    StepIndex       int    `json:"step_index"`
    AgentName       string `json:"agent_name"`
    Task            string `json:"task"`
    Status          string `json:"status"` // "started", "completed", "failed"
    Result          string `json:"result,omitempty"` // truncated
}
```

Orchestrator вызывает `onToolStatus` callback с типом `"orchestration"` для каждого шага. SSE handler в HTTP adapter отправляет это клиенту как tool status event.

## Persistent Orchestration History

### Postgres таблица

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

### OrchestrationStore порт

```go
type OrchestrationStore interface {
    Create(ctx context.Context, orch *domain.Orchestration) error
    UpdateStep(ctx context.Context, orchID string, step domain.OrchestrationStep) error
    Complete(ctx context.Context, orchID string, status string) error
    GetByID(ctx context.Context, orchID string) (*domain.Orchestration, error)
    ListByUser(ctx context.Context, userID string, limit int) ([]domain.Orchestration, error)
}
```

### Domain types

```go
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

type OrchestrationPlanStep struct {
    Agent string `json:"agent"`
    Task  string `json:"task"`
}

type OrchestrationStep struct {
    Index     int       `json:"index"`
    Agent     string    `json:"agent"`
    Task      string    `json:"task"`
    Result    string    `json:"result"`
    Status    string    `json:"status"` // "completed", "failed"
    StartedAt time.Time `json:"started_at"`
    Duration  float64   `json:"duration_ms"`
}
```

## Когда multi-agent vs single-agent

```go
func shouldOrchestrate(intent Intent, complexity ComplexityTier, message string) bool
```

Условия:
- `ComplexityTier == TierComplex` И intent не простой chat
- Или explicit keywords: "исследуй подробно", "проанализируй детально", "deep research", "подробный анализ"

## Файловая структура

```
internal/core/domain/orchestration.go                    # AgentSpec, Orchestration, OrchestrationStep
internal/core/ports/outbound.go                          # OrchestrationStore interface
internal/core/usecase/orchestrator.go                    # OrchestratorUseCase
internal/core/usecase/orchestrator_test.go
internal/core/usecase/agent_registry.go                  # AgentRegistry
internal/core/usecase/agent_registry_test.go
internal/core/usecase/agent_chat.go                      # shouldOrchestrate + вызов orchestrator
internal/infrastructure/repository/postgres/orch_repo.go # OrchestrationStore implementation
internal/config/config.go                                # AGENT_SPECS, ORCHESTRATOR_ENABLED, ORCHESTRATOR_MAX_STEPS
internal/bootstrap/bootstrap.go                          # wire orchestrator
```

## Конфигурация

```env
AGENT_SPECS=<JSON array>
ORCHESTRATOR_ENABLED=true
ORCHESTRATOR_MAX_STEPS=8
```

## Что НЕ входит

- Параллельное выполнение агентов (sequential only)
- UI для визуализации multi-agent flow (отдельный spec)
- API endpoints для управления orchestrations (только через chat)
