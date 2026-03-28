# Level 3 — Scheduled Tasks

## Описание

Cron-планировщик задач с поддержкой natural language → cron, условного выполнения, webhooks. CRUD API для управления расписаниями.

## Component Diagram

```mermaid
flowchart TB
    subgraph Scheduler["Scheduler System"]
        SchedulerUC["SchedulerUseCase\n─────────\nrobfig/cron/v3\nRegister / Execute\nNL → cron parsing"]
        CondEval["ConditionEvaluator\n─────────\ncheck condition before\nexecution"]
        Webhook["WebhookNotifier\n─────────\nPOST result to URL"]
    end

    subgraph API["API Server"]
        SchedAPI["Schedule API\n─────────\nPOST /v1/schedules\nGET /v1/schedules\nPATCH /v1/schedules/{id}\nDELETE /v1/schedules/{id}"]
    end

    subgraph Store["PostgreSQL"]
        SchedStore["ScheduleStore\n─────────\nCreate / ListByUser\nListEnabled / GetByID\nUpdate / Delete\nRecordRun"]
    end

    SchedAPI --> SchedStore
    SchedulerUC -->|"load enabled"| SchedStore
    SchedulerUC --> CondEval
    SchedulerUC -->|"execute prompt"| LLM["Ollama"]
    SchedulerUC -->|"record run"| SchedStore
    SchedulerUC --> Webhook

    Worker["Worker\ncron goroutine"] --> SchedulerUC
```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| SchedulerUseCase | `internal/core/usecase/scheduler.go` |
| ScheduleStore | `internal/infrastructure/repository/postgres/schedule_repo.go` |
| Schedule API | `internal/adapters/http/router.go` |
