# Level 3 — Multi-Agent Orchestration

## Описание

Динамический оркестратор с конфигурируемыми агентами-специалистами (researcher, coder, writer, critic). Разделяемая память через Qdrant. Персистентная история в PostgreSQL. Активируется для сложных задач (ORCHESTRATOR_ENABLED).

## Component Diagram

```mermaid
flowchart TB
    subgraph Orchestration["Multi-Agent System"]
        Orchestrator["OrchestratorUseCase\n─────────\nExecute()\nсоздаёт план → запускает\nспециалистов → агрегирует"]
        Registry["AgentRegistry\n─────────\nDefault specs:\nresearcher, coder,\nwriter, critic"]
        OrchStore["OrchestrationStore\n─────────\nPostgreSQL\nCreate / AddStep /\nComplete / GetByID"]
    end

    subgraph Specialists["Specialist Agents"]
        Researcher["🔍 Researcher\nknowledge_search + web_search"]
        Coder["💻 Coder\nexecute_python/bash"]
        Writer["✍️ Writer\ncontent generation"]
        Critic["🔎 Critic\nquality review"]
    end

    AgentChat["AgentChatUseCase"] -->|"shouldOrchestrate()"| Orchestrator
    Orchestrator --> Registry
    Orchestrator --> Researcher & Coder & Writer & Critic
    Orchestrator --> OrchStore
    Orchestrator -->|"ChatWithTools()"| LLM["Ollama / Cloud"]
```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| OrchestratorUseCase | `internal/core/usecase/orchestrator.go` |
| AgentRegistry | `internal/core/usecase/agent_registry.go` |
| OrchestrationStore | `internal/infrastructure/repository/postgres/orch_repo.go` |
