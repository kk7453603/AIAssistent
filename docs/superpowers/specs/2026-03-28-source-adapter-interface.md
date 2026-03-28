# SourceAdapter/Ingestor Interface

**Дата:** 2026-03-28
**Этап:** 2.5 — задача 4: ввести SourceAdapter/Ingestor интерфейс
**Статус:** approved

## Проблема

Ingest pipeline жёстко привязан к HTTP upload (`IngestDocumentUseCase.Upload(filename, mimeType, body)`). Нет абстракции для подключения других источников — Obsidian vault sync, web scraping, файловая система.

## Решение

Ввести `SourceAdapter` интерфейс, который нормализует контент из любого source'а в единый `IngestResult`. Рефакторить `IngestDocumentUseCase` для работы через адаптеры. Реализовать `UploadAdapter` (рефакторинг) и `WebAdapter` (новый). `ObsidianAdapter` — заглушка.

## Модель инициации (смешанная)

Разные source'ы имеют разную природу запуска:
- **HTTP upload** — синхронный push от пользователя
- **Web scraping** — асинхронный запрос через API
- **Obsidian vault sync** — периодический pull (cron/watcher, будущий scope)

Общая точка схождения — `SourceAdapter.Ingest()` нормализует результат в `IngestResult`. После этого стандартный pipeline: storage → Document → NATS → process → enrich.

## SourceAdapter интерфейс

### Порт

```go
// SourceAdapter normalizes content from any source into an ingestable document.
type SourceAdapter interface {
    Ingest(ctx context.Context, req domain.SourceRequest) (*domain.IngestResult, error)
    SourceType() string
}
```

### Domain-типы

```go
type SourceRequest struct {
    SourceType string            // "upload", "obsidian", "web"
    Filename   string            // оригинальное имя / путь
    MimeType   string            // MIME-тип (если известен)
    Body       io.Reader         // контент (для upload/file)
    URL        string            // URL (для web scraping)
    VaultID    string            // vault ID (для Obsidian)
    Path       string            // путь в vault/fs
    Meta       map[string]string // произвольные метаданные от source
}

type IngestResult struct {
    Filename   string
    MimeType   string
    Body       io.Reader
    SourceType string
    Path       string
    ExtraMeta  map[string]string
}
```

## Рефакторинг IngestDocumentUseCase

### Новая структура

```go
type IngestDocumentUseCase struct {
    repo     ports.DocumentRepository
    storage  ports.ObjectStorage
    queue    ports.MessageQueue
    adapters map[string]ports.SourceAdapter
}
```

### Новый метод `IngestFromSource`

1. Выбрать adapter по `req.SourceType` из map
2. `adapter.Ingest(ctx, req)` → `IngestResult`
3. Сохранить `result.Body` в ObjectStorage
4. Создать `domain.Document` с `SourceType`, `Path`, метаданными из result
5. Опубликовать в NATS

### Обратная совместимость

`Upload()` остаётся как обёртка:

```go
func (uc *IngestDocumentUseCase) Upload(ctx context.Context, filename, mimeType string, body io.Reader) (*domain.Document, error) {
    return uc.IngestFromSource(ctx, domain.SourceRequest{
        SourceType: "upload",
        Filename:   filename,
        MimeType:   mimeType,
        Body:       body,
    })
}
```

HTTP handler не меняется.

### Inbound port

```go
type DocumentIngestor interface {
    Upload(ctx context.Context, filename, mimeType string, body io.Reader) (*domain.Document, error)
    IngestFromSource(ctx context.Context, req domain.SourceRequest) (*domain.Document, error)
}
```

## Реализации SourceAdapter

### UploadAdapter

Минимальная обёртка — прокидывает filename/mime/body. Валидирует что `Body != nil`.

### WebAdapter (реальная реализация)

1. Валидирует `req.URL`
2. HTTP GET URL
3. Определяет контент: если markdown (по расширению или Content-Type) — пропускает как есть; иначе — извлекает текст из HTML
4. HTML → text: strip tags, извлечь `<title>`, нормализовать whitespace. Используем `golang.org/x/net/html` (стандартная).
5. Возвращает `IngestResult`:
   - `filename` из `<title>` или last path segment URL
   - `mime_type` из response Content-Type
   - `source_type` = `"web"`
   - `path` = полный URL

### ObsidianAdapter (stub)

Возвращает `errors.New("obsidian source adapter not implemented")`. Реализация в отдельном spec при подключении vault sync.

## Файловая структура

```
internal/infrastructure/source/
├── upload/adapter.go
├── web/adapter.go
├── web/adapter_test.go
├── web/html.go
├── web/html_test.go
└── obsidian/adapter.go    # stub
```

## Bootstrap wiring

```go
adapters := map[string]ports.SourceAdapter{
    "upload":   upload.New(),
    "obsidian": obsidian.New(),
    "web":      web.New(),
}
ingestUC := usecase.NewIngestDocumentUseCase(repo, storage, queue, adapters)
```

## Что НЕ входит

- Obsidian vault sync (cron/watcher, incremental updates) — отдельный spec
- Новые API endpoints для source'ов (POST /v1/ingest) — будущий scope
- Namespace/tenant isolation в Qdrant (этап 2.6)
