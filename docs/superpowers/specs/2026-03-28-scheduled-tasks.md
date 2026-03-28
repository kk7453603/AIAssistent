# Scheduled Tasks

**Дата:** 2026-03-28
**Этап:** 6 — Уровень 3.2: Автоматизация и агенты
**Статус:** approved

## Проблема

Агент реактивен — работает только когда пользователь отправляет сообщение. Нет возможности автоматически выполнять recurring задачи (ежедневные дайджесты, проверка задач, периодический поиск).

## Решение

Cron-подобный scheduler в worker. Пользователь создаёт scheduled tasks через chat (natural language → cron) или API. Scheduler выполняет задачи через `AgentChatService.Complete()`, с опциональным conditional execution и webhook notification.

## Postgres таблица

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

## Domain type

```go
type ScheduledTask struct {
    ID         string
    UserID     string
    CronExpr   string
    Prompt     string
    Condition  string     // optional: LLM-evaluated condition
    WebhookURL string     // optional: POST result here
    Enabled    bool
    LastRunAt  *time.Time
    LastResult string
    LastStatus string     // "success", "failed", "skipped"
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

## ScheduleStore port

```go
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

## Scheduler — worker cron goroutine

Каждые N секунд (`SCHEDULER_CHECK_INTERVAL_SECONDS`, default 60):

1. Query `ListEnabled()` — all enabled scheduled tasks
2. For each task: check cron expression vs current time using `robfig/cron/v3` parser
3. If cron matches current minute:
   a. If `condition` set → LLM evaluate condition → skip if "no"
   b. Call `AgentChatService.Complete()` with task prompt
   c. `RecordRun(id, result, status)`
   d. If `webhook_url` set → HTTP POST with result JSON

### Cron matching

Use `github.com/robfig/cron/v3` parser. Parse cron expression once, check `Next(lastRunAt).Before(now)` to determine if task should run. Supports: `0 9 * * *`, `@every 1h`, `@daily`, etc.

### Conditional execution

If `condition` is set:
```
LLM prompt: "Evaluate this condition and answer exactly 'yes' or 'no': {condition}"
```
If answer is "no" → `RecordRun(id, "", "skipped")`, don't execute.

### Webhook notification

If `webhook_url` is set:
```
POST {webhook_url}
Content-Type: application/json

{"task_id": "...", "prompt": "...", "result": "...", "status": "success", "executed_at": "..."}
```

Best-effort — log warning on failure, don't retry.

## Chat integration

Extend existing `task_tool` in `AgentChatUseCase` with new actions:

| Action | Description | Params |
|--------|-------------|--------|
| `schedule_create` | Create scheduled task | prompt, cron (or natural language) |
| `schedule_list` | List user's schedules | — |
| `schedule_delete` | Delete schedule | id |
| `schedule_toggle` | Enable/disable | id |

### Natural language → cron parsing

When user says "напомни каждый день в 9:00 проверить задачи":
1. LLM prompt: "Convert this scheduling request to a cron expression and task prompt. Return JSON: {\"cron\": \"...\", \"prompt\": \"...\"}"
2. Parse response → create ScheduledTask

## API endpoints

```
POST   /v1/schedules      — create {cron_expr, prompt, condition?, webhook_url?}
GET    /v1/schedules      — list user's scheduled tasks
DELETE /v1/schedules/:id   — delete
PATCH  /v1/schedules/:id   — update {enabled?, cron_expr?, prompt?, condition?, webhook_url?}
```

## Файловая структура

```
internal/core/domain/schedule.go                           # ScheduledTask type
internal/core/ports/outbound.go                            # ScheduleStore interface
internal/core/usecase/scheduler.go                         # SchedulerUseCase (cron loop + execute)
internal/core/usecase/scheduler_test.go
internal/infrastructure/repository/postgres/schedule_repo.go
internal/adapters/http/router.go                           # CRUD endpoints
internal/core/usecase/agent_chat.go                        # schedule actions in task_tool
internal/config/config.go                                  # config fields
internal/bootstrap/bootstrap.go                            # wire
cmd/worker/main.go                                         # scheduler goroutine
```

## Конфигурация

```env
SCHEDULER_ENABLED=true
SCHEDULER_CHECK_INTERVAL_SECONDS=60
```

## Что НЕ входит

- UI для управления schedules
- Timezone support (UTC only)
- Retry на failed scheduled tasks
- Distributed locking (single worker instance)
