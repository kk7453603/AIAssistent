# Personal AI Assistant (MVP)

MVP-сервис для:
- загрузки документов;
- автоматической классификации через Ollama;
- индексации чанков в Qdrant;
- использования контекста в RAG-запросах;
- OpenAI-compatible чата в OpenWebUI.
- server-side agent loop с memory/tool orchestration (feature-flag).

## Стек
- Go (`api` + `worker`)
- OpenWebUI (`latest`)
- Ollama
- Qdrant
- PostgreSQL
- NATS
- MinIO (подготовлен в compose; в MVP storage через общий volume)

## Архитектура
- Подробная карта бизнес-логики и пайплайнов: `docs/architecture.md`

## Быстрый старт
1. Скопировать переменные:
   - `cp .env.example .env`
2. Поднять стек:
   - `docker compose up -d --build`
3. Подгрузить модели в Ollama:
   - `docker compose exec ollama ollama pull llama3.1:8b`
   - `docker compose exec ollama ollama pull nomic-embed-text`
4. Открыть OpenWebUI:
   - `http://localhost:3000`

Примечание: хостовый порт Ollama в compose по умолчанию `11435`, чтобы не конфликтовать с локальным Ollama на `11434`.

### Отдельный вариант: GPU на хосте (Ollama вне Docker)
Базовый `docker-compose.yml` оставляет `ollama` в Docker (CPU).

Для режима, где Ollama работает на хосте с GPU, используйте отдельный файл переопределения:
- `docker compose -f docker-compose.yml -f docker-compose.host-gpu.yml up -d --build`

Что делает переопределение:
- заменяет сервис `ollama` в compose на легкий HTTP reverse-proxy (`nginx`);
- проксирует `ollama:11434` в хостовый Ollama (`HOST_OLLAMA_HOST:HOST_OLLAMA_PORT`);
- переписывает `Host` header в `localhost:11434`, чтобы избежать `403 Forbidden` от host Ollama;
- остальные сервисы (`api`, `worker`, `openwebui`) продолжают работать без изменений по `http://ollama:11434`.

Параметры в `.env`:
- `HOST_OLLAMA_HOST` (по умолчанию `host.docker.internal`)
- `HOST_OLLAMA_PORT` (по умолчанию `11434`)

Проверка:
- `docker compose -f docker-compose.yml -f docker-compose.host-gpu.yml logs ollama`
- `curl http://localhost:11435/api/version`

## Использование OpenWebUI
- OpenWebUI поднимается всегда вместе со стеком.
- OpenWebUI собирается из `deploy/openwebui/Dockerfile` (база `ghcr.io/open-webui/open-webui:latest`) с патчем auth-fallback для cookie.
- Модель вашего backend-сервиса доступна через OpenAI-compatible API (`/v1/models`, `/v1/chat/completions`).
- Дополнительно в UI доступны прямые Ollama-модели (`OLLAMA_BASE_URLS=http://ollama:11434`).
- При старте сервис `openwebui-tool-bootstrap` автоматически создаёт/обновляет кастомный инструмент `assistant_ingest_and_query`.

### Как использовать инструмент для документов
1. В чате OpenWebUI прикрепите файлы.
2. Сформулируйте запрос с явным намерением про загрузку/документ (например: `upload this file and summarize`).
3. Backend-сервис может вернуть `tool_calls`, после чего инструмент:
   - скачает вложения из OpenWebUI,
   - загрузит их в `POST /v1/documents`,
   - дождется `ready` по `GET /v1/documents/{id}`,
   - выполнит `POST /v1/rag/query`.

## API

OpenAPI JSON:
- `GET /openapi.json`

### 1) Модели OpenAI-compatible API
`GET /v1/models`

Пример (с опциональной авторизацией):
```bash
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer ${OPENAI_COMPAT_API_KEY}"
```

### 2) Чат OpenAI-compatible API
`POST /v1/chat/completions`

Пример JSON (без стриминга):
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${OPENAI_COMPAT_API_KEY}" \
  -d '{
    "model": "paa-rag-v1",
    "messages": [
      {"role": "user", "content": "О чем документ?"}
    ]
  }'
```

Пример agent-mode (server-side memory + tools):
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${OPENAI_COMPAT_API_KEY}" \
  -d '{
    "model": "paa-rag-v1",
    "messages": [
      {"role": "user", "content": "Добавь задачу купить молоко и напомни завтра"}
    ],
    "metadata": {
      "user_id": "demo-user-1",
      "conversation_id": "weekly-planning",
      "session_end": false
    }
  }'
```

Пример со стримингом:
```bash
curl -N -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${OPENAI_COMPAT_API_KEY}" \
  -d '{
    "model": "paa-rag-v1",
    "stream": true,
    "messages": [
      {"role": "user", "content": "Сделай summary"}
    ]
  }'
```

### 3) Загрузка документа
`POST /v1/documents` (multipart form-data, поле `file`)

Пример:
```bash
curl -X POST http://localhost:8080/v1/documents \
  -F "file=@./sample.txt"
```

### 4) Статус документа
`GET /v1/documents/{document_id}`

### 5) RAG-запрос
`POST /v1/rag/query`

Пример:
```bash
curl -X POST http://localhost:8080/v1/rag/query \
  -H "Content-Type: application/json" \
  -d '{
    "question": "О чем этот документ?",
    "limit": 5
  }'
```

## Observability

### Request ID + structured logs
- API и worker пишут JSON-логи.
- Для каждого HTTP-запроса выставляется `X-Request-Id`:
  - если клиент передал `X-Request-Id`, он сохраняется;
  - иначе backend генерирует UUID.

### Prometheus metrics
- API metrics: `GET /metrics` на `http://localhost:8080/metrics`
- Worker metrics: `GET /metrics` на `http://localhost:${WORKER_METRICS_PORT:-9090}/metrics`
- Worker health: `GET /healthz` на `http://localhost:${WORKER_METRICS_PORT:-9090}/healthz`
- API traffic controls:
  - rate limit возвращает `429 Too Many Requests` + заголовок `Retry-After`;
  - backpressure при перегрузке возвращает `503 Service Unavailable`.
- Resilience:
  - временные сбои Ollama/Qdrant/NATS ретраятся;
  - при деградации dependency сервис может вернуть `503 Service Unavailable` (через `domain.ErrTemporary`).

Примеры:
```bash
curl http://localhost:8080/metrics
curl http://localhost:9090/metrics
```

RAG mode-aware метрика:
- `paa_rag_mode_requests_total{service,endpoint,mode}`

### Dashboards + alerts
Локальный monitoring stack включает:
- Prometheus
- Alertmanager (UI-only receiver)
- Grafana с автоподгрузкой дашборда `PAA Overview`

Проверить конфигурацию:
```bash
make monitoring-validate
```

Поднять stack:
```bash
docker compose up -d --build prometheus alertmanager grafana
```

URL:
- Prometheus: `http://localhost:9091`
- Alertmanager: `http://localhost:9093`
- Grafana: `http://localhost:3001`

Проверки:
- В Prometheus на странице `Status -> Targets` должны быть `UP` цели:
  - `paa-api`
  - `paa-worker`
- В Grafana должен автоматически появиться dashboard `PAA Overview` в папке `Personal AI Assistant`.

Alert smoke:
1. Остановить API контейнер:
   - `docker compose stop api`
2. Подождать 2+ минуты и проверить alert `ApiDown` в Alertmanager/Grafana.
3. Вернуть API:
   - `docker compose start api`
4. Убедиться, что alert перешёл в `resolved`.

Troubleshooting:
- Если `paa-worker` в статусе `DOWN`, проверьте что worker metrics слушает `9090` внутри контейнера.
- Если dashboard не появился, проверьте логи Grafana:
  - `docker compose logs grafana`
- Если alert rules не подхватываются, проверьте:
  - `docker compose logs prometheus`
  - `make monitoring-validate`

## Demo-сценарии (Stage 4)

### 1) End-to-end ingestion -> RAG
1. Загрузить документ:
```bash
curl -X POST http://localhost:8080/v1/documents -F "file=@./sample.txt"
```
2. Дождаться `status=ready`:
```bash
curl http://localhost:8080/v1/documents/<document_id>
```
3. Выполнить RAG-запрос:
```bash
curl -X POST http://localhost:8080/v1/rag/query \
  -H "Content-Type: application/json" \
  -d '{"question":"Сделай summary документа","limit":5}'
```
Ожидаемый результат: осмысленный ответ + непустой `sources` (если контекст найден).

### 2) OpenAI-compatible chat + tool trigger (intent-based)
1. В OpenWebUI прикрепить файл.
2. Отправить запрос с явным намерением действия над вложением, например:
   - `upload this file and summarize key points`
   - `проанализируй вложенный документ и выдели риски`
3. Проверить, что backend возвращает `tool_calls`, а после tool-output генерируется финальный ответ.

Ожидаемый результат: tool ветка срабатывает на action+attachment intent и не срабатывает на чисто информационные вопросы (`what is document database?`).

### 3) Alert smoke (операционный демо)
1. Остановить API:
```bash
docker compose stop api
```
2. Через 2+ минуты проверить `ApiDown` в Alertmanager/Grafana.
3. Поднять API обратно:
```bash
docker compose start api
```
4. Убедиться, что alert переходит в `resolved`.

## Production runbook (локальная эксплуатация)

### SLO/основные сигналы
- API availability: target `up{job="paa-api"} == 1`.
- API errors: 5xx ratio < 5% (алерт `ApiHighErrorRate`).
- API latency: p95 < 2s (алерт `ApiLatencyP95High`).
- Worker queue lag: p95 < 60s (алерт `WorkerQueueLagP95High`).
- Worker processing errors: < 10% (алерт `WorkerProcessingErrorRateHigh`).

### Preflight перед запуском
1. Проверить env:
   - `cp .env.example .env` (если еще не создан).
   - убедиться, что заданы `OPENWEBUI_SECRET_KEY`, `GRAFANA_ADMIN_USER`, `GRAFANA_ADMIN_PASSWORD`.
2. Проверить monitoring config:
```bash
make monitoring-validate
docker compose config
```

### Запуск/остановка
- Полный запуск:
```bash
docker compose up -d --build
```
- Только мониторинг:
```bash
docker compose up -d prometheus alertmanager grafana
```
- Мягкая остановка:
```bash
docker compose stop
```

### Smoke-check после запуска
1. API health:
   - `curl -f http://localhost:8080/healthz`
2. Метрики:
   - `curl -f http://localhost:8080/metrics`
   - `curl -f http://localhost:9090/metrics`
3. Monitoring UI:
   - `http://localhost:9091` (Prometheus)
   - `http://localhost:9093` (Alertmanager)
   - `http://localhost:3001` (Grafana)
4. Prometheus targets:
   - `paa-api`, `paa-worker` должны быть `UP`.

### Runbook по инцидентам
1. `ApiDown` / `WorkerDown`:
   - проверить контейнеры: `docker compose ps`
   - проверить логи: `docker compose logs api worker --tail=200`
   - проверить доступность зависимостей: `postgres`, `nats`, `qdrant`, `ollama`
   - восстановить сервис: `docker compose restart api` или `docker compose restart worker`
2. `ApiHighErrorRate`:
   - проверить последние 5xx в логах API
   - проверить зависимость Ollama/Qdrant (часто причина 503/temporary failures)
   - при необходимости снизить входящий поток (`API_RATE_LIMIT_RPS`, `API_RATE_LIMIT_BURST`)
3. `ApiLatencyP95High`:
   - проверить нагрузку и `API_BACKPRESSURE_MAX_IN_FLIGHT`
   - оценить размер контекста/сложность запросов
   - проверить задержки Ollama/Qdrant
4. `WorkerQueueLagP95High`:
   - проверить скорость ingestion vs processing
   - добавить worker instance или уменьшить входящий поток ingestion
5. `WorkerProcessingErrorRateHigh`:
   - проверить тип ошибок в `worker` логах
   - проверить корректность моделей Ollama и доступность Qdrant

### Плановое восстановление после деградации
1. Убедиться, что внешние зависимости доступны.
2. Перезапустить деградировавшие сервисы.
3. Проверить, что alerts `resolved`.
4. Повторить functional smoke:
   - upload -> ready -> rag query.

## Retrieval evaluation

Скрипты:
- `scripts/eval/generate_cases_from_manifest.sh` — генерирует retrieval-кейсы (JSONL) из `manifest.csv`.
- `scripts/eval/run.sh` — считает `precision@k`, `recall@k`, `MRR@k` через `POST /v1/rag/query`.
- `scripts/eval/compare_modes.sh` — сравнивает отчеты `semantic`, `hybrid`, `hybrid+rerank` и считает дельты.

Примеры:
```bash
# 1) Сгенерировать кейсы из manifest (например, из tmp/rag-300)
make eval-cases \
  EVAL_MANIFEST=./tmp/rag-300/manifest.csv \
  EVAL_GENERATED_CASES=./tmp/eval/retrieval_cases.jsonl

# 2) Запустить оценку
make eval \
  EVAL_CASES=./tmp/eval/retrieval_cases.jsonl \
  EVAL_K=5 \
  EVAL_REPORT=./tmp/eval/report.json

# 3) Сравнить режимы retrieval
make eval-compare \
  EVAL_REPORT_SEMANTIC=./tmp/eval/report_semantic.json \
  EVAL_REPORT_HYBRID=./tmp/eval/report_hybrid.json \
  EVAL_REPORT_HYBRID_RERANK=./tmp/eval/report_hybrid_rerank.json \
  EVAL_COMPARE_OUT=./tmp/eval/modes_compare.json
```

Формат кейса (JSONL):
```json
{"id":"Q1","question":"What is the risk level for document doc_0001_support_ap-south.txt?","expected_filenames":["doc_0001_support_ap-south.txt"]}
```

Базовый пример отчета с метриками и latency snapshot:
- `docs/evaluation/baseline.md`
- Режимный бенчмарк hybrid retrieval:
  - `docs/evaluation/advanced-retrieval.md`

## Генерация тестовых данных для RAG
Скрипт: `scripts/rag/generate_test_data.sh`

Что создаёт:
- `documents/*.txt` (синтетические документы)
- `manifest.csv` (атрибуты документов)
- `sample_queries.txt` (общий набор тестовых вопросов, часть может не совпасть с конкретной случайной выборкой)
- `sample_queries_grounded.txt` (вопросы, гарантированно соответствующие данным из `manifest.csv`)
- `eval_cases.csv` (эталон: `question -> expected_answer` для grounded-вопросов)
- `upload_results.csv` (если включён `--upload`)

Примеры:
```bash
# 1) Сгенерировать 1000 документов локально
scripts/rag/generate_test_data.sh --count 1000 --out-dir ./tmp/rag-1k

# 2) Сгенерировать и загрузить в API, дождаться статусов ready/failed
scripts/rag/generate_test_data.sh \
  --count 300 \
  --out-dir ./tmp/rag-300-upload \
  --upload \
  --wait-ready \
  --api-url http://localhost:8080
```

## Переменные окружения

### Базовые
- `API_PORT`
- `LOG_LEVEL`
- `POSTGRES_DSN`
- `NATS_URL`
- `NATS_SUBJECT`
- `OLLAMA_URL`
- `OLLAMA_GEN_MODEL`
- `OLLAMA_EMBED_MODEL`
- `QDRANT_URL`
- `QDRANT_COLLECTION`
- `QDRANT_MEMORY_COLLECTION`
- `STORAGE_PATH`
- `CHUNK_SIZE`
- `CHUNK_OVERLAP`
- `RAG_TOP_K`
- `RAG_RETRIEVAL_MODE` (`semantic`, `hybrid`, `hybrid+rerank`)
- `RAG_HYBRID_CANDIDATES`
- `RAG_FUSION_STRATEGY` (сейчас поддерживается только `rrf`)
- `RAG_FUSION_RRF_K`
- `RAG_RERANK_TOP_N`
- `WORKER_METRICS_PORT`

### Слой OpenAI-compatible API
- `OPENAI_COMPAT_API_KEY` (опционально; если пусто, `/v1/models` и `/v1/chat/completions` работают без авторизации)
- `OPENAI_COMPAT_MODEL_ID` (по умолчанию: `paa-rag-v1`)
- `OPENAI_COMPAT_CONTEXT_MESSAGES` (по умолчанию: `5`)
- `OPENAI_COMPAT_STREAM_CHUNK_CHARS` (по умолчанию: `120`)
- `OPENAI_COMPAT_TOOL_TRIGGER_KEYWORDS` (CSV-список триггер-слов для ветки `tool_calls`)
- `AGENT_MODE_ENABLED` (по умолчанию: `false`)
- `AGENT_MAX_ITERATIONS` (по умолчанию: `6`)
- `AGENT_TIMEOUT_SECONDS` (по умолчанию: `25`)
- `AGENT_SHORT_MEMORY_MESSAGES` (по умолчанию: `12`)
- `AGENT_SUMMARY_EVERY_TURNS` (по умолчанию: `6`)
- `AGENT_MEMORY_TOP_K` (по умолчанию: `4`)
- `AGENT_KNOWLEDGE_TOP_K` (по умолчанию: `5`)
- `API_RATE_LIMIT_RPS` (по умолчанию: `40`, `<=0` отключает rate limit)
- `API_RATE_LIMIT_BURST` (по умолчанию: `80`)
- `API_BACKPRESSURE_MAX_IN_FLIGHT` (по умолчанию: `64`, `<=0` отключает backpressure)
- `API_BACKPRESSURE_WAIT_MS` (по умолчанию: `250`)
- `RESILIENCE_BREAKER_ENABLED` (по умолчанию: `true`)
- `RESILIENCE_RETRY_MAX_ATTEMPTS` (по умолчанию: `3`)
- `RESILIENCE_RETRY_INITIAL_BACKOFF_MS` (по умолчанию: `100`)
- `RESILIENCE_RETRY_MAX_BACKOFF_MS` (по умолчанию: `400`)
- `RESILIENCE_RETRY_MULTIPLIER` (по умолчанию: `2.0`)
- `RESILIENCE_BREAKER_MIN_REQUESTS` (по умолчанию: `10`)
- `RESILIENCE_BREAKER_FAILURE_RATIO` (по умолчанию: `0.5`)
- `RESILIENCE_BREAKER_OPEN_MS` (по умолчанию: `30000`)
- `RESILIENCE_BREAKER_HALF_OPEN_MAX_CALLS` (по умолчанию: `2`)
- `NATS_CONNECT_TIMEOUT_MS` (по умолчанию: `2000`)
- `NATS_RECONNECT_WAIT_MS` (по умолчанию: `2000`)
- `NATS_MAX_RECONNECTS` (по умолчанию: `60`)
- `NATS_RETRY_ON_FAILED_CONNECT` (по умолчанию: `true`)
- `GRAFANA_ADMIN_USER` (по умолчанию: `admin`)
- `GRAFANA_ADMIN_PASSWORD` (по умолчанию: `ChangeMeGrafana123!`)

### Инициализация OpenWebUI
- `OPENWEBUI_ADMIN_EMAIL`
- `OPENWEBUI_ADMIN_PASSWORD`
- `OPENWEBUI_ADMIN_NAME`
- `OPENWEBUI_SECRET_KEY` (зафиксируйте и не меняйте между рестартами)

## Процесс spec-first
Контракт API описывается в спецификации, затем из нее генерируется серверный код:

1. Обновить спецификацию:
   - `api/openapi/openapi.yaml`
2. Сгенерировать код:
   - `make generate-openapi`
3. Реализовать бизнес-логику сгенерированных методов в:
   - `internal/adapters/http/router.go` (`StrictServerInterface`)
4. Проверить:
   - `make test`

## Architecture decisions
- Inbound/outbound порты разделены:
  - inbound: `internal/core/ports/inbound.go`
  - outbound: `internal/core/ports/outbound.go`
- HTTP-адаптер зависит от inbound-контрактов (`DocumentIngestor`, `DocumentQueryService`, `DocumentReader`), а не от concrete use case типов.
- Use case слой не содержит transport-логики и прямого логирования.
- OpenAI-compatible слой разнесен по ответственности:
  - auth: `internal/adapters/http/openai_auth.go`
  - orchestration: `internal/adapters/http/openai_chat.go`
  - agent orchestration: `internal/adapters/http/openai_agent.go`
  - tool branch: `internal/adapters/http/openai_tooling.go`
  - SSE: `internal/adapters/http/openai_sse.go`
  - parsing/helpers: `internal/adapters/http/openai_message_parsing.go`, `internal/adapters/http/openai_response.go`
- Маппинг доменных ошибок в HTTP:
  - `internal/core/domain/errors.go`
  - `internal/adapters/http/error_mapping.go`

### Как добавлять новый use case
1. Определите inbound-интерфейс в `internal/core/ports/inbound.go` (если текущих недостаточно).
2. Реализуйте use case в `internal/core/usecase/*` через outbound-порты.
3. Подключите реализацию в `internal/bootstrap/bootstrap.go`.
4. Используйте inbound-интерфейс в адаптере (`internal/adapters/http/*`).
5. Добавьте unit-тесты use case и adapter-тесты на маппинг/контракты.

## Ограничения MVP
- Извлечение текста реализовано для UTF-8 текстовых файлов.
- Для PDF/DOCX/OCR нужен отдельный адаптер извлечения текста.
- `debug` в OpenAI-compatible ответе технический и может не рендериться в UI.

## Диагностика проблем
- Если документ получает статус `failed` с ошибкой вида `ollama generate status: 404 Not Found`, обычно не загружена модель в контейнер `ollama`.
- Проверить модели:
  - `docker compose exec ollama ollama list`
- Подгрузить модели:
  - `docker compose exec ollama ollama pull llama3.1:8b`
  - `docker compose exec ollama ollama pull nomic-embed-text`
- Если OpenWebUI не видит backend-модель, проверьте:
  - `OPENAI_API_BASE_URLS=http://api:8080/v1` в сервисе `openwebui`
  - `GET http://localhost:8080/v1/models` локально
- Если `openwebui-tool-bootstrap` не создает инструмент:
  - убедитесь, что корректны `OPENWEBUI_ADMIN_*` в `.env`
  - посмотрите логи `docker compose logs openwebui-tool-bootstrap`
- Если после успешного логина в OpenWebUI чаты/папки/модели не грузятся и в логах много `401` на `/api/v1/...`:
  - очистите данные сайта для `localhost:3000` (cookies + localStorage) и войдите заново;
  - убедитесь, что в `.env` задан стабильный `OPENWEBUI_SECRET_KEY` (иначе старые токены становятся невалидными);
  - пересоберите и поднимите сервис заново: `docker compose up -d --build openwebui`.
- Если `401 Unauthorized` на `/v1/chat/completions`:
  - либо задайте `Authorization: Bearer <OPENAI_COMPAT_API_KEY>`,
  - либо очистите `OPENAI_COMPAT_API_KEY` для отключения auth.
- Если stream не работает в клиенте:
  - проверьте, что клиент отправляет `"stream": true` и умеет читать SSE (`text/event-stream`).
