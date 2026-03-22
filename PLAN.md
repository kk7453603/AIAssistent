# PLAN: Personal AI Assistant (Roadmap v2)

## Цель (2-3 месяца)
Из MVP RAG-сервиса перейти к "второму мозгу": добавить измеримое качество retrieval, agent loop с инструментами и долговременную память, не ломая текущую Clean Architecture.

## Текущий baseline (на 2026-02-14)
- [x] Рабочий ingestion pipeline: upload -> NATS -> worker -> Qdrant/Postgres.
- [x] OpenAI-compatible API + OpenWebUI tool integration.
- [x] Spec-first API (`openapi.yaml` + генерация).
- [x] ADR и архитектурная карта (`docs/adr/*`, `docs/architecture.md`).
- [x] Базовые unit/integration тесты по use cases/adapter'ам.
- [x] Health endpoint (`GET /healthz`).

## Подтвержденные gap'ы (по коду)
- [x] Добавлены базовые продуктовые метрики и тех. observability (latency/error/token usage, queue lag).
- [x] Добавлен evaluation-framework для retrieval quality (`scripts/eval`, `make eval`).
- [x] Retrieval не semantic-only: hybrid + rerank доступны через конфиг.
- [x] Добавлен agent loop (итеративный tool-use) и долгосрочная память диалогов.
- [ ] Ingest нестабилен из-за LLM-классификации (ошибки JSON ломают обработку).
- [ ] Reranker недостаточно устойчив к RU (ASCII-токенизация).

## Приоритизация
- `P0` (критично): observability + evaluation + agent skeleton.
- `P1` (высокий impact): hybrid search + reranking + memory retrieval.
- `P2` (расширение): персональные источники знаний, multi-agent сценарии.

## План по этапам

### Этап 1 (Недели 1-2): Observability + Quality Baseline
- [x] Добавить structured logging (`request_id`, `document_id`, `duration_ms`, `model`).
- [x] Добавить базовые метрики API/worker (RPS, p95 latency, error rate, queue lag).
- [x] Добавить метрики LLM/RAG (tokens in/out, retrieval hit-rate, no-context rate).
- [x] Подготовить `scripts/eval/` для offline-набора кейсов (`precision@k`, `recall@k`, `MRR`).
- [x] Зафиксировать baseline-результаты в `docs/evaluation/baseline.md`.

Definition of Done:
- `GET /metrics` доступен для API и worker.
- Есть минимум 30-50 eval кейсов и повторяемый запуск через `make eval`.
- Видно текущий baseline retrieval quality и latency.

### Этап 2 (Недели 3-4): Advanced Retrieval
- [x] Реализовать hybrid retrieval (semantic + lexical/BM25).
- [x] Добавить configurable fusion strategy (например, RRF).
- [x] Добавить reranking top-N кандидатов.
- [x] Добавить A/B режимы (`semantic`, `hybrid`, `hybrid+rerank`) через конфиг.
- [x] Обновить eval и сравнить режимы на одном датасете.

Definition of Done:
- На eval-наборе `hybrid+rerank` улучшает `MRR`/`precision@k` против semantic-only. (подтверждено на smoke-наборе; полный прогон 30-50 кейсов запланирован отдельно)
- Режим retrieval переключается без изменения бизнес-логики adapter слоя.

### Этап 2.5 (Недели 4-5): Stabilize Ingest + Multilingual Rerank
- [ ] Убрать LLM-классификацию из ingest: перейти на детерминированное извлечение метаданных (frontmatter/путь/файл) и сделать классификацию best-effort/async.
- [ ] Унифицировать токенизацию для reranker и sparse: Unicode-aware токены (RU/EN), общий helper.
- [ ] Расширить метаданные документа: `source_type`, `source_id`, `path`, `tags`, `headers`, `created_at`, `updated_at`.
- [ ] Ввести `SourceAdapter`/`Ingestor` интерфейс под будущие источники (Obsidian, веб, файлы, базы знаний).

Definition of Done:
- Ingest не падает из-за LLM-ответов; все документы проходят векторизацию.
- Rerank адекватно работает на RU/EN (подтверждено на hard-case eval).
- Метаданные унифицированы и пригодны для multi-source фильтров.

### Этап 2.6 (Недели 5-6): Multi-Source Ready RAG
- [ ] Единый контракт на метаданные и схему фильтров (source, tags, path, time).
- [ ] Векторная коллекция с namespace/tenant-изолированием по источнику.
- [ ] Конфигurable chunking на источник (md headers для Obsidian, plain/fixed для файлов).

Definition of Done:
- Добавление нового источника не требует изменений в retrieval core.

### Этап 3 (Недели 5-6): Agent Loop + Memory
- [x] Добавить доменную модель агента (итерации, лимиты, план шагов).
- [x] Реализовать минимум 2 инструмента:
  - [x] `KnowledgeSearchTool` (текущий RAG).
  - [x] `TaskTool` (full CRUD + delete задачи для персональной памяти).
- [x] Ввести short-term memory (последние N сообщений) + long-term summaries в vector store.
- [x] Добавить сохранение и retrieval past conversations по semantic search.
- [x] Добавить guardrails: max iterations, timeout, safe tool error handling.

Definition of Done:
- Агент может сделать multi-step сценарий (поиск -> уточнение -> финальный ответ).
- В новом диалоге агент извлекает релевантный контекст из прошлых разговоров.

### Этап 4 (Недели 7-8): Hardening + Productization
- [x] Добавить rate limiting и backpressure.
- [x] Добавить resilience для Ollama/Qdrant/NATS (retry policy + circuit breaker pattern).
- [x] Добавить dashboards/alerts (минимум latency, errors, queue backlog).
- [x] Улучшить OpenWebUI trigger logic (intent detection вместо простых keywords).
- [x] Подготовить demo-сценарии и README-раздел "production runbook".

Definition of Done:
- Сервис выдерживает деградацию внешнего зависимого сервиса без полной недоступности.
- Есть runbook и понятный набор алертов для ручной эксплуатации.

### Этап 5 (Недели 9-10): Desktop UI (Tauri) — DONE

- [x] Scaffold: Tauri + React + Tailwind + Zustand + Vite
- [x] Chat UI: SSE streaming, markdown rendering, think blocks, conversation management
- [x] Model selector in InputBar (fetches from Ollama)
- [x] Vault Browser: file tree, markdown preview, search, vault selector
- [x] Dashboard: tool statistics from Prometheus metrics, MCP server status, activity feed
- [x] Settings: General (API URL, vaults path, theme, language), Models (Ollama list, pull, selectors), MCP (add/remove servers), Agent (iterations, timeouts, toggles)
- [x] System tray + global hotkeys + quick-ask mini-window (Tauri-only)
- [x] Theme switching (light/dark/system) with Tailwind dark mode
- [x] Mobile responsive layout (collapsible sidebar, icon-only nav)
- [x] CORS middleware for browser-based frontend
- [x] Conversation persistence (messages saved per conversation in localStorage)
- [x] Favicon

Specs: `docs/superpowers/specs/2026-03-22-ui-*.md`

### Этап 5.5: UI Bug Fixes & Polish — DONE

- [x] Fix SearXNG connection (port mismatch 8888→8080, limiter disabled)
- [x] Fix UTF-8 sanitization for web search results (PostgreSQL SQLSTATE 22021)
- [x] Fix tool status deduplication (spinner stuck after streaming)
- [x] Fix Dashboard metrics parser (metric name mismatch, label order)
- [x] Connect Activity Feed to chat tool events
- [x] Improve agent system prompt (conversation context, disambiguated search queries)
- [x] Increase ShortMemoryMessages 12→20

### Планируемые улучшения (Этап 6+)

#### Уровень 1: UX и взаимодействие

- [ ] Multi-modal input (vision models: LLaVA/Qwen-VL, фото → описание → vault)
- [ ] Voice input/output (Whisper STT → агент → TTS)
- [ ] Telegram бот (полноценный интерфейс к агенту)
- [ ] Proactive notifications (напоминания о задачах через Telegram/webhook)
- [ ] Mobile app (React Native or Tauri Mobile)

#### Уровень 2: Качество и память

- [ ] PDF/DOCX/OCR document extraction
- [ ] RAG evaluation pipeline (RAGAS, faithfulness, relevancy)
- [ ] Adaptive model routing (автовыбор модели по типу задачи)
- [ ] Knowledge graph (граф связей между заметками)
- [ ] Auto-tagging & classification (автотегирование при синхронизации)
- [ ] Fine-tuned embedding models for RU

#### Уровень 3: Автоматизация и агенты

- [ ] Multi-agent orchestration (researcher, coder, writer)
- [ ] Scheduled tasks (recurring cron-подобный scheduler)
- [ ] Self-improving agent (анализ ошибок → предложение улучшений)
- [ ] Plugin system for custom tools (добавление тулов через YAML/JSON без Go кода)

#### Уровень 4: Инфраструктура

- [ ] Fine-tuning pipeline (LoRA/QLoRA на своих данных)
- [ ] Vector DB optimization (ColBERT, multi-vector)
- [ ] Edge deployment (мобильный агент через ONNX/llama.cpp)
- [ ] E2E encrypted sync
- [ ] Offline mode (local-first)
- [ ] Multi-user support with auth

## Планируемые изменения по коду
- `internal/adapters/http/`: middleware для request-id, metrics, rate limit.
- `internal/core/usecase/query.go`: стратегия retrieval и hooks под evaluation/metrics.
- `internal/core/domain/`: agent/memory сущности, limits, errors.
- `internal/infrastructure/vector/qdrant/`: hybrid search + rerank adapters.
- `internal/infrastructure/llm/ollama/`: telemetry по inference, retries/timeouts.
- `scripts/eval/`: генерация/запуск eval-кейсов и экспорт отчетов.
- `internal/core/usecase/process.go`: заменить LLM-классификацию на детерминированный extractor.
- `internal/infrastructure/extractor/`: frontmatter/path/tag parser для Obsidian + shared metadata schema.

## Риски и решения
- Риск: рост latency из-за reranking.
  - План: двухступенчатый retrieval (cheap recall -> selective rerank top-N).
- Риск: усложнение архитектуры агента.
  - План: начать с single-agent loop, без multi-agent orchestration.
- Риск: отсутствие объективной оценки прогресса.
  - План: не принимать изменения retrieval без прогонки eval-набора.

## Рабочий ритм (с учетом режима обучения)
- Будни (1-2 ч): мелкие задачи, тесты, фиксы, документация.
- Выходные (3-4 ч): крупные фичи этапа и интеграционные проверки.
- Чек-ин: каждое воскресенье 17:00, короткий weekly review с фактами по метрикам.

## KPI на конец roadmap
- Retrieval quality: +20-30% по `MRR` относительно baseline.
- Надежность: p95 API latency и error rate под наблюдением, без "слепых зон".
- Product capability: минимум один устойчивый multi-step agent workflow.

## UI Specs

- [Scaffold](docs/superpowers/specs/2026-03-22-ui-1-scaffold.md)
- [Chat UI](docs/superpowers/specs/2026-03-22-ui-2-chat.md)
- [Vault Browser](docs/superpowers/specs/2026-03-22-ui-3-vault-browser.md)
- [Dashboard & Settings](docs/superpowers/specs/2026-03-22-ui-4-dashboard-settings.md)
- [System Tray & Hotkeys](docs/superpowers/specs/2026-03-22-ui-5-tray-hotkeys.md)
