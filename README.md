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

## API

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

## Ограничения MVP
- Извлечение текста реализовано для UTF-8 текстовых файлов.
- Для PDF/DOCX/OCR нужен отдельный extractor-адаптер.
