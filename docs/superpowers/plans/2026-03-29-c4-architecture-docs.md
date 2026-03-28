# C4 Architecture Documentation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create comprehensive C4 architecture documentation (Context → Container → Component) for all project features in `docs/c4/`, referenced from `README.md`.

**Architecture:** 12 Markdown files with Mermaid diagrams in `docs/c4/`. Index file for navigation. README.md updated with link. Existing `docs/architecture.md` preserved.

**Tech Stack:** Mermaid (C4 + flowchart + sequence), Markdown

**Spec:** `docs/superpowers/specs/2026-03-29-c4-architecture-docs-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `docs/c4/README.md` | Create | Index — navigation and glossary |
| `docs/c4/c4-context.md` | Create | L1 — System Context diagram |
| `docs/c4/c4-containers.md` | Create | L2 — Container diagram |
| `docs/c4/c4-rag-pipeline.md` | Create | L3 — RAG ingest/query components |
| `docs/c4/c4-agent-loop.md` | Create | L3 — Agent planner/tools/memory |
| `docs/c4/c4-obsidian.md` | Create | L3 — Obsidian vault integration |
| `docs/c4/c4-knowledge-graph.md` | Create | L3 — Neo4j graph components |
| `docs/c4/c4-multi-agent.md` | Create | L3 — Multi-agent orchestration |
| `docs/c4/c4-self-improvement.md` | Create | L3 — Self-improvement pipeline |
| `docs/c4/c4-scheduled-tasks.md` | Create | L3 — Cron scheduler |
| `docs/c4/c4-mcp.md` | Create | L3 — MCP server/client |
| `docs/c4/c4-tauri-ui.md` | Create | L3 — Tauri desktop app |
| `docs/c4/c4-monitoring.md` | Create | L3 — Observability stack |
| `README.md` | Modify | Add architecture docs section |

---

### Task 1: Create Index and L1/L2 Diagrams

**Files:**
- Create: `docs/c4/README.md`
- Create: `docs/c4/c4-context.md`
- Create: `docs/c4/c4-containers.md`

- [ ] **Step 1: Create `docs/c4/README.md`**

```markdown
# C4 Architecture — Personal AI Assistant

Архитектурная документация проекта в нотации [C4 Model](https://c4model.com/) (Simon Brown).

Все диаграммы в формате Mermaid — рендерятся на GitHub.

## Уровни

### Level 1 — System Context
- [System Context](c4-context.md) — PAA и внешний мир

### Level 2 — Containers
- [Containers](c4-containers.md) — API, Worker, UI, инфраструктура

### Level 3 — Components (по фичам)
- [RAG Pipeline](c4-rag-pipeline.md) — загрузка, обработка, поиск документов
- [Agent Loop](c4-agent-loop.md) — каскадный поиск, инструменты, память
- [Obsidian](c4-obsidian.md) — синхронизация vault, создание заметок
- [Knowledge Graph](c4-knowledge-graph.md) — Neo4j, связи, расширение запросов
- [Multi-Agent](c4-multi-agent.md) — оркестратор специалистов
- [Self-Improvement](c4-self-improvement.md) — события, обратная связь, улучшения
- [Scheduled Tasks](c4-scheduled-tasks.md) — cron-планировщик
- [MCP](c4-mcp.md) — MCP-сервер и внешние MCP-клиенты
- [Tauri UI](c4-tauri-ui.md) — десктопное приложение
- [Monitoring](c4-monitoring.md) — Prometheus, Grafana, алерты

## Глоссарий

| Термин | Значение |
|--------|----------|
| PAA | Personal AI Assistant — основная система |
| RAG | Retrieval-Augmented Generation — генерация с поиском |
| MCP | Model Context Protocol — протокол контекста моделей |
| SSE | Server-Sent Events — потоковая передача |
| Chunk | Фрагмент документа для векторного поиска |
| Embedding | Векторное представление текста |
| Reranking | Переранжирование результатов поиска |
```

- [ ] **Step 2: Create `docs/c4/c4-context.md`**

```markdown
# Level 1 — System Context

## Описание

PAA — AI-ассистент с каскадным поиском, интеграцией с Obsidian и десктопным UI. Взаимодействует с пользователями через Tauri-приложение, OpenWebUI и MCP-клиенты.

## Диаграмма

​```mermaid
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
​```

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
```

- [ ] **Step 3: Create `docs/c4/c4-containers.md`**

```markdown
# Level 2 — Containers

## Описание

PAA состоит из двух Go-бинарников (API и Worker), десктопного приложения на Tauri и инфраструктурных сервисов. Гексагональная архитектура (Ports & Adapters).

## Диаграмма

​```mermaid
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
​```

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
```

- [ ] **Step 4: Verify Mermaid renders**

Open `docs/c4/c4-context.md` and `docs/c4/c4-containers.md` in a Markdown previewer or push to GitHub to verify Mermaid diagrams render.

- [ ] **Step 5: Commit**

```bash
git add docs/c4/README.md docs/c4/c4-context.md docs/c4/c4-containers.md
git commit -m "docs(c4): add index, L1 system context, L2 containers"
```

---

### Task 2: L3 — RAG Pipeline

**Files:**
- Create: `docs/c4/c4-rag-pipeline.md`

- [ ] **Step 1: Create `docs/c4/c4-rag-pipeline.md`**

```markdown
# Level 3 — RAG Pipeline

## Описание

Retrieval-Augmented Generation — ядро системы. Загрузка документов из нескольких источников, асинхронная обработка (extract → classify → chunk → embed → index), гибридный поиск с reranking, генерация ответа.

## Component Diagram

​```mermaid
flowchart TB
    subgraph API["API Server"]
        IngestUC["IngestUseCase\n─────────\nUpload()\nIngestFromSource()"]
        QueryUC["QueryUseCase\n─────────\nAnswer()\nSearch + Rerank + Generate"]
        QueryFusion["QueryFusionUseCase\n─────────\nMulti-query expansion"]
        RerankUC["RerankUseCase\n─────────\nCross-encoder scoring"]
    end

    subgraph Worker["Worker"]
        ProcessUC["ProcessUseCase\n─────────\nProcessByID()\nextract → classify →\nchunk → embed → index"]
        EnrichUC["EnrichUseCase\n─────────\nAsync LLM enrichment"]
    end

    subgraph Sources["Source Adapters"]
        UploadSrc["UploadAdapter\nsource_type: upload"]
        WebSrc["WebAdapter\nsource_type: web\nHTML → text"]
        ObsSrc["ObsidianAdapter\nsource_type: obsidian\n(stub)"]
    end

    subgraph Extractors["Extractor Registry"]
        TxtExt["PlaintextExtractor\ntext/plain, text/markdown"]
        PdfExt["PDFExtractor\napplication/pdf"]
        DocxExt["DOCXExtractor\napplication/vnd.openxmlformats"]
        SpreadExt["SpreadsheetExtractor\nxlsx, csv"]
        MetaExt["MetadataExtractor\nfrontmatter, headers"]
    end

    subgraph Chunking["Chunker Registry"]
        FixedChunk["FixedSizeChunker\ndefault"]
        MarkdownChunk["MarkdownChunker\nfor obsidian"]
    end

    IngestUC --> UploadSrc & WebSrc & ObsSrc
    IngestUC -->|"save file"| FS["Local FS"]
    IngestUC -->|"create doc"| PG["PostgreSQL"]
    IngestUC -->|"publish"| NATS["NATS"]

    ProcessUC -->|"open file"| FS
    ProcessUC --> TxtExt & PdfExt & DocxExt & SpreadExt
    ProcessUC --> MetaExt
    ProcessUC --> FixedChunk & MarkdownChunk
    ProcessUC -->|"classify + embed"| Ollama["Ollama"]
    ProcessUC -->|"index vectors"| Qdrant["Qdrant"]
    ProcessUC -->|"save status"| PG

    QueryUC -->|"embed query"| Ollama
    QueryUC -->|"search"| Qdrant
    QueryUC --> RerankUC
    QueryUC --> QueryFusion
    QueryUC -->|"generate answer"| Ollama
​```

## Key Flows

### Загрузка документа

​```mermaid
sequenceDiagram
    participant Client
    participant API as IngestUC
    participant FS as LocalFS
    participant PG as PostgreSQL
    participant NATS
    participant Worker as ProcessUC
    participant Ollama
    participant Qdrant

    Client->>API: POST /v1/documents (file)
    API->>FS: Save(storage_key, bytes)
    API->>PG: Create(status=uploaded)
    API->>NATS: Publish(document_id)
    API-->>Client: 202 + document_id

    NATS->>Worker: document_id
    Worker->>PG: UpdateStatus(processing)
    Worker->>FS: Open(file)
    Worker->>Worker: Extract text (by MIME type)
    Worker->>Ollama: Classify(text)
    Worker->>Worker: Chunk (fixed/markdown)
    Worker->>Ollama: Embed(chunks)
    Worker->>Qdrant: IndexChunks(vectors)
    Worker->>PG: UpdateStatus(ready)
​```

### Поиск и ответ

​```mermaid
sequenceDiagram
    participant Client
    participant API as QueryUC
    participant Ollama
    participant Qdrant
    participant Reranker as RerankUC

    Client->>API: POST /v1/rag/query
    API->>Ollama: EmbedQuery(question)
    API->>Qdrant: Search(vector, filter)
    API->>Qdrant: SearchLexical(text, filter)
    API->>Reranker: Rerank(query, chunks, topN)
    API->>Ollama: GenerateAnswer(question, chunks)
    API-->>Client: { text, sources }
​```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| IngestUseCase | `internal/core/usecase/ingest.go` |
| ProcessUseCase | `internal/core/usecase/process.go` |
| QueryUseCase | `internal/core/usecase/query.go` |
| QueryFusionUseCase | `internal/core/usecase/query_fusion.go` |
| RerankUseCase | `internal/core/usecase/rerank.go` |
| EnrichUseCase | `internal/core/usecase/enrich.go` |
| ExtractorRegistry | `internal/infrastructure/extractor/registry.go` |
| ChunkerRegistry | `internal/infrastructure/chunking/registry.go` |
| Source adapters | `internal/infrastructure/source/upload/`, `web/`, `obsidian/` |
| Qdrant client | `internal/infrastructure/vector/qdrant/client.go` |
| Multi-collection | `internal/infrastructure/vector/qdrant/multi_collection.go` |
```

- [ ] **Step 2: Commit**

```bash
git add docs/c4/c4-rag-pipeline.md
git commit -m "docs(c4): add L3 RAG Pipeline component diagram"
```

---

### Task 3: L3 — Agent Loop

**Files:**
- Create: `docs/c4/c4-agent-loop.md`

- [ ] **Step 1: Create `docs/c4/c4-agent-loop.md`**

```markdown
# Level 3 — Agent Loop

## Описание

Серверный agent loop с native function calling. Каскадный поиск: база знаний → память LLM → веб-поиск. Адаптивный выбор модели по сложности запроса. Краткосрочная и долгосрочная память.

## Component Diagram

​```mermaid
flowchart TB
    subgraph AgentSystem["Agent Loop (API Server)"]
        AgentChat["AgentChatUseCase\n─────────\nComplete()\nmain loop: planner → tools → persist"]
        IntentCls["IntentClassifier\n─────────\nkeyword + LLM\ngeneral/search/code/creative"]
        ComplexCls["ComplexityClassifier\n─────────\nrule-based + LLM\nsimple/complex/code"]
        ModelRouter["ModelRouter\n─────────\nadaptive model selection\nby complexity tier"]
        ToolCache["ToolResultCache\n─────────\ndedup repeated calls\nTTL per tool"]
        Guardrails["Guardrails\n─────────\ncode safety checks\nfor execute_python/bash"]
    end

    subgraph Tools["Tool Dispatch"]
        KS["knowledge_search\n→ QueryUseCase"]
        WS["web_search\n→ SearXNG"]
        OW["obsidian_write\n→ ObsidianNoteWriter"]
        TT["task_tool\n→ TaskStore CRUD"]
        MCPTool["MCP tools\n→ MCPToolRegistry"]
    end

    subgraph Memory["Memory System"]
        ShortMem["ConversationStore\n─────────\nPostgreSQL\nrecent messages"]
        LongMem["MemoryVectorStore\n─────────\nQdrant\nsemantic summaries"]
        MemStore["MemoryStore\n─────────\nPostgreSQL\nsummary metadata"]
    end

    AgentChat --> IntentCls
    AgentChat --> ComplexCls
    ComplexCls --> ModelRouter
    AgentChat --> ToolCache
    AgentChat --> Guardrails

    AgentChat -->|"ChatWithTools()"| LLM["Ollama / Cloud LLM"]
    AgentChat --> KS & WS & OW & TT & MCPTool

    AgentChat -->|"load history"| ShortMem
    AgentChat -->|"semantic search"| LongMem
    AgentChat -->|"persist summary"| MemStore
    AgentChat -->|"index summary"| LongMem
​```

## Каскадный поиск

​```mermaid
flowchart LR
    Q["Запрос пользователя"]
    KB["1. knowledge_search\n(Qdrant RAG)"]
    LLM["2. LLM memory\n(ответ из знаний модели)"]
    WEB["3. web_search\n(SearXNG)"]
    OBS["4. obsidian_write\n(сохранить найденное)"]

    Q --> KB
    KB -->|"не найдено"| LLM
    LLM -->|"не знает"| WEB
    WEB -->|"найдено"| OBS
​```

## Key Flow: Agent Complete()

​```mermaid
sequenceDiagram
    participant Client
    participant Agent as AgentChatUseCase
    participant Conv as ConversationStore
    participant MemVec as MemoryVectorStore
    participant LLM as Ollama
    participant Tool as ToolDispatch

    Client->>Agent: Complete(user_id, messages)
    Agent->>Agent: classifyIntent(message)
    Agent->>Agent: classifyComplexity → select model
    Agent->>Conv: ListRecentMessages()
    Agent->>MemVec: SearchSummaries(query_vector)
    Agent->>Agent: buildSystemPrompt(intent, memory)

    loop max_iterations
        Agent->>LLM: ChatWithTools(messages, toolSchemas)
        alt tool_calls returned
            Agent->>Tool: executeToolCall(name, args)
            Tool-->>Agent: tool result
            Agent->>Agent: append tool message
        else content returned
            Agent->>Agent: finalAnswer = content
        end
    end

    Agent->>Conv: AppendMessage(user + assistant)
    Agent->>Agent: maybePersistSummary()
    Agent-->>Client: AgentRunResult
​```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| AgentChatUseCase | `internal/core/usecase/agent_chat.go` |
| IntentClassifier | `internal/core/usecase/intent.go` |
| ComplexityClassifier | `internal/core/usecase/complexity.go` |
| ToolResultCache | `internal/core/usecase/tool_cache.go` |
| Guardrails | `internal/core/usecase/guardrails.go` |
| ToolHelpers | `internal/core/usecase/tool_helpers.go` |
| ModelRouter | `internal/infrastructure/llm/routing/routing.go` |
| ConversationRepo | `internal/infrastructure/repository/postgres/conversation_repository.go` |
| MemoryRepo | `internal/infrastructure/repository/postgres/memory_repository.go` |
| MemoryVectorStore | `internal/infrastructure/vector/qdrant/memory_client.go` |
```

- [ ] **Step 2: Commit**

```bash
git add docs/c4/c4-agent-loop.md
git commit -m "docs(c4): add L3 Agent Loop component diagram"
```

---

### Task 4: L3 — Obsidian, Knowledge Graph, Multi-Agent

**Files:**
- Create: `docs/c4/c4-obsidian.md`
- Create: `docs/c4/c4-knowledge-graph.md`
- Create: `docs/c4/c4-multi-agent.md`

- [ ] **Step 1: Create `docs/c4/c4-obsidian.md`**

```markdown
# Level 3 — Obsidian Integration

## Описание

Управление Obsidian vault: регистрация, синхронизация (ручная/автоматическая), создание заметок через агента, браузер файлов. Vault-конфигурация хранится в JSON-файлах.

## Component Diagram

​```mermaid
flowchart TB
    subgraph ObsidianSystem["Obsidian Integration (API Server)"]
        Handlers["ObsidianHandlers\n─────────\nList / Upsert / Remove\nSync / CreateNote\nListFiles / FileContent"]
        Config["VaultConfig\n─────────\nJSON registry\nobsidian_config.json"]
        State["SyncState\n─────────\nPer-vault state\nfile hashes"]
        NoteWriter["ObsidianNoteWriter\n─────────\nCreateNote()\nagent tool interface"]
    end

    subgraph External["External"]
        VaultFS["Obsidian Vault\n(Filesystem)"]
        IngestUC["IngestUseCase\n─────────\nIngestFromSource()"]
    end

    Handlers --> Config
    Handlers --> State
    Handlers --> VaultFS
    Handlers -->|"sync: ingest new files"| IngestUC
    NoteWriter --> VaultFS
    NoteWriter -->|"agent obsidian_write"| Handlers

    UI["Tauri UI\nVaultBrowserPage"] -->|"HTTP"| Handlers
​```

## Key Flow: Vault Sync

​```mermaid
sequenceDiagram
    participant User
    participant API as ObsidianHandlers
    participant FS as Vault Filesystem
    participant State as SyncState
    participant Ingest as IngestUseCase

    User->>API: POST /v1/obsidian/sync/{id}
    API->>FS: listMarkdownFiles()
    API->>State: loadState(vault_id)
    loop each .md file
        API->>FS: hashFile(path)
        alt hash changed or new
            API->>Ingest: IngestFromSource(obsidian, file)
            API->>State: update hash
        end
    end
    API->>State: saveState(vault_id)
    API-->>User: 200 { synced: N }
​```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| ObsidianHandlers | `internal/adapters/http/obsidian_handlers.go` |
| ObsidianAdapter | `internal/infrastructure/source/obsidian/adapter.go` |
| IngestUseCase | `internal/core/usecase/ingest.go` |
```

- [ ] **Step 2: Create `docs/c4/c4-knowledge-graph.md`**

```markdown
# Level 3 — Knowledge Graph

## Описание

Граф знаний на Neo4j. Извлечение wikilinks из документов, вычисление семантической близости, расширение поисковых запросов через связи графа. Опциональный компонент (`GRAPH_ENABLED`).

## Component Diagram

​```mermaid
flowchart TB
    subgraph GraphSystem["Knowledge Graph"]
        Neo4jStore["Neo4jGraphStore\n─────────\nUpsertDocument()\nAddLink() / AddSimilarity()\nGetRelated() / FindByTitle()\nGetGraph()"]
        LinkExtractor["LinkExtractor\n─────────\nParseWikilinks()\nfrom document text"]
        QueryExpander["expandQueryWithGraph()\n─────────\nFindByTitle → GetRelated\nadd related titles to query"]
        GraphAPI["Graph API\n─────────\nGET /v1/graph\nfilter by source_type,\ncategory, min_score"]
    end

    ProcessUC["ProcessUseCase\n(Worker)"] -->|"extract links"| LinkExtractor
    ProcessUC -->|"upsert node + edges"| Neo4jStore
    AgentChat["AgentChatUseCase\n(API)"] -->|"expand query"| QueryExpander
    QueryExpander --> Neo4jStore
    GraphAPI --> Neo4jStore

    Neo4jStore --> Neo4j["Neo4j DB"]
    UI["Tauri UI\nGraphPage"] -->|"HTTP"| GraphAPI
​```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| Neo4jGraphStore | `internal/infrastructure/graph/neo4j/client.go` |
| LinkExtractor | `internal/core/usecase/links.go` |
| Query expansion | `internal/core/usecase/agent_chat.go:expandQueryWithGraph` |
| NoopGraphStore | `internal/infrastructure/graph/noop.go` |
| Graph API handler | `internal/adapters/http/router.go` |
```

- [ ] **Step 3: Create `docs/c4/c4-multi-agent.md`**

```markdown
# Level 3 — Multi-Agent Orchestration

## Описание

Динамический оркестратор с конфигурируемыми агентами-специалистами (researcher, coder, writer, critic). Разделяемая память через Qdrant. Персистентная история в PostgreSQL. Активируется для сложных задач (`ORCHESTRATOR_ENABLED`).

## Component Diagram

​```mermaid
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
​```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| OrchestratorUseCase | `internal/core/usecase/orchestrator.go` |
| AgentRegistry | `internal/core/usecase/agent_registry.go` |
| OrchestrationStore | `internal/infrastructure/repository/postgres/orch_repo.go` |
```

- [ ] **Step 4: Commit**

```bash
git add docs/c4/c4-obsidian.md docs/c4/c4-knowledge-graph.md docs/c4/c4-multi-agent.md
git commit -m "docs(c4): add L3 Obsidian, Knowledge Graph, Multi-Agent"
```

---

### Task 5: L3 — Self-Improvement, Scheduled Tasks, MCP

**Files:**
- Create: `docs/c4/c4-self-improvement.md`
- Create: `docs/c4/c4-scheduled-tasks.md`
- Create: `docs/c4/c4-mcp.md`

- [ ] **Step 1: Create `docs/c4/c4-self-improvement.md`**

```markdown
# Level 3 — Self-Improvement

## Описание

Система самоулучшения: сбор событий агента, получение обратной связи от пользователя, периодический LLM-анализ, автоматическое применение безопасных улучшений.

## Component Diagram

​```mermaid
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
​```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| EventCollector | `internal/core/usecase/event_collector.go` |
| SelfImproveUseCase | `internal/core/usecase/self_improve.go` |
| EventStore | `internal/infrastructure/repository/postgres/events_repo.go` |
| FeedbackStore | `internal/infrastructure/repository/postgres/feedback_repo.go` |
| ImprovementStore | `internal/infrastructure/repository/postgres/improvements_repo.go` |
```

- [ ] **Step 2: Create `docs/c4/c4-scheduled-tasks.md`**

```markdown
# Level 3 — Scheduled Tasks

## Описание

Cron-планировщик задач с поддержкой natural language → cron, условного выполнения, webhooks. CRUD API для управления расписаниями.

## Component Diagram

​```mermaid
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
​```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| SchedulerUseCase | `internal/core/usecase/scheduler.go` |
| ScheduleStore | `internal/infrastructure/repository/postgres/schedule_repo.go` |
| Schedule API | `internal/adapters/http/router.go` |
```

- [ ] **Step 3: Create `docs/c4/c4-mcp.md`**

```markdown
# Level 3 — MCP Integration

## Описание

Двустороння MCP-интеграция: PAA выступает как MCP-сервер (для Claude Code, Cursor, OpenWebUI) и как MCP-клиент (подключается к внешним серверам: filesystem, code-runner, GitHub). Дополнительно: JSON-defined HTTP API tools.

## Component Diagram

​```mermaid
flowchart TB
    subgraph MCPSystem["MCP Integration"]
        MCPServer["MCPServer\n─────────\nStreamable HTTP on /mcp\ntools: knowledge_search,\nweb_search, obsidian_write,\ntask_create/list/get/\nupdate/delete/complete"]
        MCPRegistry["MCPToolRegistry\n─────────\nListTools() — built-in + MCP\nIsBuiltIn()\nCallMCPTool()"]
        MCPClient["MCPClient\n─────────\nConnect to external\nMCP servers\n(stdio/SSE/HTTP)"]
        HTTPTools["HTTPToolsPlugin\n─────────\nJSON-defined HTTP tools\nenv var expansion\nJSONPath output"]
    end

    subgraph ExternalMCP["External MCP Servers"]
        FSServer["filesystem\nread/write files"]
        CodeRunner["code-runner\nexecute python/bash"]
        GitHub["github\nrepo operations"]
    end

    subgraph Clients["MCP Clients"]
        ClaudeCode["Claude Code"]
        Cursor["Cursor"]
        OWUI["OpenWebUI"]
    end

    Clients -->|"MCP protocol"| MCPServer
    MCPServer --> MCPRegistry
    MCPRegistry --> MCPClient
    MCPClient --> FSServer & CodeRunner & GitHub
    MCPRegistry --> HTTPTools

    AgentChat["AgentChatUseCase"] -->|"CallMCPTool()"| MCPRegistry
​```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| MCPServer | `internal/infrastructure/mcp/server.go` |
| MCPClient | `internal/infrastructure/mcp/client.go` |
| MCPToolRegistry | `internal/infrastructure/mcp/registry.go` |
| HTTPToolsPlugin | `internal/infrastructure/mcp/http_tools.go` |
| MCP ports | `internal/core/ports/mcp.go` |
```

- [ ] **Step 4: Commit**

```bash
git add docs/c4/c4-self-improvement.md docs/c4/c4-scheduled-tasks.md docs/c4/c4-mcp.md
git commit -m "docs(c4): add L3 Self-Improvement, Scheduled Tasks, MCP"
```

---

### Task 6: L3 — Tauri UI, Monitoring + Update README

**Files:**
- Create: `docs/c4/c4-tauri-ui.md`
- Create: `docs/c4/c4-monitoring.md`
- Modify: `README.md`

- [ ] **Step 1: Create `docs/c4/c4-tauri-ui.md`**

```markdown
# Level 3 — Tauri Desktop UI

## Описание

Десктопное приложение на Tauri (Rust + React). Chat с SSE streaming, Obsidian browser, settings, dashboard, 3D граф знаний. Zustand для state management, typed API client.

## Component Diagram

​```mermaid
flowchart TB
    subgraph TauriApp["Tauri Desktop App"]
        subgraph Pages["Pages"]
            Chat["ChatPage\n─────────\nSSE streaming\nthink blocks\ntool status"]
            Vault["VaultBrowserPage\n─────────\nvault management\nfile browser"]
            Settings["SettingsPage\n─────────\nmodel, MCP, agent\ntheme, language"]
            Dashboard["DashboardPage\n─────────\ntool usage stats"]
            Graph["GraphPage\n─────────\n3D knowledge graph\nreact-force-graph-3d"]
            QuickAsk["QuickAskPage\n─────────\nquick question"]
        end

        subgraph Stores["Zustand Stores"]
            ChatStore["chatStore\n─────────\nmessages, streaming,\nmodel selection"]
            ConvStore["conversationStore\n─────────\nhistory, search"]
            VaultSt["vaultStore\n─────────\nvault CRUD, files"]
            SettingsSt["settingsStore\n─────────\ntheme, models, MCP"]
            DashSt["dashboardStore\n─────────\ntool usage data"]
            GraphSt["graphStore\n─────────\nnodes, edges, filters"]
        end

        subgraph APIClient["API Client"]
            Client["client.ts\n─────────\napiFetch()\nSSE streaming"]
            GraphAPI["graph.ts\n─────────\nfetchGraph()"]
            HealthAPI["health.ts\n─────────\nhealthCheck()"]
            Types["types.ts\n─────────\nall API types"]
        end
    end

    Chat --> ChatStore --> Client
    Vault --> VaultSt --> Client
    Settings --> SettingsSt --> Client
    Dashboard --> DashSt --> Client
    Graph --> GraphSt --> GraphAPI

    Client -->|"HTTP/SSE"| API["PAA API Server"]
​```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| App + routing | `ui/src/App.tsx` |
| ChatPage | `ui/src/pages/ChatPage.tsx` |
| VaultBrowserPage | `ui/src/pages/VaultBrowserPage.tsx` |
| SettingsPage | `ui/src/pages/SettingsPage.tsx` |
| DashboardPage | `ui/src/pages/DashboardPage.tsx` |
| GraphPage | `ui/src/pages/GraphPage.tsx` |
| API client | `ui/src/api/client.ts` |
| Types | `ui/src/api/types.ts` |
| Stores | `ui/src/stores/` |
```

- [ ] **Step 2: Create `docs/c4/c4-monitoring.md`**

```markdown
# Level 3 — Monitoring

## Описание

Observability-стек: Prometheus metrics (API + Worker), Grafana dashboards, Alertmanager. Метрики: HTTP requests, agent tool calls, document processing, LLM latency.

## Component Diagram

​```mermaid
flowchart TB
    subgraph AppMetrics["Application Metrics"]
        HTTPMetrics["HTTPMetrics\n─────────\nrequest_duration\nstatus_codes\nin_flight"]
        AgentMetrics["AgentMetrics\n─────────\ntool_call_total\nintent_classifications\niterations_per_request\nrequest_duration"]
        WorkerMetrics["WorkerMetrics\n─────────\ndocuments_processed\nprocessing_duration"]
    end

    subgraph Endpoints["Metrics Endpoints"]
        APIMetrics["API :8080/metrics"]
        WorkerEndpoint["Worker :9090/metrics"]
    end

    subgraph MonStack["Monitoring Stack"]
        Prometheus["Prometheus\n─────────\nscrape targets\nrecording rules\nalert rules"]
        Grafana["Grafana\n─────────\npre-built dashboards\ndata source: Prometheus"]
        Alertmanager["Alertmanager\n─────────\nalert routing\nnotifications"]
    end

    HTTPMetrics --> APIMetrics
    AgentMetrics --> APIMetrics
    WorkerMetrics --> WorkerEndpoint

    Prometheus -->|"scrape"| APIMetrics
    Prometheus -->|"scrape"| WorkerEndpoint
    Prometheus -->|"alerts"| Alertmanager
    Grafana -->|"query"| Prometheus
​```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| HTTPMetrics | `internal/observability/metrics/http.go` |
| AgentMetrics | `internal/observability/metrics/agent_metrics.go` |
| WorkerMetrics | `internal/observability/metrics/worker.go` |
| Prometheus config | `deploy/monitoring/prometheus/` |
| Grafana dashboards | `deploy/monitoring/grafana/dashboards/` |
| Alertmanager config | `deploy/monitoring/alertmanager/` |
```

- [ ] **Step 3: Update `README.md` — add architecture section**

Find the line `## Архитектура` in README.md and add a new section right before it:

```markdown
## Документация по архитектуре

- **[C4 Architecture (Context → Container → Component)](docs/c4/README.md)** — полная карта системы с Mermaid-диаграммами для каждой фичи
- **[Architecture & Business Logic](docs/architecture.md)** — описание бизнес-логики, пайплайнов и sequence-диаграммы
- **[ADR](docs/adr/)** — записи архитектурных решений

---
```

- [ ] **Step 4: Verify all links resolve**

```bash
# Check that all referenced source files exist
for f in cmd/api/main.go cmd/worker/main.go internal/bootstrap/bootstrap.go \
  internal/adapters/http/router.go internal/core/usecase/agent_chat.go \
  internal/core/usecase/ingest.go internal/core/usecase/process.go \
  internal/core/usecase/query.go internal/infrastructure/mcp/server.go \
  internal/infrastructure/graph/neo4j/client.go \
  ui/src/App.tsx ui/src/pages/ChatPage.tsx; do
  [ -f "$f" ] && echo "OK: $f" || echo "MISSING: $f"
done
```

Expected: all files exist.

- [ ] **Step 5: Commit**

```bash
git add docs/c4/c4-tauri-ui.md docs/c4/c4-monitoring.md README.md
git commit -m "docs(c4): add L3 Tauri UI, Monitoring; update README with architecture links"
```
