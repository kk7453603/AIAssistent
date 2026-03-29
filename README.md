# Personal AI Assistant (PAA)

**Go-based cascading AI assistant with RAG, agent tools, Obsidian integration, and a Tauri desktop UI.**

**AI-ассистент на Go с каскадным поиском, RAG-пайплайном, интеграцией с Obsidian и десктопным UI на Tauri.**

![Go Version](https://img.shields.io/badge/Go-1.25.6-00ADD8?logo=go&logoColor=white)
![Coverage](https://kk7453603.github.io/AIAssistent/badges/coverage.svg)
[![License: PolyForm Small Business 1.0.0](https://img.shields.io/badge/License-PolyForm%20Small%20Business%201.0.0-blue.svg)](https://polyformproject.org/licenses/small-business/1.0.0)

---

## Возможности

### AI Agent

- Каскадный поиск: база знаний (Qdrant) -> память LLM -> веб-поиск (SearXNG)
- Native function calling с автоматическим выбором инструментов
- Chain-of-thought рассуждения (блоки `<think>`, видны в UI как сворачиваемые секции в стиле Claude.ai)
- Intent router для классификации намерений пользователя
- Оркестрация инструментов: `knowledge_search`, `web_search`, `obsidian_write`, `task_tool`, MCP-инструменты, HTTP-инструменты
- Multi-agent orchestration (researcher, coder, writer, critic) с визуализацией в чате
- Adaptive model routing — автовыбор модели по сложности запроса
- Self-improving agent — анализ ошибок и автоулучшения

### RAG Pipeline

- Загрузка документов через API (multipart upload)
- Multi-format extraction: PDF, DOCX, XLSX, CSV, Markdown
- Multi-source ingest: upload, web scraping, Obsidian
- Детерминированная классификация (frontmatter/path) + async LLM enrichment
- Чанкинг: фиксированный (`fixed`) и по структуре Markdown (`markdown`), настраиваемый по источнику
- Гибридный retrieval: семантический поиск + BM25 + reranking
- Query expansion (multi-query retrieval)
- Multi-collection Qdrant с каскадным поиском по источникам
- Knowledge Graph (Neo4j) — wikilinks, similarity, retrieval boost

### Управление знаниями

- Синхронизация Obsidian vault: автоматическая по расписанию и ручная
- Auto-discovery vault-ов при старте
- Создание заметок в Obsidian vault через агента
- Браузер vault с UI-управлением
- Поддержка нескольких vault одновременно

### Десктопный UI (Tauri)

- Чат с потоковой генерацией (SSE streaming)
- Отображение chain-of-thought в стиле Claude.ai (Sparkles icon, таймер, amber theme)
- Визуализация Multi-Agent Orchestration (vertical stepper в чате)
- 3D Knowledge Graph (react-force-graph-3d) — интерактивная визуализация связей документов
- Выбор модели и LLM-провайдера
- История разговоров с поиском
- Браузер Obsidian vault с auto-discovery
- Дашборд: документы, статистика инструментов, MCP, расписания, self-improving agent, activity feed
- Настройки: тема, язык, модели, MCP-серверы, параметры агента, HTTP Tools
- Адаптивная верстка для мобильных устройств

### Память

- Краткосрочная: история разговоров в PostgreSQL
- Долгосрочная: векторные саммари в Qdrant (`conversation_memory`)
- Автоматическое суммирование каждые N ходов
- Retrieval релевантных воспоминаний при новых запросах

### Scheduled Tasks

- Cron-планировщик для периодических задач агента
- CRUD API (`/v1/schedules`) + UI на Dashboard
- Условное выполнение (conditions)
- Webhook-уведомления

### Веб-поиск

- Интеграция с SearXNG (self-hosted метапоисковик)
- Автоматический fallback, когда в базе знаний нет ответа

### MCP (Model Context Protocol)

- Встроенный MCP-сервер (`/mcp`) для внешних клиентов (OpenWebUI, Claude Code, Cursor)
- Подключение к внешним MCP-серверам: filesystem, code-runner, GitHub
- JSON HTTP Tools Plugin — определение HTTP API инструментов через конфиг
- Настройка через переменные окружения (JSON-массив)

### Инфраструктура

- Docker Compose для всего стека
- Поддержка хостового GPU (Ollama proxy через nginx)
- Несколько LLM-провайдеров: Ollama, OpenRouter, Groq, Together, Cerebras, HuggingFace
- Fallback LLM провайдер при ошибке основного
- Prometheus + Grafana + Alertmanager (дашборды, алерты, RAGAS evaluation dashboard)
- Rate limiting, backpressure, circuit breaker
- Retry с exponential backoff для внешних зависимостей
- CORS, structured JSON logging, request ID tracing

---

## Документация по архитектуре

- **[C4 Architecture (Context -> Container -> Component)](docs/c4/README.md)** — полная карта системы с Mermaid-диаграммами для каждой фичи
- **[Architecture & Business Logic](docs/architecture.md)** — описание бизнес-логики, пайплайнов и sequence-диаграммы
- **[ADR](docs/adr/)** — записи архитектурных решений

---

## Архитектура

```text
+---------------------------------------------------+
|               Desktop UI (Tauri)                   |
|      React + Tailwind + Zustand + Vite             |
|  Chat | Vaults | Dashboard | Graph | Settings      |
+---------------------+-----------------------------+
                      | HTTP/SSE
+---------------------v-----------------------------+
|              PAA API  (Go, cmd/api)                |
|   OpenAI-compatible /v1/chat/completions           |
|                                                    |
|   +---------------+    +--------------------+      |
|   |  Agent Loop    |    |  RAG Pipeline      |      |
|   |  (planner +    |    |  (query + rerank)  |      |
|   |   tools)       |    |                    |      |
|   +------+---------+    +--------+-----------+      |
|          |                       |                  |
|   +------v---------+    +--------v-----------+      |
|   | Tool Router    |    | Embedding +        |      |
|   | knowledge      |    | Generation         |      |
|   | web_search     |    | (Ollama / cloud)   |      |
|   | obsidian       |    |                    |      |
|   | task_tool      |    |                    |      |
|   | MCP tools      |    |                    |      |
|   | HTTP tools     |    |                    |      |
|   +----------------+    +--------------------+      |
+----+----+----+----+----+----+----+-----------------+
     |    |    |    |    |    |    |
  +--v-+ +v--+ +v--+ +v---+ +v------+ +v----+ +v----+
  |Qdr-| | PG| |NATS| |Olla-| |SearXNG| |Neo4j| |MCP  |
  |ant | |   | |    | |ma  | |       | |     | |srvs |
  +----+ +---+ +----+ +----+ +-------+ +-----+ +-----+
```

**Worker** (`cmd/worker`) — NATS-подписчик для асинхронной обработки документов: извлечение текста -> метаданные -> чанкинг -> эмбеддинг -> индексация в Qdrant -> граф (Neo4j).

### Гексагональная архитектура (Ports & Adapters)

| Пакет | Назначение |
| ----- | --------- |
| `internal/core/domain` | Доменные модели (Document, Conversation, Task, Memory, Orchestration) |
| `internal/core/ports` | Inbound/outbound интерфейсы |
| `internal/core/usecase` | Бизнес-логика (ingest, process, query, rerank, agent_chat, orchestrator, scheduler, self_improve) |
| `internal/adapters/http` | HTTP-обработчики, OpenAI-compat API, SSE, Obsidian vault management |
| `internal/infrastructure/` | Реализации: ollama, qdrant, postgres, nats, searxng, neo4j, mcp, resilience, extractors |
| `internal/pkg/tokenizer` | Shared Unicode-aware tokenizer (RU/EN) |
| `ui/` | Tauri + React фронтенд |

---

## Быстрый старт

```bash
# Клонировать репозиторий
git clone https://github.com/kk7453603/AIAssistent.git
cd AIAssistent

# Создать файл конфигурации
cp .env.example .env

# Вариант A: Docker Ollama (CPU)
docker compose up -d --build

# Вариант B: Хостовый GPU Ollama
docker compose -f docker-compose.yml -f docker-compose.host-gpu.yml up -d --build

# Загрузить модели в Ollama
docker compose exec ollama ollama pull llama3.1:8b
docker compose exec ollama ollama pull nomic-embed-text
```

После запуска:
- **Tauri UI**: `http://localhost:1420`
- **OpenWebUI**: `http://localhost:3000`
- **API**: `http://localhost:8080`

---

## Разработка

```bash
# Тесты
make test

# Статический анализ
make vet

# Покрытие core + HTTP-адаптеров
make test-core-cover

# Перегенерация OpenAPI-кода
make generate

# Валидация конфигурации мониторинга
make monitoring-validate

# RAG evaluation suite
make eval

# RAGAS evaluation (faithfulness, relevancy, correctness)
make eval-ragas
```

### UI (Tauri + React)

```bash
cd ui

# Vite dev server (только фронтенд)
npm run dev

# Полное Tauri-приложение
npm run tauri dev
```

### Один тест

```bash
go test ./internal/core/usecase/ -run TestQueryUseCase -v
```

### Spec-first API

OpenAPI-спецификация: `api/openapi/openapi.yaml`

```bash
# 1. Обновить спецификацию
# 2. Сгенерировать серверный код
make generate-openapi
# 3. Реализовать логику в internal/adapters/http/
# 4. Проверить
make test
```

---

## Конфигурация

Все настройки задаются через переменные окружения. Полный список -- в файле [`.env.example`](.env.example).

### Core

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `API_PORT` | `8080` | Порт API-сервера |
| `LOG_LEVEL` | `info` | Уровень логирования |
| `POSTGRES_DSN` | | DSN для PostgreSQL |
| `NATS_URL` | `nats://nats:4222` | URL NATS-сервера |
| `STORAGE_PATH` | `/data/storage` | Путь для хранения загруженных файлов |

### LLM / Embedding

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `OLLAMA_URL` | `http://ollama:11434` | URL Ollama API |
| `OLLAMA_GEN_MODEL` | `llama3.1:8b` | Модель генерации |
| `OLLAMA_PLANNER_MODEL` | `qwen3:14b` | Модель планировщика агента |
| `OLLAMA_EMBED_MODEL` | `nomic-embed-text` | Модель эмбеддингов |
| `LLM_PROVIDER` | `ollama` | Провайдер: `ollama`, `openai-compat`, `groq`, `together`, `openrouter`, `cerebras`, `huggingface` |
| `LLM_PROVIDER_URL` | | URL внешнего провайдера |
| `LLM_PROVIDER_KEY` | | API-ключ провайдера |
| `LLM_MODEL` | | Модель для внешнего провайдера |
| `EMBED_PROVIDER` | `ollama` | Провайдер эмбеддингов: `ollama`, `openai-compat` |
| `RERANKER_PROVIDER` | `fallback` | Провайдер reranker: `fallback`, `ollama`, `openai-compat` |

### Fallback LLM

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `LLM_FALLBACK_PROVIDER` | | Провайдер fallback LLM |
| `LLM_FALLBACK_URL` | | URL fallback провайдера |
| `LLM_FALLBACK_KEY` | | API-ключ fallback |
| `LLM_FALLBACK_MODEL` | | Модель fallback |
| `LLM_EXTRA_PROVIDERS` | | Доп. провайдеры через запятую (`huggingface,openrouter`) |

### RAG / Retrieval

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `QDRANT_URL` | `http://qdrant:6333` | URL Qdrant |
| `QDRANT_EMBED_DIM` | `0` | Размерность вектора (0 = авто) |
| `QDRANT_SEARCH_ORDER` | `upload,web,obsidian` | Порядок каскадного поиска по коллекциям |
| `CHUNK_SIZE` | `900` | Размер чанка (символы) |
| `CHUNK_OVERLAP` | `150` | Перекрытие чанков |
| `CHUNK_STRATEGY` | `fixed` | Стратегия: `fixed`, `markdown` |
| `CHUNK_CONFIG` | | Per-source JSON конфиг чанкинга |
| `RAG_TOP_K` | `5` | Топ-K результатов retrieval |
| `RAG_RETRIEVAL_MODE` | `semantic` | Режим: `semantic`, `hybrid` |
| `RAG_HYBRID_CANDIDATES` | `30` | Кандидатов для hybrid search |
| `RAG_FUSION_STRATEGY` | `rrf` | Стратегия fusion: `rrf` |
| `RAG_RERANK_TOP_N` | `20` | Топ-N для reranking |
| `QUERY_EXPANSION_ENABLED` | `false` | Включить multi-query expansion |

### Agent

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `AGENT_MODE_ENABLED` | `true` | Включить серверный agent loop |
| `AGENT_MAX_ITERATIONS` | `10` | Макс. итераций агента |
| `AGENT_TIMEOUT_SECONDS` | `90` | Общий таймаут агента |
| `AGENT_PLANNER_TIMEOUT_SECONDS` | `20` | Таймаут планировщика |
| `AGENT_TOOL_TIMEOUT_SECONDS` | `30` | Таймаут инструмента |
| `AGENT_SHORT_MEMORY_MESSAGES` | `12` | Кол-во последних сообщений в контексте |
| `AGENT_SUMMARY_EVERY_TURNS` | `6` | Суммаризация каждые N ходов |
| `AGENT_MEMORY_TOP_K` | `4` | Топ-K воспоминаний из долговременной памяти |
| `AGENT_KNOWLEDGE_TOP_K` | `5` | Топ-K результатов knowledge_search |
| `AGENT_INTENT_ROUTER_ENABLED` | `true` | Включить intent router |
| `OPENAI_COMPAT_API_KEY` | | Bearer-токен для API (пусто = без авторизации) |

### Knowledge Graph (Neo4j)

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `GRAPH_ENABLED` | `false` | Включить Neo4j knowledge graph |
| `NEO4J_URL` | `bolt://neo4j:7687` | URL Neo4j |
| `NEO4J_USER` | `neo4j` | Пользователь Neo4j |
| `NEO4J_PASSWORD` | `password` | Пароль Neo4j |
| `GRAPH_SIMILARITY_THRESHOLD` | `0.75` | Порог similarity для создания связей |
| `GRAPH_BOOST_FACTOR` | `0.7` | Коэффициент boost от графа при retrieval |

### Multi-Agent Orchestration

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `ORCHESTRATOR_ENABLED` | `false` | Включить multi-agent orchestration |
| `ORCHESTRATOR_MAX_STEPS` | `8` | Макс. шагов оркестратора |
| `AGENT_SPECS` | | JSON-массив кастомных агентов |
| `MODEL_ROUTING` | | JSON для adaptive model routing |

### Self-Improving Agent

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `SELF_IMPROVE_ENABLED` | `false` | Включить self-improving agent |
| `SELF_IMPROVE_INTERVAL_HOURS` | `24` | Интервал анализа (часы) |
| `SELF_IMPROVE_AUTO_APPLY` | `true` | Автоприменение safe improvements |

### Scheduled Tasks

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `SCHEDULER_ENABLED` | `false` | Включить cron-планировщик |
| `SCHEDULER_CHECK_INTERVAL_SECONDS` | `60` | Интервал проверки задач |

### HTTP Tools Plugin

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `HTTP_TOOLS` | | JSON-массив HTTP tool definitions |
| `HTTP_TOOLS_FILE` | | Путь к JSON-файлу с HTTP tools |

### Web Search

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `WEB_SEARCH_ENABLED` | `true` | Включить SearXNG веб-поиск |
| `WEB_SEARCH_URL` | `http://searxng:8080` | URL SearXNG |
| `WEB_SEARCH_LIMIT` | `5` | Макс. результатов поиска |

### MCP

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `MCP_SERVER_ENABLED` | `true` | Включить MCP-сервер (`/mcp`) |
| `MCP_SERVERS` | `[...]` | JSON-массив внешних MCP-серверов |

### Obsidian

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `OBSIDIAN_VAULTS_HOST_PATH` | `./obsidian_vaults` | Путь к vault-ам на хосте |
| `ASSISTANT_OBSIDIAN_DEFAULT_INTERVAL_MINUTES` | `15` | Интервал авто-синхронизации |

### Rate Limiting / Resilience

| Переменная | По умолчанию | Описание |
| ---------- | ------------ | -------- |
| `API_RATE_LIMIT_RPS` | `40` | Rate limit (запросов/сек), `<=0` отключает |
| `API_BACKPRESSURE_MAX_IN_FLIGHT` | `64` | Макс. одновременных запросов |
| `RESILIENCE_BREAKER_ENABLED` | `true` | Включить circuit breaker |
| `RESILIENCE_RETRY_MAX_ATTEMPTS` | `3` | Макс. попыток retry |

---

## API Endpoints

### OpenAI-compatible

| Метод | Путь | Описание |
|-------|------|----------|
| `POST` | `/v1/chat/completions` | Chat completion (streaming SSE) |
| `POST` | `/v1/documents` | Загрузка документа |
| `GET` | `/v1/documents` | Список документов |
| `GET` | `/v1/documents/{id}/content` | Контент документа |

### Obsidian Vaults

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/v1/obsidian/vaults` | Список vault-ов |
| `POST` | `/v1/obsidian/vaults` | Добавить vault |
| `POST` | `/v1/obsidian/vaults/{id}/sync` | Синхронизация vault |
| `POST` | `/v1/obsidian/vaults/{id}/notes` | Создать заметку |
| `GET` | `/v1/obsidian/vaults/{id}/files` | Список файлов |
| `GET` | `/v1/obsidian/vaults/{id}/files/content` | Контент файла |
| `GET` | `/v1/obsidian/find?filename=X` | Поиск файла по всем vault-ам |

### Knowledge Graph

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/v1/graph` | Граф узлов и связей |

### Scheduled Tasks

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/v1/schedules` | Список расписаний |
| `POST` | `/v1/schedules` | Создать расписание |
| `PATCH` | `/v1/schedules/{id}` | Обновить расписание |
| `DELETE` | `/v1/schedules/{id}` | Удалить расписание |

### Self-Improving Agent

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/v1/events/summary` | Агрегация событий агента |
| `POST` | `/v1/feedback` | Отправить feedback |
| `GET` | `/v1/feedback/summary` | Статистика feedback |
| `GET` | `/v1/improvements` | Pending improvements |
| `PATCH` | `/v1/improvements/{id}` | Approve/dismiss improvement |

### Tools & Monitoring

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/v1/tools` | Список HTTP tools |
| `GET` | `/healthz` | Health check |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/mcp` | MCP server endpoint |

---

## Мониторинг

```bash
docker compose up -d prometheus alertmanager grafana
```

| Сервис | URL | Описание |
| ------ | --- | ------ |
| API metrics | `http://localhost:8080/metrics` | Prometheus metrics |
| Prometheus | `http://localhost:9091` | Metric storage |
| Alertmanager | `http://localhost:9093` | Alert routing |
| Grafana | `http://localhost:3001` | Dashboards |

Grafana dashboards:
- **PAA Overview** — общие метрики API и worker
- **PAA RAGAS Evaluation** — faithfulness, relevancy, correctness, context precision

Конфигурация:
- Prometheus: `deploy/monitoring/prometheus/`
- Alertmanager: `deploy/monitoring/alertmanager/`
- Grafana dashboards: `deploy/monitoring/grafana/dashboards/`

```bash
make monitoring-validate
```

---

## Скриншоты

![UI](img/image.png)

---

## Лицензия

PolyForm Small Business License. См. файл [LICENSE](LICENSE).

---

## Участие в разработке

1. Форкните репозиторий
2. Создайте feature-ветку: `git checkout -b feature/my-feature`
3. Сделайте изменения и добавьте тесты
4. Убедитесь, что тесты проходят: `make test && make vet`
5. Создайте Pull Request

### Структура коммитов

Используйте [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`.

### Добавление нового use case

1. Определите inbound-интерфейс в `internal/core/ports/inbound.go`
2. Реализуйте use case в `internal/core/usecase/` через outbound-порты
3. Подключите в `internal/bootstrap/bootstrap.go`
4. Используйте inbound-интерфейс в адаптере (`internal/adapters/http/`)
5. Добавьте unit-тесты
