# Personal AI Assistant (MVP)

MVP-сервис для:
- загрузки документов;
- автоматической классификации через Ollama;
- индексации чанков в Qdrant;
- использования контекста в RAG-запросах;
- OpenAI-compatible чата в OpenWebUI.

## Стек
- Go (`api` + `worker`)
- OpenWebUI (`latest`)
- Ollama
- Qdrant
- PostgreSQL
- NATS
- MinIO (подготовлен в compose; в MVP storage через общий volume)

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

## OpenWebUI usage
- OpenWebUI поднимается всегда вместе со стеком.
- Модель вашего backend доступна через OpenAI-compatible API (`/v1/models`, `/v1/chat/completions`).
- Дополнительно в UI доступны прямые Ollama-модели (`OLLAMA_BASE_URLS=http://ollama:11434`).
- При старте сервис `openwebui-tool-bootstrap` автоматически создаёт/обновляет custom tool `assistant_ingest_and_query`.

### Как использовать tool для документов
1. В чате OpenWebUI прикрепите файлы.
2. Сформулируйте запрос с явным intent про загрузку/документ (например: `upload this file and summarize`).
3. Backend может вернуть `tool_calls`, после чего tool:
   - скачает вложения из OpenWebUI,
   - загрузит их в `POST /v1/documents`,
   - дождется `ready` по `GET /v1/documents/{id}`,
   - выполнит `POST /v1/rag/query`.

## API

OpenAPI JSON:
- `GET /openapi.json`

### 1) OpenAI-compatible модели
`GET /v1/models`

Пример (с optional auth):
```bash
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer ${OPENAI_COMPAT_API_KEY}"
```

### 2) OpenAI-compatible чат
`POST /v1/chat/completions`

Пример JSON (non-stream):
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

Пример stream:
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

## Переменные окружения

### Core
- `API_PORT`
- `POSTGRES_DSN`
- `NATS_URL`
- `NATS_SUBJECT`
- `OLLAMA_URL`
- `OLLAMA_GEN_MODEL`
- `OLLAMA_EMBED_MODEL`
- `QDRANT_URL`
- `QDRANT_COLLECTION`
- `STORAGE_PATH`
- `CHUNK_SIZE`
- `CHUNK_OVERLAP`
- `RAG_TOP_K`

### OpenAI-compatible слой
- `OPENAI_COMPAT_API_KEY` (optional; если пусто, `/v1/models` и `/v1/chat/completions` без auth)
- `OPENAI_COMPAT_MODEL_ID` (default: `paa-rag-v1`)
- `OPENAI_COMPAT_CONTEXT_MESSAGES` (default: `5`)
- `OPENAI_COMPAT_STREAM_CHUNK_CHARS` (default: `120`)
- `OPENAI_COMPAT_TOOL_TRIGGER_KEYWORDS` (CSV-список триггер-слов для ветки `tool_calls`)

### OpenWebUI bootstrap
- `OPENWEBUI_ADMIN_EMAIL`
- `OPENWEBUI_ADMIN_PASSWORD`
- `OPENWEBUI_ADMIN_NAME`

## Spec-first workflow
Контракт API описывается в спецификации, затем из нее генерируется серверный код:

1. Обновить спецификацию:
   - `api/openapi/openapi.yaml`
2. Сгенерировать код:
   - `make generate-openapi`
3. Реализовать бизнес-логику сгенерированных методов в:
   - `internal/adapters/http/router.go` (`StrictServerInterface`)
4. Проверить:
   - `make test`

## Ограничения MVP
- Извлечение текста реализовано для UTF-8 текстовых файлов.
- Для PDF/DOCX/OCR нужен отдельный extractor-адаптер.
- `debug` в OpenAI-compatible ответе технический и может не рендериться в UI.

## Troubleshooting
- Если документ получает статус `failed` с ошибкой вида `ollama generate status: 404 Not Found`, обычно не загружена модель в контейнер `ollama`.
- Проверить модели:
  - `docker compose exec ollama ollama list`
- Подгрузить модели:
  - `docker compose exec ollama ollama pull llama3.1:8b`
  - `docker compose exec ollama ollama pull nomic-embed-text`
- Если OpenWebUI не видит backend-модель, проверьте:
  - `OPENAI_API_BASE_URLS=http://api:8080/v1` в сервисе `openwebui`
  - `GET http://localhost:8080/v1/models` локально
- Если `openwebui-tool-bootstrap` не создает tool:
  - убедитесь, что корректны `OPENWEBUI_ADMIN_*` в `.env`
  - посмотрите логи `docker compose logs openwebui-tool-bootstrap`
- Если `401 Unauthorized` на `/v1/chat/completions`:
  - либо задайте `Authorization: Bearer <OPENAI_COMPAT_API_KEY>`,
  - либо очистите `OPENAI_COMPAT_API_KEY` для отключения auth.
- Если stream не работает в клиенте:
  - проверьте, что клиент отправляет `"stream": true` и умеет читать SSE (`text/event-stream`).
