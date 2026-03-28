# C4 Architecture Documentation — Design Spec

## Goal

Create comprehensive C4 architecture documentation covering all project features, placed in `docs/c4/`, referenced from `README.md`.

## Scope

- **Level 1 (System Context)**: One diagram — PAA as a whole, external actors and systems.
- **Level 2 (Container)**: One diagram — API, Worker, Tauri UI, all infrastructure services.
- **Level 3 (Component)**: 10 diagrams — one per feature area, showing internal components, use cases, adapters, and infrastructure dependencies.
- **Index**: `docs/c4/README.md` with navigation and glossary.
- **README.md**: New "Архитектура" section linking to C4 docs.

## File Structure

```
docs/c4/
├── README.md                  # Index — navigation across all diagrams
├── c4-context.md              # L1 — System Context
├── c4-containers.md           # L2 — Container diagram
├── c4-rag-pipeline.md         # L3 — RAG: ingest, extract, chunk, embed, query, rerank
├── c4-agent-loop.md           # L3 — Agent: planner, tool dispatch, memory, cascading search
├── c4-obsidian.md             # L3 — Obsidian: vault sync, note creation, file browser
├── c4-knowledge-graph.md      # L3 — Knowledge Graph: Neo4j, link extraction, query expansion
├── c4-multi-agent.md          # L3 — Orchestrator: researcher, coder, writer, critic
├── c4-self-improvement.md     # L3 — Events, feedback, analysis, auto-apply
├── c4-scheduled-tasks.md      # L3 — Cron scheduler, conditions, webhooks
├── c4-mcp.md                  # L3 — MCP server + external MCP clients
├── c4-tauri-ui.md             # L3 — Tauri: React pages, stores, API client
└── c4-monitoring.md           # L3 — Prometheus, Grafana, Alertmanager, metrics
```

## Diagram Format

All diagrams use **Mermaid** syntax (renders on GitHub). C4 diagrams use `C4Context`, `C4Container`, `C4Component` Mermaid directives where supported, or `flowchart` with C4 naming conventions where Mermaid C4 syntax is limited. Sequence diagrams for key flows.

## L1: System Context

**External Actors:**
- User (via Tauri Desktop App)
- User (via OpenWebUI)
- MCP Clients (Claude Code, Cursor, external tools)

**External Systems:**
- LLM Providers: Ollama (self-hosted), OpenRouter, Groq, Together, Cerebras, HuggingFace
- SearXNG (self-hosted web search)
- Obsidian Vaults (local filesystem)
- Neo4j (optional knowledge graph)

**PAA System Boundary:**
- API Server — HTTP endpoints, OpenAI-compatible API, MCP server
- Worker — async document processing pipeline
- Tauri Desktop App — React frontend

## L2: Containers

| Container | Technology | Responsibility |
|-----------|-----------|---------------|
| API Server | Go binary (`cmd/api`) | HTTP handlers, agent loop, RAG query, MCP server, metrics |
| Worker | Go binary (`cmd/worker`) | NATS subscriber, document processing, LLM enrichment, scheduler cron |
| Tauri Desktop | Rust + React/Vite | Chat UI, Obsidian browser, settings, dashboard |
| PostgreSQL | PostgreSQL 16 | Documents, conversations, tasks, memory, orchestrations, events, feedback, schedules |
| Qdrant | Qdrant | Multi-collection vector search (per source_type + memory) |
| NATS | NATS | Async document processing + enrichment queues |
| Ollama | Ollama | LLM inference + embeddings + model discovery |
| Neo4j | Neo4j 5 | Knowledge graph (optional) |
| SearXNG | SearXNG | Self-hosted web search |
| Prometheus | Prometheus | Metrics collection |
| Grafana | Grafana | Dashboards |
| Alertmanager | Alertmanager | Alert routing |

## L3: Component Details Per Feature

### 1. RAG Pipeline (`c4-rag-pipeline.md`)

**Components:**
- IngestUseCase — upload orchestration, source adapter routing
- ProcessUseCase — extract → classify → chunk → embed → index pipeline
- QueryUseCase — embed query → search → rerank → generate answer
- QueryFusionUseCase — multi-query expansion
- RerankUseCase — cross-encoder + BM25 hybrid scoring
- ExtractorRegistry — MIME-type routing (plaintext, PDF, DOCX, XLSX/CSV)
- ChunkerRegistry — per-source chunking (fixed, markdown)
- SourceAdapters — upload, web scraping, obsidian stub

**Key Flows:**
1. Document Upload → IngestUC → NATS → ProcessUC → Qdrant
2. Query → QueryUC → Qdrant Search → Rerank → LLM Generate

**Code Anchors:** `internal/core/usecase/ingest.go`, `process.go`, `query.go`, `rerank.go`, `query_fusion.go`, `internal/infrastructure/extractor/`, `internal/infrastructure/chunking/`

### 2. Agent Loop (`c4-agent-loop.md`)

**Components:**
- AgentChatUseCase — main loop: planner → tool dispatch → persist
- IntentClassifier — keyword + LLM classification
- ComplexityClassifier — rule-based + LLM tier assignment
- ToolResultCache — dedup repeated tool calls
- ModelRouter — adaptive model selection by complexity tier
- Guardrails — code safety checks
- ConversationStore — short-term memory (PostgreSQL)
- MemoryVectorStore — long-term memory (Qdrant)

**Key Flows:**
1. Complete() → intent classify → build system prompt → loop(ChatWithTools → executeToolCall)
2. Cascading search: knowledge_search → LLM memory → web_search → obsidian_write
3. Memory: persist messages → periodic summary → embed → index

**Code Anchors:** `internal/core/usecase/agent_chat.go`, `intent.go`, `complexity.go`, `tool_cache.go`, `guardrails.go`, `tool_helpers.go`

### 3. Obsidian Integration (`c4-obsidian.md`)

**Components:**
- ObsidianHandlers — vault CRUD, sync, note creation, file browser
- ObsidianSourceAdapter — stub for vault ingest
- ObsidianNoteWriter — agent tool for note creation
- VaultConfig — JSON-based vault registry

**Key Flows:**
1. Vault registration → config save → sync (walk files → hash → ingest new/changed)
2. Agent obsidian_write → ObsidianNoteWriter → create .md file on disk

**Code Anchors:** `internal/adapters/http/obsidian_handlers.go`, `internal/infrastructure/source/obsidian/`

### 4. Knowledge Graph (`c4-knowledge-graph.md`)

**Components:**
- Neo4jGraphStore — CRUD operations on nodes/edges
- LinkExtractor — wikilink parsing from document text
- SimilarityIndexer — semantic similarity between documents
- GraphQueryExpander — expand search queries using graph relations
- GraphAPI — `GET /v1/graph` endpoint

**Key Flows:**
1. ProcessUC → extract links → upsert graph nodes → add similarity edges
2. AgentChat → expandQueryWithGraph → FindByTitle → GetRelated → augment query

**Code Anchors:** `internal/infrastructure/graph/neo4j/`, `internal/core/usecase/links.go`, `internal/core/usecase/agent_chat.go:expandQueryWithGraph`

### 5. Multi-Agent Orchestration (`c4-multi-agent.md`)

**Components:**
- OrchestratorUseCase — dynamic agent dispatch
- AgentRegistry — specialist agent specs (researcher, coder, writer, critic)
- OrchestrationStore — persistent history (PostgreSQL)
- SharedMemory — Qdrant-backed agent memory

**Key Flows:**
1. shouldOrchestrate() → OrchestratorUseCase.Execute() → dispatch specialists → aggregate results

**Code Anchors:** `internal/core/usecase/orchestrator.go`, `agent_registry.go`, `internal/infrastructure/repository/postgres/orch_repo.go`

### 6. Self-Improvement (`c4-self-improvement.md`)

**Components:**
- EventCollector — records agent execution events
- FeedbackStore — user feedback on responses
- SelfImproveUseCase — periodic LLM analysis of events + feedback
- ImprovementStore — generated improvement suggestions
- AutoApplier — safe auto-apply of improvements

**Key Flows:**
1. Agent loop → EventCollector.Record() → periodic analysis → generate improvements
2. User feedback → POST /v1/feedback → stored → analyzed

**Code Anchors:** `internal/core/usecase/event_collector.go`, `self_improve.go`, `internal/infrastructure/repository/postgres/events_repo.go`, `feedback_repo.go`, `improvements_repo.go`

### 7. Scheduled Tasks (`c4-scheduled-tasks.md`)

**Components:**
- SchedulerUseCase — cron scheduler using robfig/cron/v3
- NaturalLanguageParser — NL → cron expression
- ConditionEvaluator — conditional execution
- WebhookNotifier — POST results to webhook URL
- ScheduleStore — CRUD in PostgreSQL
- ScheduleAPI — `/v1/schedules` endpoints

**Key Flows:**
1. Create schedule → store → cron registers → periodic execute → record run
2. Conditional: evaluate condition → skip or execute

**Code Anchors:** `internal/core/usecase/scheduler.go`, `internal/infrastructure/repository/postgres/schedule_repo.go`

### 8. MCP Integration (`c4-mcp.md`)

**Components:**
- MCPServer — Streamable HTTP handler (`/mcp`)
- MCPToolRegistry — aggregates built-in + external tools
- MCPClient — connects to external MCP servers (filesystem, code-runner, GitHub)
- HTTPToolsPlugin — JSON-defined HTTP API tools

**Key Flows:**
1. External client → POST /mcp → MCPServer → dispatch to tool handler → return result
2. Agent → toolRegistry.CallMCPTool() → MCPClient → external server → response

**Code Anchors:** `internal/infrastructure/mcp/server.go`, `client.go`, `registry.go`, `http_tools.go`

### 9. Tauri Desktop UI (`c4-tauri-ui.md`)

**Components:**
- ChatPage — streaming chat with SSE, think blocks, tool status
- ObsidianPage — vault browser and management
- SettingsPage — model selection, MCP config, agent params
- DashboardPage — tool usage statistics
- GraphPage — 3D knowledge graph visualization
- API Client — typed fetch wrappers for all endpoints
- Zustand Stores — chat, settings, obsidian, graph state

**Code Anchors:** `ui/src/pages/`, `ui/src/api/`, `ui/src/stores/`, `ui/src/components/`

### 10. Monitoring (`c4-monitoring.md`)

**Components:**
- HTTPMetrics — request duration, status codes, in-flight (Prometheus)
- AgentMetrics — tool call counts, intent classifications, iterations, duration
- WorkerMetrics — document processing counts and durations
- PrometheusConfig — scrape targets, recording rules, alerts
- GrafanaDashboards — pre-built dashboards
- AlertmanagerConfig — alert routing

**Key Flows:**
1. API/Worker → expose /metrics → Prometheus scrape → Grafana visualize
2. Alert rules → Alertmanager → notification

**Code Anchors:** `internal/observability/metrics/`, `deploy/monitoring/`

## README.md Changes

Add new section "Архитектура" after the architecture ASCII diagram, before "Быстрый старт":

```markdown
## Документация по архитектуре

- **[C4 Architecture (Context → Container → Component)](docs/c4/README.md)** — полная карта системы с Mermaid-диаграммами
- **[Architecture & Business Logic](docs/architecture.md)** — описание бизнес-логики, пайплайнов, sequence-диаграммы
- **[ADR](docs/adr/)** — записи архитектурных решений
```

## Verification

1. All Mermaid diagrams render correctly on GitHub
2. All code anchor file paths are valid
3. README.md links resolve
4. No broken internal links between docs/c4/ files
5. Every feature from CLAUDE.md is covered by at least one L3 diagram
