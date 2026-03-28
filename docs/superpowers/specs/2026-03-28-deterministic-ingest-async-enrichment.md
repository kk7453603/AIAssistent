# Deterministic Ingest + Async LLM Enrichment

**Дата:** 2026-03-28
**Этап:** 2.5 — Stabilize Ingest + Multilingual Rerank (задача 1 из 4)
**Статус:** approved

## Проблема

LLM-классификация в ingest pipeline — точка отказа. Если LLM возвращает невалидный JSON или таймаутит, документ переходит в `failed` и не попадает в поиск. Это критично: ingest должен быть надёжным и предсказуемым.

## Решение

Разделить pipeline на два этапа:

1. **Синхронный ingest** — детерминированное извлечение метаданных + chunk + embed + index → `ready`. Документ доступен для поиска сразу.
2. **Async enrichment** — LLM-классификация через отдельную NATS-очередь. Дополняет метаданные без блокировки. Ошибка enrichment не влияет на доступность документа.

## Pipeline Flow

### Текущий (проблемный)

```
upload → NATS(ingest) → worker: extract → classify(LLM) → chunk → embed → index → ready/failed
```

### Новый

```
upload → NATS(ingest) → worker: extract → deterministic_meta → chunk → embed → index → ready
                                  ↓ (после index)
                            NATS(enrich) → enrichment: LLM classify → merge → update Postgres + Qdrant
```

## Deterministic Metadata Extractor

### Интерфейс

```go
// MetadataExtractor извлекает метаданные детерминированно, без LLM.
type MetadataExtractor interface {
    ExtractMetadata(ctx context.Context, doc *domain.Document, text string) (domain.DocumentMetadata, error)
}
```

### Стратегия извлечения (по приоритету)

1. **Markdown frontmatter** — YAML между `---\n...\n---`:
   ```yaml
   ---
   category: programming
   tags: [go, patterns]
   title: Clean Architecture in Go
   ---
   ```

2. **Путь/имя файла**:
   - Расширение → `source_type` (md, txt, etc.)
   - Директория → `category` hint
   - Имя → `title` fallback

3. **MIME-type** → `source_type` fallback

4. **Summary** — первые 200 символов текста (до `\n\n` или лимит)

### Результирующая структура

```go
type DocumentMetadata struct {
    SourceType string   // "markdown", "text", "unknown"
    Category   string   // из frontmatter или пути
    Tags       []string // из frontmatter
    Title      string   // из frontmatter, H1, или filename
    Summary    string   // первые N символов
    Headers    []string // H1-H3 заголовки из markdown
    Path       string   // оригинальный путь (для multi-source)
}
```

Frontmatter-парсинг — только для markdown (по расширению или MIME). Для остальных — filename + truncated summary.

## Async LLM Enrichment

### NATS subject

`documents.enrich` — отдельный subject от `documents.ingest`.

### Enrichment flow

1. Читает документ из Postgres
2. Извлекает текст (существующий `TextExtractor`)
3. LLM classify (существующий `DocumentClassifier`)
4. Merge с детерминированными данными
5. Update Postgres
6. Update Qdrant payload для всех chunk'ов документа

### Merge-стратегия

| Поле       | Правило                                    |
|------------|--------------------------------------------|
| category   | `deterministic \|\| llm` (frontmatter приоритетнее) |
| tags       | `union(deterministic, llm)` (объединяем)   |
| summary    | `llm \|\| deterministic` (LLM-summary лучше)  |
| confidence | `llm` (только LLM даёт score)             |

### Обработка ошибок

- LLM таймаут / невалидный JSON → log warning, документ остаётся с детерминированными метаданными
- Qdrant update fail → retry через resilience executor, при исчерпании — log error
- Никаких `StatusFailed` из enrichment — документ уже `ready`

## Изменения в существующем коде

### `ProcessDocumentUseCase`

- Убрать `classifier` из зависимостей
- Добавить `metadataExtractor` (`MetadataExtractor`)
- Добавить `queue` для публикации в `documents.enrich` после успешной индексации
- `applyClassification` → `applyMetadata` (работает с `DocumentMetadata`)
- Pipeline: `load → extract text → extract metadata → chunk → embed → index → publish enrich → ready`

### Новый `EnrichDocumentUseCase`

- Зависимости: `repo`, `extractor` (plaintext), `classifier` (LLM), `vectorDB`
- Метод `EnrichByID(ctx, documentID)`: load → extract text → LLM classify → merge → update Postgres → update Qdrant payload

### `cmd/worker`

Два подписчика:
- `SubscribeDocumentIngested` → `ProcessByID` (без LLM)
- `SubscribeDocumentEnrich` → `EnrichByID` (новый)

### Новые методы в портах

```go
// VectorStore
UpdateChunksPayload(ctx context.Context, docID string, payload map[string]any) error

// MessageQueue
PublishDocumentEnrich(ctx context.Context, documentID string) error
SubscribeDocumentEnrich(ctx context.Context, handler func(context.Context, string) error) error
```

### `domain.Document` — новые поля

```go
SourceType string   `json:"source_type"`
Title      string   `json:"title"`
Headers    []string `json:"headers,omitempty"`
Path       string   `json:"path"`
```

Существующие `Category`, `Tags`, `Summary`, `Confidence` остаются.

### Qdrant payload — расширение

При `IndexChunks` добавляем: `source_type`, `title`, `path`, `tags`.
`UpdateChunksPayload` — `set_payload` по фильтру `doc_id`.

### Миграция БД

`002_add_metadata.sql`:
```sql
ALTER TABLE documents ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS headers JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS path TEXT NOT NULL DEFAULT '';
```

## Что НЕ входит в эту задачу

- Унификация токенизации (задача 2 этапа 2.5)
- `SourceAdapter`/`Ingestor` интерфейс (задача 4 этапа 2.5)
- Multi-source namespace isolation (этап 2.6)
- PDF/DOCX extraction (этап 6+)
