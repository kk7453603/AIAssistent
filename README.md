# Personal AI Assistant (MVP)

MVP-сервис для:
- загрузки документов;
- автоматической классификации через Ollama;
- индексации чанков в Qdrant;
- использования контекста в RAG-запросах.

## Стек
- Go (`api` + `worker`)
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

Примечание: хостовый порт Ollama в compose по умолчанию `11435`, чтобы не конфликтовать с локальным Ollama на `11434`.

## API

OpenAPI JSON:
- `GET /openapi.json`

### 1) Загрузка документа
`POST /v1/documents` (multipart form-data, поле `file`)

Пример:
```bash
curl -X POST http://localhost:8080/v1/documents \
  -F "file=@./sample.txt"
```

### 2) Статус документа
`GET /v1/documents/{document_id}`

### 3) RAG-запрос
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

## Troubleshooting
- Если документ получает статус `failed` с ошибкой вида `ollama generate status: 404 Not Found`, это обычно значит, что модель не загружена в контейнер `ollama`.
- Проверить модели:
  - `docker compose exec ollama ollama list`
- Подгрузить модели:
  - `docker compose exec ollama ollama pull llama3.1:8b`
  - `docker compose exec ollama ollama pull nomic-embed-text`
