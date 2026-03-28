# Multi-Source Ready RAG

**Дата:** 2026-03-28
**Этап:** 2.6 — Multi-Source Ready RAG
**Статус:** approved

## Проблема

Все документы хранятся в одной Qdrant-коллекции без разделения по источнику. `SearchFilter` поддерживает только `Category`. Chunking глобальный — нельзя настроить стратегию под конкретный source_type. Добавление нового источника требует изменений в retrieval core.

## Решение

1. Расширить `SearchFilter` до полноценного контракта (source, tags, path, time).
2. Изолировать source_type'ы в отдельные Qdrant-коллекции с cascading search по приоритету.
3. Сделать chunking configurable per source_type.

## Секция 1: Расширенный SearchFilter

### Новый контракт

```go
type SearchFilter struct {
    SourceTypes []string  // фильтр по source_type (пусто = все)
    Categories  []string  // фильтр по category (пусто = все)
    Tags        []string  // any match (пусто = все)
    PathPrefix  string    // prefix match по path
}
```

### Правила

- Пустой фильтр = без ограничений (обратная совместимость).
- Несколько значений в `SourceTypes`/`Categories`/`Tags` — OR внутри поля, AND между полями.
- `PathPrefix` — строковый prefix match по `path` в payload.

### Qdrant mapping

- `SourceTypes` → `must: [{key: "source_type", match: {any: [...]}}]`
- `Categories` → `must: [{key: "category", match: {any: [...]}}]`
- `Tags` → `must: [{key: "tags", match: {any: [...]}}]`
- `PathPrefix` → prefix match на `path`

`buildCategoryFilter` заменяется на общий `buildFilter(filter SearchFilter)`.

## Секция 2: Multi-collection с изоляцией по source_type

### Коллекции

Каждый source_type — своя Qdrant-коллекция: `{base}_{source_type}`.
- `documents_upload`
- `documents_web`
- `documents_obsidian`

Base collection берётся из конфига `QDRANT_COLLECTION`.

### MultiCollectionVectorStore

```go
type MultiCollectionStore struct {
    baseURL        string
    baseCollection string
    clients        map[string]*Client  // source_type → Client
    searchOrder    []string            // приоритет cascading search
    options        Options
}
```

Реализует `ports.VectorStore`. Use case'ы не меняются.

### IndexChunks

Роутит в коллекцию по `doc.SourceType`:
```
doc.SourceType = "web" → clients["web"].IndexChunks(...)
```

### Search / SearchLexical — cascading

1. Итерируем `searchOrder` (например `["obsidian", "upload", "web"]`).
2. Если фильтр задаёт `SourceTypes` — ищем только в указанных коллекциях.
3. Для каждой коллекции: query с limit → собираем кандидатов.
4. Если набрали >= limit кандидатов — early stop.
5. Merge результатов по score.

### UpdateChunksPayload

Нужен source_type для роутинга. Добавляем `sourceType string` как параметр, или выводим из docID через Postgres.

Проще: расширить сигнатуру `UpdateChunksPayload(ctx, docID, sourceType, payload)`.

### Конфигурация

```env
QDRANT_SEARCH_ORDER=obsidian,upload,web
```

## Секция 3: Configurable chunking per source

### Конфигурация

```env
# Глобальный fallback
CHUNK_STRATEGY=fixed
CHUNK_SIZE=900
CHUNK_OVERLAP=100

# Per-source overrides (JSON)
CHUNK_CONFIG={"obsidian":{"strategy":"markdown","chunk_size":1200,"overlap":150},"web":{"strategy":"fixed","chunk_size":600,"overlap":50}}
```

Если `CHUNK_CONFIG` не задан — все source'ы используют глобальные настройки.

### ChunkerRegistry

Порт (интерфейс в `ports`):

```go
// ChunkerRegistry selects a Chunker based on source type.
type ChunkerRegistry interface {
    ForSource(sourceType string) Chunker
}
```

Реализация в `internal/infrastructure/chunking/`:

```go
type Registry struct {
    chunkers map[string]ports.Chunker
    fallback ports.Chunker
}

func (r *Registry) ForSource(sourceType string) ports.Chunker
```

### Интеграция с ProcessDocumentUseCase

Заменить `chunker ports.Chunker` на `chunkers ports.ChunkerRegistry`:

```go
chunker := uc.chunkers.ForSource(doc.SourceType)
chunks := chunker.Split(text)
```

## Изменения в портах

### VectorStore — расширенная сигнатура

```go
type VectorStore interface {
    IndexChunks(ctx context.Context, doc *domain.Document, chunks []string, vectors [][]float32) error
    Search(ctx context.Context, queryVector []float32, limit int, filter domain.SearchFilter) ([]domain.RetrievedChunk, error)
    SearchLexical(ctx context.Context, queryText string, limit int, filter domain.SearchFilter) ([]domain.RetrievedChunk, error)
    UpdateChunksPayload(ctx context.Context, docID string, sourceType string, payload map[string]any) error
}
```

Единственное изменение — `UpdateChunksPayload` получает `sourceType` для роутинга в нужную коллекцию.

## Что НЕ входит

- Новые chunking-стратегии (только existing fixed и markdown)
- Obsidian vault sync (отдельный spec)
- Новые API endpoints для source'ов
- Миграция существующих данных из старой коллекции
