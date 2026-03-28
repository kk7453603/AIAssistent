# Level 2 — Containers

## Описание

PAA состоит из двух Go-бинарников (API и Worker), десктопного приложения на Tauri и инфраструктурных сервисов. Гексагональная архитектура (Ports & Adapters).

## Диаграмма

```mermaid
flowchart TB
    subgraph Clients["Клиенты"]
        Tauri["Tauri Desktop\nReact + Vite + Zustand"]
        OWU["OpenWebUI"]
        MCP_CL["MCP-клиенты"]
    end

    subgraph AppLayer["Application Layer"]
        API["API Server\ncmd/api\n─────────\nHTTP handlers\nOpenAI-compat API\nAgent Loop\nRAG Query\nMCP Server\nObsidian handlers\nPrometheus /metrics"]

        WK["Worker\ncmd/worker\n─────────\nNATS subscriber\nDocument processing\nLLM enrichment\nScheduler cron\nSelf-improvement"]
    end

    subgraph DataLayer["Data & Infrastructure"]
        PG["PostgreSQL\n─────────\nDocuments, Conversations\nTasks, Memory\nOrchestrations, Events\nFeedback, Improvements\nSchedules"]

        QD["Qdrant\n─────────\nMulti-collection vectors\n(per source_type)\nMemory summaries"]

        NT["NATS\n─────────\ndocuments.ingest\ndocuments.enrich"]

        OL["Ollama\n─────────\nLLM inference\nEmbeddings\nModel discovery"]

        N4["Neo4j\n─────────\nKnowledge graph\n(optional)"]

        SX["SearXNG\n─────────\nWeb search"]

        FS["Local FS\n─────────\nDocument storage\nObsidian vaults"]
    end

    Tauri -->|"HTTP/SSE"| API
    OWU -->|"OpenAI-compat"| API
    MCP_CL -->|"MCP Streamable HTTP"| API

    API --> PG
    API --> QD
    API --> NT
    API --> OL
    API --> N4
    API --> SX
    API --> FS

    WK --> NT
    WK --> PG
    WK --> QD
    WK --> OL
    WK --> N4
    WK --> FS
```

## Контейнеры

| Контейнер | Технология | Ответственность |
|-----------|-----------|----------------|
| API Server | Go (`cmd/api`) | HTTP handlers, agent loop, RAG query, MCP server, metrics |
| Worker | Go (`cmd/worker`) | NATS subscriber, document processing, enrichment, scheduler, self-improve |
| Tauri Desktop | Rust + React/Vite | Chat UI, Obsidian browser, settings, dashboard, graph |
| PostgreSQL | PostgreSQL 16 | Реляционное хранилище состояния |
| Qdrant | Qdrant | Векторный поиск (multi-collection) |
| NATS | NATS | Асинхронная очередь задач |
| Ollama | Ollama | LLM + embeddings |
| Neo4j | Neo4j 5 | Граф знаний (опционально) |
| SearXNG | SearXNG | Веб-поиск |

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| API Router | `internal/adapters/http/router.go` |
| Worker main | `cmd/worker/main.go` |
| Bootstrap | `internal/bootstrap/bootstrap.go` |
| Docker Compose | `docker-compose.yml` |
| Tauri App | `ui/src/App.tsx` |
