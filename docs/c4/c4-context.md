# Level 1 — System Context

## Описание

PAA — AI-ассистент с каскадным поиском, интеграцией с Obsidian и десктопным UI. Взаимодействует с пользователями через Tauri-приложение, OpenWebUI и MCP-клиенты.

## Диаграмма

```mermaid
flowchart TB
    User["👤 Пользователь"]
    TauriApp["🖥 Tauri Desktop App"]
    OpenWebUI["🌐 OpenWebUI"]
    MCPClients["🔧 MCP-клиенты\n(Claude Code, Cursor)"]

    subgraph PAA["Personal AI Assistant"]
        APIServer["API Server\n(Go)"]
        Worker["Worker\n(Go)"]
    end

    LLM["🤖 LLM Provider\n(Ollama / Cloud)"]
    SearXNG["🔍 SearXNG\n(Web Search)"]
    ObsVaults["📝 Obsidian Vaults\n(Filesystem)"]
    Neo4j["🕸 Neo4j\n(Knowledge Graph)"]
    PG["🗄 PostgreSQL"]
    Qdrant["📊 Qdrant\n(Vector DB)"]
    NATS["📨 NATS\n(Message Queue)"]

    User --> TauriApp
    User --> OpenWebUI
    MCPClients --> APIServer

    TauriApp -->|"HTTP/SSE"| APIServer
    OpenWebUI -->|"OpenAI-compat API"| APIServer

    APIServer --> LLM
    APIServer --> SearXNG
    APIServer --> ObsVaults
    APIServer --> Neo4j
    APIServer --> PG
    APIServer --> Qdrant
    APIServer --> NATS

    Worker --> NATS
    Worker --> LLM
    Worker --> PG
    Worker --> Qdrant
    Worker --> Neo4j
```

## Внешние актёры

| Актёр | Описание |
|-------|----------|
| Пользователь | Работает через Tauri UI или OpenWebUI |
| MCP-клиенты | Claude Code, Cursor — подключаются через MCP-протокол |

## Внешние системы

| Система | Назначение |
|---------|-----------|
| LLM Provider | Ollama (self-hosted), OpenRouter, Groq, Together, Cerebras, HuggingFace |
| SearXNG | Self-hosted метапоисковик для web search |
| Obsidian Vaults | Локальные файлы заметок пользователя |
| Neo4j | Граф знаний (опционально) |

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| API Server | `cmd/api/main.go` |
| Worker | `cmd/worker/main.go` |
| Bootstrap | `internal/bootstrap/bootstrap.go` |
| Docker Compose | `docker-compose.yml` |
