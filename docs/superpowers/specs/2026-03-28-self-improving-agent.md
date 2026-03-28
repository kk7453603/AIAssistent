# Self-Improving Agent

**Дата:** 2026-03-28
**Этап:** 6 — Уровень 3.3: Автоматизация и агенты
**Статус:** approved

## Проблема

Агент не учится на ошибках. Технические сбои, пустые retrieval результаты, negative user feedback — всё теряется в логах. Нет механизма для систематического анализа и улучшения.

## Решение

Сбор структурированных событий (ошибки, fallback'и, feedback), периодический LLM-анализ паттернов, генерация конкретных improvement suggestions, автоматическое применение безопасных улучшений (system prompts, keywords, reindexing).

## Data Collection

### Автоматические события (из существующего кода)

| Event Type | Источник | Что собираем |
|-----------|----------|-------------|
| `tool_error` | Agent tool execution | tool name, error message |
| `empty_retrieval` | QueryUseCase | query text, filter used |
| `fallback` | Agent cascading search | from → to (RAG → web → LLM) |
| `critic_rejection` | Orchestrator | critic feedback text |
| `timeout` | LLM calls | model, duration |
| `parse_error` | JSON parsing | raw response snippet |

### User feedback (новое)

```
POST /v1/feedback
{
  "conversation_id": "...",
  "message_id": "...",
  "rating": "up" | "down",
  "comment": "optional text"
}
```

## Storage

### Postgres таблицы

```sql
CREATE TABLE IF NOT EXISTS agent_events (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    conversation_id TEXT,
    event_type TEXT NOT NULL,
    details JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_events_type_created
    ON agent_events(event_type, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_feedback (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    message_id TEXT,
    rating TEXT NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_feedback_user_created
    ON agent_feedback(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_improvements (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL,
    description TEXT NOT NULL,
    action JSONB NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    applied_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_agent_improvements_status
    ON agent_improvements(status, created_at DESC);
```

## Ports

```go
type EventStore interface {
    Record(ctx context.Context, event *domain.AgentEvent) error
    ListByType(ctx context.Context, eventType string, since time.Time, limit int) ([]domain.AgentEvent, error)
    CountByType(ctx context.Context, since time.Time) (map[string]int, error)
}

type FeedbackStore interface {
    Create(ctx context.Context, fb *domain.AgentFeedback) error
    ListRecent(ctx context.Context, since time.Time, limit int) ([]domain.AgentFeedback, error)
    CountByRating(ctx context.Context, since time.Time) (map[string]int, error)
}

type ImprovementStore interface {
    Create(ctx context.Context, imp *domain.AgentImprovement) error
    ListPending(ctx context.Context) ([]domain.AgentImprovement, error)
    UpdateStatus(ctx context.Context, id string, status string) error
    MarkApplied(ctx context.Context, id string) error
}
```

## Analysis — LLM Job

Background job в worker (cron, каждые N часов):

1. Query `agent_events` + `agent_feedback` за последний период
2. Агрегировать: top error types, most-downvoted conversations, frequent empty retrievals
3. LLM prompt с агрегированными данными → structured suggestions
4. Parse suggestions → `agent_improvements` records

### LLM Analysis Prompt

```
Analyze these agent performance metrics and suggest specific improvements.

Error summary (last 24h):
- tool_error: {count} occurrences, top errors: {details}
- empty_retrieval: {count} queries with no results
- fallback: {count} cascades to web/LLM
- timeout: {count} LLM timeouts

User feedback (last 24h):
- Positive: {up_count}
- Negative: {down_count}
- Sample negative comments: {comments}

Return JSON array of improvements:
[{"category": "system_prompt|intent_keywords|model_routing|reindex_document|eval_case|add_document",
  "description": "human-readable description",
  "action": {category-specific payload}}]
```

## Auto-Apply Whitelist

| Category | Action | Auto-apply? | Mechanism |
|----------|--------|-------------|-----------|
| `system_prompt` | Update AgentSpec prompt | Yes | Reload agent registry |
| `intent_keywords` | Add keyword to classifier | Yes | Append to keyword list |
| `model_routing` | Change tier assignment | Yes | Update ModelRouting |
| `reindex_document` | Re-process document | Yes | Publish to NATS ingest |
| `eval_case` | Create eval case from error | Yes | Append to cases JSONL |
| `add_document` | Suggest user add document | No | Notification only |

### ImprovementApplier

```go
type ImprovementApplier struct {
    agentChat   ports.AgentChatService
    queue       ports.MessageQueue
    // ... references to mutable config
}

func (a *ImprovementApplier) Apply(ctx context.Context, imp domain.AgentImprovement) error
```

Checks whitelist → applies action → updates status to "applied" → logs for audit.

## EventCollector

Thin wrapper injected into `AgentChatUseCase`. Captures events during agent execution:

```go
type EventCollector struct {
    store ports.EventStore
}

func (c *EventCollector) RecordToolError(ctx, userID, convID, toolName string, err error)
func (c *EventCollector) RecordEmptyRetrieval(ctx, userID, convID, query string)
func (c *EventCollector) RecordFallback(ctx, userID, convID, from, to string)
func (c *EventCollector) RecordTimeout(ctx, userID, convID, model string, duration time.Duration)
```

## Файловая структура

```
internal/core/domain/improvement.go                           # AgentEvent, AgentFeedback, AgentImprovement
internal/core/ports/outbound.go                               # EventStore, FeedbackStore, ImprovementStore
internal/core/usecase/self_improve.go                         # SelfImproveUseCase (analysis + auto-apply)
internal/core/usecase/self_improve_test.go
internal/core/usecase/event_collector.go                      # EventCollector
internal/infrastructure/repository/postgres/events_repo.go
internal/infrastructure/repository/postgres/feedback_repo.go
internal/infrastructure/repository/postgres/improvements_repo.go
internal/adapters/http/router.go                              # POST /v1/feedback
internal/config/config.go                                     # config fields
internal/bootstrap/bootstrap.go                               # wire
cmd/worker/main.go                                            # cron goroutine for analysis
```

## Конфигурация

```env
SELF_IMPROVE_ENABLED=true
SELF_IMPROVE_INTERVAL_HOURS=24
SELF_IMPROVE_AUTO_APPLY=true
```

## Что НЕ входит

- UI dashboard для просмотра improvements
- Code generation / Go patches
- A/B testing applied improvements
- Real-time improvement (только batch analysis по cron)
