# PDF/DOCX/XLSX/CSV Document Extraction

**Дата:** 2026-03-28
**Этап:** 6 — Уровень 2.1: Качество и память
**Статус:** approved

## Проблема

Текущий extractor (`plaintext.Extractor`) поддерживает только UTF-8 текстовые файлы. Бинарные форматы (PDF, DOCX, XLSX) отклоняются с ошибкой. Это ограничивает RAG — большинство документов в реальном мире в PDF/Office форматах.

## Решение

Native Go extractors для PDF, DOCX, XLSX, CSV. `ExtractorRegistry` выбирает реализацию по MIME-type. Без внешних сервисов (нет Tika/Docker зависимости).

## ExtractorRegistry

### Порт

```go
type ExtractorRegistry interface {
    ForMimeType(mimeType string) TextExtractor
}
```

### Реализация

```go
type Registry struct {
    extractors map[string]ports.TextExtractor
    fallback   ports.TextExtractor
}

func (r *Registry) Register(mimeType string, ext ports.TextExtractor)
func (r *Registry) ForMimeType(mimeType string) ports.TextExtractor
```

### MIME routing

| MIME-type | Extractor |
|-----------|-----------|
| `application/pdf` | `pdf.Extractor` |
| `application/vnd.openxmlformats-officedocument.wordprocessingml.document` | `docx.Extractor` |
| `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` | `spreadsheet.Extractor` |
| `text/csv` | `spreadsheet.Extractor` |
| всё остальное | `plaintext.Extractor` (fallback) |

## Extractors

### PDF Extractor

Go библиотека: `ledongthuc/pdf` (или `dslipak/pdf`).
Извлекает text layer постранично. Объединяет страницы с разделителем `\n\n`.
Если text layer пустой (scanned PDF) — возвращает ошибку (OCR out of scope).

### DOCX Extractor

Подход: raw XML unzip (без тяжёлых зависимостей).
Открывает DOCX как ZIP → читает `word/document.xml` → парсит `<w:t>` элементы → собирает текст.
Параграфы разделяются `\n`, run'ы внутри параграфа — пробелом.

### Spreadsheet Extractor

XLSX: Go библиотека `xuri/excelize`.
CSV: `encoding/csv` (stdlib).
Итерирует sheets → rows → cells. Формат вывода: `Sheet: <name>\n<row1_col1>\t<row1_col2>\n...`

## Файловая структура

```
internal/infrastructure/extractor/
├── plaintext/extractor.go        # существующий, без изменений
├── pdf/extractor.go
├── pdf/extractor_test.go
├── docx/extractor.go
├── docx/extractor_test.go
├── spreadsheet/extractor.go      # XLSX + CSV
├── spreadsheet/extractor_test.go
├── registry.go
└── registry_test.go
```

## Интеграция

### ProcessDocumentUseCase

Заменить `extractor ports.TextExtractor` на `extractors ports.ExtractorRegistry`.
В `extractText`: `extractor := uc.extractors.ForMimeType(doc.MimeType)`.

### EnrichDocumentUseCase

Аналогично заменить `extractor ports.TextExtractor` на `extractors ports.ExtractorRegistry`.

### Bootstrap

```go
extractorRegistry := extractor.NewRegistry(plaintextExt)
extractorRegistry.Register("application/pdf", pdfExt)
extractorRegistry.Register("application/vnd.openxmlformats-officedocument.wordprocessingml.document", docxExt)
extractorRegistry.Register("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", spreadsheetExt)
extractorRegistry.Register("text/csv", spreadsheetExt)
```

## Что НЕ входит

- OCR для scanned PDF (нет native Go OCR без C dependencies)
- PPTX, ODT (можно добавить позже по тому же паттерну)
- Предпросмотр документов в UI
- Apache Tika или другие внешние сервисы
