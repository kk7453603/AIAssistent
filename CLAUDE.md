# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Personal AI Assistant (PAA) — Go-based cascading AI assistant with OpenAI-compatible API. Searches Obsidian vaults first (Qdrant RAG), falls back to LLM memory, then to web search (SearXNG). Supports multi-source document ingestion (upload, web scraping, Obsidian), multi-format extraction (PDF, DOCX, XLSX, CSV, Markdown), multi-agent orchestration, knowledge graph (Neo4j), adaptive model routing, self-improving agent, scheduled tasks, and MCP tool server connections.

## Build & Run

```bash
# Full stack (Docker)
docker compose up -d --build

# With host GPU Ollama
docker compose -f docker-compose.yml -f docker-compose.host-gpu.yml up -d --build

# Pull required models
docker compose exec ollama ollama pull llama3.1:8b
docker compose exec ollama ollama pull nomic-embed-text
```

## Development Commands

```bash
make test                # go test ./...
make vet                 # go vet ./...
make test-core-cover     # coverage for core + http adapters
make generate            # regenerate OpenAPI server code
make monitoring-validate # validate Prometheus rules
make eval                # run RAG evaluation suite
make eval-ragas          # RAGAS evaluation (faithfulness, relevancy, correctness)
```

Run a single test:
```bash
go test ./internal/core/usecase/ -run TestQueryUseCase -v
```

## Architecture

Hexagonal (ports & adapters) with two binaries:

- **`cmd/api`** — HTTP server (OpenAI-compatible `/v1/chat/completions`, document upload, RAG query, Obsidian vault management + note creation, Prometheus metrics)
- **`cmd/worker`** — NATS subscriber for document processing (extract → metadata → chunk → embed → index → graph) and async LLM enrichment. Also runs scheduler cron and self-improvement analysis.

Key packages:
- `internal/core/domain` — domain models (Document, Conversation, Task, Memory)
- `internal/core/ports` — inbound/outbound interfaces (DocumentIngestor, DocumentQueryService, AgentChatService, VectorStore, etc.)
- `internal/core/usecase` — business logic (ingest, process, query, rerank, query_fusion, agent_chat with cascading search)
- `internal/adapters/http` — HTTP handlers, OpenAI-compat chat/streaming, SSE, Obsidian note creation, middleware (rate limit, backpressure)
- `internal/infrastructure/` — implementations: `ollama/` (LLM), `qdrant/` (vectors), `postgres/` (repos), `nats/` (queue), `localfs/` (storage), `resilience/` (retry + circuit breaker), `websearch/searxng/` (web search)
- `internal/infrastructure/extractor/` — document text extraction: `plaintext/`, `pdf/`, `docx/`, `spreadsheet/` (XLSX/CSV), `metadata/` (frontmatter), `registry.go` (MIME-type routing)
- `internal/infrastructure/source/` — multi-source ingest adapters: `upload/`, `web/` (HTML→text), `obsidian/` (stub)
- `internal/infrastructure/graph/neo4j/` — knowledge graph (Neo4j): link extraction, semantic similarity, graph traversal
- `internal/infrastructure/mcp/` — MCP tool registry, external MCP servers, HTTP tools plugin (`http_tools.go`)
- `internal/pkg/tokenizer/` — shared Unicode-aware tokenizer for reranker and sparse search
- `internal/bootstrap` — dependency wiring (App struct)
- `internal/config` — env-based configuration

## Code Generation

OpenAPI spec: `api/openapi/openapi.yaml`
Generated server code: `internal/adapters/http/openapi/server.gen.go`
Config: `api/openapi/oapi-codegen.yaml`

Regenerate after spec changes:
```bash
make generate-openapi
# or: go generate ./internal/adapters/http/openapi
```

## Infrastructure Services

PostgreSQL (document state, conversations, tasks, memory, orchestrations, events, feedback, improvements, schedules), Qdrant (multi-collection vector search — per source_type + memory), NATS (async document processing + enrichment queues), Ollama (LLM + embeddings + model discovery), Neo4j (knowledge graph — optional), SearXNG (self-hosted web search), OpenWebUI (chat UI).

DB migrations: `db/migrations/001_init.sql`, `002_add_metadata.sql` (applied via `repo.EnsureSchema` — includes orchestrations, events, feedback, improvements, schedules tables)

## Configuration

All config via environment variables (see `.env.example`). Key settings:
- `CHUNK_STRATEGY`: `fixed` (default) or `markdown`
- `RAG_RETRIEVAL_MODE`: `semantic`, `hybrid`
- `AGENT_MODE_ENABLED`: enables server-side agent loop with cascading search, memory/tools (enabled by default)
- `WEB_SEARCH_ENABLED`: enables SearXNG web search as fallback tool
- `WEB_SEARCH_URL`: SearXNG instance URL (default `http://searxng:8888`)
- `OPENAI_COMPAT_API_KEY`: optional bearer auth for API
- `QDRANT_SEARCH_ORDER`: cascading search priority across source collections (default `upload,web,obsidian`)
- `CHUNK_CONFIG`: per-source chunking (JSON, e.g. `{"obsidian":{"strategy":"markdown","chunk_size":1200}}`)
- `MODEL_ROUTING`: adaptive model selection by complexity (JSON, or auto-discovered from Ollama)
- `GRAPH_ENABLED`: enables Neo4j knowledge graph (requires Neo4j service)
- `ORCHESTRATOR_ENABLED`: enables multi-agent orchestration (researcher, coder, writer, critic)
- `SELF_IMPROVE_ENABLED`: enables periodic self-improvement analysis
- `SCHEDULER_ENABLED`: enables cron-based scheduled tasks
- `HTTP_TOOLS`: JSON-defined HTTP API tools for the agent

## Agent Cascading Search Strategy

When agent mode is enabled, the planner follows a strict cascade:
1. **knowledge_search** — search Obsidian vaults / uploaded documents in Qdrant
2. **LLM memory** — answer from model knowledge with disclaimer "В базе знаний информация не найдена"
3. **web_search** — search the internet via SearXNG if LLM doesn't know
4. **obsidian_write** — suggest saving found information to an appropriate Obsidian vault (only after user confirmation)

Agent tools: `knowledge_search`, `web_search`, `obsidian_write`, `task_tool`, plus any MCP tools and HTTP tools from config

Chain-of-thought reasoning is visible in OpenWebUI as collapsible `<think>` blocks.

## OpenWebUI Integration

- Pipe function: `deploy/openwebui/functions/paa_rag_pipe.py` — intercepts file uploads → PAA ingest, proxies chat → `/v1/chat/completions`
- Obsidian vault management tool: `deploy/openwebui/tools/assistant_obsidian_vaults.py`
- MCP/tool servers: connectable via OpenWebUI Settings → External Tools (`ENABLE_DIRECT_CONNECTIONS=true`)

## Monitoring

- Prometheus: `deploy/monitoring/prometheus/`
- Alertmanager: `deploy/monitoring/alertmanager/`
- Grafana dashboards: `deploy/monitoring/grafana/dashboards/`
- API metrics at `:8080/metrics`, Worker metrics at `:9090/metrics`

## New Features (Этап 6)

- **Multi-Source Ingest**: `SourceAdapter` pattern — upload, web scraping (HTML→text), Obsidian stub. `IngestFromSource()` universal method.
- **Multi-Format Extraction**: `ExtractorRegistry` routes by MIME-type — PDF (`ledongthuc/pdf`), DOCX (XML parsing), XLSX (`excelize`), CSV (stdlib).
- **Multi-Collection Qdrant**: Each source_type gets its own collection (`documents_upload`, `documents_web`, etc.) with cascading search by priority.
- **Per-Source Chunking**: `ChunkerRegistry` + `CHUNK_CONFIG` JSON — markdown for Obsidian, fixed for web.
- **Adaptive Model Routing**: `ComplexityClassifier` (rule-based + LLM fallback) → auto-select model by tier. Auto-discovery from Ollama.
- **Knowledge Graph**: Neo4j — wikilinks, shared tags, semantic similarity. Retrieval boost + graph-based query rewriting. `GET /v1/graph` API.
- **Multi-Agent Orchestration**: Dynamic orchestrator with configurable specialist agents. Shared Qdrant memory. Persistent Postgres history.
- **Self-Improving Agent**: Event collection, user feedback (`POST /v1/feedback`), periodic LLM analysis, auto-apply safe improvements.
- **JSON HTTP Tools**: Define HTTP API tools via JSON config (`HTTP_TOOLS`). Env var expansion, body templates, JSONPath output.
- **Scheduled Tasks**: Cron scheduler (`robfig/cron/v3`). Natural language → cron. Conditional execution. Webhooks. CRUD API (`/v1/schedules`).
- **RAGAS Evaluation**: Python pipeline (`scripts/eval/ragas_eval.py`) — faithfulness, relevancy, correctness, context precision/recall.
- **Unified Tokenizer**: `internal/pkg/tokenizer.TokenizeUnicode` — shared RU/EN Unicode-aware tokenizer.

## Language

Project documentation and commit messages are in Russian. Code, comments, and identifiers are in English.
