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
- [ ] Retrieval пока semantic-only (без hybrid BM25 + rerank).
- [x] Добавлен agent loop (итеративный tool-use) и долгосрочная память диалогов.

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

## Планируемые изменения по коду
- `internal/adapters/http/`: middleware для request-id, metrics, rate limit.
- `internal/core/usecase/query.go`: стратегия retrieval и hooks под evaluation/metrics.
- `internal/core/domain/`: agent/memory сущности, limits, errors.
- `internal/infrastructure/vector/qdrant/`: hybrid search + rerank adapters.
- `internal/infrastructure/llm/ollama/`: telemetry по inference, retries/timeouts.
- `scripts/eval/`: генерация/запуск eval-кейсов и экспорт отчетов.

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
