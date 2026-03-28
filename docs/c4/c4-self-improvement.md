# Level 3 — Self-Improvement

## Описание

Система самоулучшения: сбор событий агента, получение обратной связи от пользователя, периодический LLM-анализ, автоматическое применение безопасных улучшений.

## Component Diagram

```mermaid
flowchart TB
    subgraph SelfImprove["Self-Improvement Pipeline (Worker)"]
        EventCollector["EventCollector\n─────────\nRecord(event)\ntool calls, errors,\nresponse quality"]
        SelfImproveUC["SelfImproveUseCase\n─────────\nperiodic analysis\nLLM reviews events +\nfeedback → improvements"]
    end

    subgraph Stores["Data Stores (PostgreSQL)"]
        EventStore["EventStore\n─────────\nRecord / ListByType\nCountByType"]
        FeedbackStore["FeedbackStore\n─────────\nCreate / ListRecent\nCountByRating"]
        ImpStore["ImprovementStore\n─────────\nCreate / ListPending\nUpdateStatus / MarkApplied"]
    end

    subgraph API["API Server"]
        FeedbackAPI["POST /v1/feedback\n─────────\nuser rating + comment"]
    end

    AgentChat["AgentChatUseCase"] -->|"record events"| EventCollector
    EventCollector --> EventStore
    FeedbackAPI --> FeedbackStore
    SelfImproveUC --> EventStore
    SelfImproveUC --> FeedbackStore
    SelfImproveUC -->|"generate"| ImpStore
    SelfImproveUC -->|"analyze"| LLM["Ollama"]
```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| EventCollector | `internal/core/usecase/event_collector.go` |
| SelfImproveUseCase | `internal/core/usecase/self_improve.go` |
| EventStore | `internal/infrastructure/repository/postgres/events_repo.go` |
| FeedbackStore | `internal/infrastructure/repository/postgres/feedback_repo.go` |
| ImprovementStore | `internal/infrastructure/repository/postgres/improvements_repo.go` |
