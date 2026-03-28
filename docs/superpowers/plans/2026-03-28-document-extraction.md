# PDF/DOCX/XLSX/CSV Document Extraction — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add native Go extractors for PDF, DOCX, XLSX, CSV with an ExtractorRegistry that routes by MIME-type, enabling RAG on binary document formats.

**Architecture:** `ExtractorRegistry` selects a `TextExtractor` by MIME-type. Each format has its own extractor package. `ProcessDocumentUseCase` and `EnrichDocumentUseCase` receive `ExtractorRegistry` instead of a single `TextExtractor`. Existing `plaintext.Extractor` becomes the fallback.

**Tech Stack:** Go, `ledongthuc/pdf` (PDF), `archive/zip` + `encoding/xml` (DOCX), `xuri/excelize` (XLSX), `encoding/csv` (CSV).

**Spec:** `docs/superpowers/specs/2026-03-28-document-extraction.md`

---

### Task 1: Add ExtractorRegistry port

**Files:**
- Modify: `internal/core/ports/outbound.go`

- [ ] **Step 1: Add ExtractorRegistry interface**

In `internal/core/ports/outbound.go`, add after `TextExtractor`:

```go
// ExtractorRegistry selects a TextExtractor based on MIME type.
type ExtractorRegistry interface {
	ForMimeType(mimeType string) TextExtractor
}
```

- [ ] **Step 2: Run vet**

Run: `go vet ./internal/core/ports/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/core/ports/outbound.go
git commit -m "feat(ports): add ExtractorRegistry interface"
```

---

### Task 2: Implement ExtractorRegistry

**Files:**
- Create: `internal/infrastructure/extractor/registry.go`
- Create: `internal/infrastructure/extractor/registry_test.go`

- [ ] **Step 1: Write test**

Create `internal/infrastructure/extractor/registry_test.go`:

```go
package extractor

import (
	"context"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type fakeExtractor struct {
	text string
}

func (f *fakeExtractor) Extract(context.Context, *domain.Document) (string, error) {
	return f.text, nil
}

func TestRegistry_ForMimeType_Registered(t *testing.T) {
	fallback := &fakeExtractor{text: "fallback"}
	pdfExt := &fakeExtractor{text: "pdf-text"}

	reg := NewRegistry(fallback)
	reg.Register("application/pdf", pdfExt)

	got := reg.ForMimeType("application/pdf")
	text, _ := got.Extract(context.Background(), &domain.Document{})
	if text != "pdf-text" {
		t.Errorf("expected pdf-text, got %q", text)
	}
}

func TestRegistry_ForMimeType_Fallback(t *testing.T) {
	fallback := &fakeExtractor{text: "fallback"}
	reg := NewRegistry(fallback)

	got := reg.ForMimeType("application/unknown")
	text, _ := got.Extract(context.Background(), &domain.Document{})
	if text != "fallback" {
		t.Errorf("expected fallback, got %q", text)
	}
}

func TestRegistry_ForMimeType_WithCharset(t *testing.T) {
	fallback := &fakeExtractor{text: "fallback"}
	pdfExt := &fakeExtractor{text: "pdf-text"}

	reg := NewRegistry(fallback)
	reg.Register("application/pdf", pdfExt)

	// MIME type may include charset: "application/pdf; charset=utf-8"
	got := reg.ForMimeType("application/pdf; charset=utf-8")
	text, _ := got.Extract(context.Background(), &domain.Document{})
	if text != "pdf-text" {
		t.Errorf("expected pdf-text with charset suffix, got %q", text)
	}
}

var _ ports.ExtractorRegistry = (*Registry)(nil)
```

Run: `go test ./internal/infrastructure/extractor/ -v`
Expected: FAIL

- [ ] **Step 2: Implement Registry**

Create `internal/infrastructure/extractor/registry.go`:

```go
package extractor

import (
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// Registry selects a TextExtractor based on MIME type.
type Registry struct {
	extractors map[string]ports.TextExtractor
	fallback   ports.TextExtractor
}

func NewRegistry(fallback ports.TextExtractor) *Registry {
	return &Registry{
		extractors: make(map[string]ports.TextExtractor),
		fallback:   fallback,
	}
}

func (r *Registry) Register(mimeType string, ext ports.TextExtractor) {
	r.extractors[mimeType] = ext
}

func (r *Registry) ForMimeType(mimeType string) ports.TextExtractor {
	// Strip charset suffix: "application/pdf; charset=utf-8" → "application/pdf"
	base := strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])
	if ext, ok := r.extractors[base]; ok {
		return ext
	}
	return r.fallback
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/infrastructure/extractor/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/extractor/registry.go internal/infrastructure/extractor/registry_test.go
git commit -m "feat(extractor): implement ExtractorRegistry with MIME-type routing"
```

---

### Task 3: Implement PDF Extractor

**Files:**
- Create: `internal/infrastructure/extractor/pdf/extractor.go`
- Create: `internal/infrastructure/extractor/pdf/extractor_test.go`
- Create: `internal/infrastructure/extractor/pdf/testdata/sample.pdf` (test fixture)

- [ ] **Step 1: Add dependency**

Run: `go get github.com/ledongthuc/pdf`

- [ ] **Step 2: Write test**

Create `internal/infrastructure/extractor/pdf/extractor_test.go`:

```go
package pdf

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type storageFake struct {
	data []byte
}

func (f *storageFake) Save(context.Context, string, io.Reader) error { return nil }
func (f *storageFake) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(f.data))), nil
}

func TestPDFExtractor_Extract(t *testing.T) {
	// Create a minimal test: read a real PDF from testdata if available,
	// otherwise skip.
	pdfData, err := os.ReadFile("testdata/sample.pdf")
	if err != nil {
		t.Skip("testdata/sample.pdf not found — generate with: echo 'Hello PDF' | enscript -p - | ps2pdf - testdata/sample.pdf")
	}

	storage := &storageFake{data: pdfData}
	ext := NewExtractor(storage)

	doc := &domain.Document{
		StoragePath: "test.pdf",
		Filename:    "test.pdf",
		MimeType:    "application/pdf",
	}

	text, err := ext.Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if text == "" {
		t.Error("expected non-empty text from PDF")
	}
}

func TestPDFExtractor_EmptyStorage(t *testing.T) {
	storage := &storageFake{data: []byte{}}
	ext := NewExtractor(storage)

	doc := &domain.Document{StoragePath: "empty.pdf", Filename: "empty.pdf"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Error("expected error for empty/invalid PDF")
	}
}
```

- [ ] **Step 3: Implement PDF Extractor**

Create `internal/infrastructure/extractor/pdf/extractor.go`:

```go
package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	lpdf "github.com/ledongthuc/pdf"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type Extractor struct {
	storage ports.ObjectStorage
}

func NewExtractor(storage ports.ObjectStorage) *Extractor {
	return &Extractor{storage: storage}
}

func (e *Extractor) Extract(ctx context.Context, doc *domain.Document) (string, error) {
	reader, err := e.storage.Open(ctx, doc.StoragePath)
	if err != nil {
		return "", fmt.Errorf("open pdf document: %w", err)
	}
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read pdf document: %w", err)
	}

	if len(raw) == 0 {
		return "", fmt.Errorf("empty pdf file: %s", doc.Filename)
	}

	r, err := lpdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("parse pdf: %w", err)
	}

	var pages []string
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		t := strings.TrimSpace(text)
		if t != "" {
			pages = append(pages, t)
		}
	}

	if len(pages) == 0 {
		return "", fmt.Errorf("no text content in pdf: %s (might be scanned/image-only)", doc.Filename)
	}

	return strings.Join(pages, "\n\n"), nil
}
```

- [ ] **Step 4: Create test PDF fixture**

Run: `mkdir -p internal/infrastructure/extractor/pdf/testdata`

Create a minimal PDF for testing (if `enscript` is available):
```bash
echo "Hello World. This is a test PDF document." | enscript -B -p - 2>/dev/null | ps2pdf - internal/infrastructure/extractor/pdf/testdata/sample.pdf 2>/dev/null || echo "Skip PDF fixture — install enscript+ghostscript or create manually"
```

If tools unavailable, the test will skip gracefully.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/infrastructure/extractor/pdf/ -v`
Expected: PASS (or SKIP for sample.pdf test if fixture not created)

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/extractor/pdf/
git commit -m "feat(extractor/pdf): PDF text extraction via ledongthuc/pdf"
```

---

### Task 4: Implement DOCX Extractor

**Files:**
- Create: `internal/infrastructure/extractor/docx/extractor.go`
- Create: `internal/infrastructure/extractor/docx/extractor_test.go`

- [ ] **Step 1: Write test**

Create `internal/infrastructure/extractor/docx/extractor_test.go`:

```go
package docx

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type storageFake struct {
	data []byte
}

func (f *storageFake) Save(context.Context, string, io.Reader) error { return nil }
func (f *storageFake) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(f.data))), nil
}

func TestDOCXExtractor_Extract(t *testing.T) {
	docxData, err := os.ReadFile("testdata/sample.docx")
	if err != nil {
		t.Skip("testdata/sample.docx not found")
	}

	storage := &storageFake{data: docxData}
	ext := NewExtractor(storage)

	doc := &domain.Document{
		StoragePath: "test.docx",
		Filename:    "test.docx",
		MimeType:    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}

	text, err := ext.Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if text == "" {
		t.Error("expected non-empty text from DOCX")
	}
}

func TestExtractTextFromDocxXML(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>Hello</w:t></w:r> <w:r><w:t>World</w:t></w:r></w:p>
<w:p><w:r><w:t>Second paragraph.</w:t></w:r></w:p>
</w:body>
</w:document>`

	text := extractTextFromDocxXML([]byte(xml))
	if !strings.Contains(text, "Hello") {
		t.Errorf("expected 'Hello' in text, got %q", text)
	}
	if !strings.Contains(text, "World") {
		t.Errorf("expected 'World' in text, got %q", text)
	}
	if !strings.Contains(text, "Second paragraph") {
		t.Errorf("expected 'Second paragraph' in text, got %q", text)
	}
}

func TestExtractTextFromDocxXML_Empty(t *testing.T) {
	text := extractTextFromDocxXML([]byte(`<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body></w:body></w:document>`))
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
}
```

- [ ] **Step 2: Implement DOCX Extractor**

Create `internal/infrastructure/extractor/docx/extractor.go`:

```go
package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type Extractor struct {
	storage ports.ObjectStorage
}

func NewExtractor(storage ports.ObjectStorage) *Extractor {
	return &Extractor{storage: storage}
}

func (e *Extractor) Extract(ctx context.Context, doc *domain.Document) (string, error) {
	reader, err := e.storage.Open(ctx, doc.StoragePath)
	if err != nil {
		return "", fmt.Errorf("open docx document: %w", err)
	}
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read docx document: %w", err)
	}

	if len(raw) == 0 {
		return "", fmt.Errorf("empty docx file: %s", doc.Filename)
	}

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("open docx as zip: %w", err)
	}

	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("open word/document.xml: %w", err)
			}
			defer rc.Close()

			xmlData, err := io.ReadAll(rc)
			if err != nil {
				return "", fmt.Errorf("read word/document.xml: %w", err)
			}

			text := extractTextFromDocxXML(xmlData)
			if text == "" {
				return "", fmt.Errorf("no text content in docx: %s", doc.Filename)
			}
			return text, nil
		}
	}

	return "", fmt.Errorf("word/document.xml not found in docx: %s", doc.Filename)
}

// extractTextFromDocxXML parses OOXML and extracts text from <w:t> elements.
// Paragraphs (<w:p>) are separated by newlines.
func extractTextFromDocxXML(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var paragraphs []string
	var currentParagraph strings.Builder
	inText := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				inText = true
			}
			if t.Name.Local == "p" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				if currentParagraph.Len() > 0 {
					paragraphs = append(paragraphs, strings.TrimSpace(currentParagraph.String()))
					currentParagraph.Reset()
				}
			}
		case xml.EndElement:
			if t.Name.Local == "t" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				inText = false
			}
		case xml.CharData:
			if inText {
				currentParagraph.Write(t)
			}
		}
	}

	if currentParagraph.Len() > 0 {
		paragraphs = append(paragraphs, strings.TrimSpace(currentParagraph.String()))
	}

	var nonEmpty []string
	for _, p := range paragraphs {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}

	return strings.Join(nonEmpty, "\n")
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/infrastructure/extractor/docx/ -v`
Expected: PASS (XML parsing tests pass, file test skips if no fixture)

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/extractor/docx/
git commit -m "feat(extractor/docx): DOCX text extraction via XML parsing"
```

---

### Task 5: Implement Spreadsheet Extractor (XLSX + CSV)

**Files:**
- Create: `internal/infrastructure/extractor/spreadsheet/extractor.go`
- Create: `internal/infrastructure/extractor/spreadsheet/extractor_test.go`

- [ ] **Step 1: Add dependency**

Run: `go get github.com/xuri/excelize/v2`

- [ ] **Step 2: Write test**

Create `internal/infrastructure/extractor/spreadsheet/extractor_test.go`:

```go
package spreadsheet

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type storageFake struct {
	data []byte
}

func (f *storageFake) Save(context.Context, string, io.Reader) error { return nil }
func (f *storageFake) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(f.data))), nil
}

func TestCSVExtract(t *testing.T) {
	csv := "Name,Age,City\nAlice,30,Moscow\nBob,25,London\n"
	storage := &storageFake{data: []byte(csv)}
	ext := NewExtractor(storage)

	doc := &domain.Document{
		StoragePath: "data.csv",
		Filename:    "data.csv",
		MimeType:    "text/csv",
	}

	text, err := ext.Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if !strings.Contains(text, "Alice") {
		t.Errorf("expected 'Alice' in text, got %q", text)
	}
	if !strings.Contains(text, "Moscow") {
		t.Errorf("expected 'Moscow' in text, got %q", text)
	}
}

func TestCSVExtract_Empty(t *testing.T) {
	storage := &storageFake{data: []byte("")}
	ext := NewExtractor(storage)

	doc := &domain.Document{StoragePath: "empty.csv", Filename: "empty.csv", MimeType: "text/csv"}
	_, err := ext.Extract(context.Background(), doc)
	if err == nil {
		t.Error("expected error for empty CSV")
	}
}
```

- [ ] **Step 3: Implement Spreadsheet Extractor**

Create `internal/infrastructure/extractor/spreadsheet/extractor.go`:

```go
package spreadsheet

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type Extractor struct {
	storage ports.ObjectStorage
}

func NewExtractor(storage ports.ObjectStorage) *Extractor {
	return &Extractor{storage: storage}
}

func (e *Extractor) Extract(ctx context.Context, doc *domain.Document) (string, error) {
	reader, err := e.storage.Open(ctx, doc.StoragePath)
	if err != nil {
		return "", fmt.Errorf("open spreadsheet: %w", err)
	}
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read spreadsheet: %w", err)
	}

	if len(raw) == 0 {
		return "", fmt.Errorf("empty spreadsheet file: %s", doc.Filename)
	}

	mime := strings.SplitN(doc.MimeType, ";", 2)[0]
	switch mime {
	case "text/csv":
		return extractCSV(raw, doc.Filename)
	default:
		return extractXLSX(raw, doc.Filename)
	}
}

func extractCSV(data []byte, filename string) (string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	records, err := r.ReadAll()
	if err != nil {
		return "", fmt.Errorf("parse csv: %w", err)
	}

	if len(records) == 0 {
		return "", fmt.Errorf("no data in csv: %s", filename)
	}

	var lines []string
	for _, row := range records {
		lines = append(lines, strings.Join(row, "\t"))
	}

	return strings.Join(lines, "\n"), nil
}

func extractXLSX(data []byte, filename string) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	var sections []string
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		if len(rows) == 0 {
			continue
		}

		var lines []string
		lines = append(lines, fmt.Sprintf("Sheet: %s", sheet))
		for _, row := range rows {
			lines = append(lines, strings.Join(row, "\t"))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if len(sections) == 0 {
		return "", fmt.Errorf("no data in xlsx: %s", filename)
	}

	return strings.Join(sections, "\n\n"), nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/infrastructure/extractor/spreadsheet/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/extractor/spreadsheet/
git commit -m "feat(extractor/spreadsheet): XLSX and CSV text extraction"
```

---

### Task 6: Wire ExtractorRegistry into ProcessDocumentUseCase and EnrichDocumentUseCase

**Files:**
- Modify: `internal/core/usecase/process.go`
- Modify: `internal/core/usecase/process_test.go`
- Modify: `internal/core/usecase/enrich.go`
- Modify: `internal/core/usecase/enrich_test.go`

- [ ] **Step 1: Update ProcessDocumentUseCase**

In `internal/core/usecase/process.go`, replace `extractor ports.TextExtractor` with `extractors ports.ExtractorRegistry`:

```go
type ProcessDocumentUseCase struct {
	repo          ports.DocumentRepository
	extractors    ports.ExtractorRegistry
	metaExtractor ports.MetadataExtractor
	chunkers      ports.ChunkerRegistry
	embedder      ports.Embedder
	vectorDB      ports.VectorStore
	queue         ports.MessageQueue
}
```

Update constructor parameter from `extractor ports.TextExtractor` to `extractors ports.ExtractorRegistry`.

Update `extractText`:

```go
func (uc *ProcessDocumentUseCase) extractText(ctx context.Context, doc *domain.Document) (string, error) {
	extractor := uc.extractors.ForMimeType(doc.MimeType)
	text, err := extractor.Extract(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("extract text: %w", err)
	}
	if text == "" {
		return "", domain.WrapError(domain.ErrInvalidInput, "extract text", errors.New("empty extracted text"))
	}
	return text, nil
}
```

- [ ] **Step 2: Update EnrichDocumentUseCase**

In `internal/core/usecase/enrich.go`, replace `extractor ports.TextExtractor` with `extractors ports.ExtractorRegistry`:

```go
type EnrichDocumentUseCase struct {
	repo       ports.DocumentRepository
	extractors ports.ExtractorRegistry
	classifier ports.DocumentClassifier
	vectorDB   ports.VectorStore
}
```

Update constructor and `EnrichByID`:

```go
text, err := uc.extractors.ForMimeType(doc.MimeType).Extract(ctx, doc)
```

- [ ] **Step 3: Update process_test.go**

Add `extractorRegistryFake`:

```go
type extractorRegistryFake struct {
	text string
	err  error
}

func (f *extractorRegistryFake) ForMimeType(string) ports.TextExtractor {
	return &extractorFake{text: f.text, err: f.err}
}
```

Replace all `&extractorFake{text: ...}` in `NewProcessDocumentUseCase` calls with `&extractorRegistryFake{text: ...}`.

- [ ] **Step 4: Update enrich_test.go**

Add same `extractorRegistryFake` (or reuse if in same package). Replace `&extractorFake{text: ...}` in `NewEnrichDocumentUseCase` calls.

- [ ] **Step 5: Run tests**

Run: `go build ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(usecase): replace TextExtractor with ExtractorRegistry in Process and Enrich"
```

---

### Task 7: Wire ExtractorRegistry in bootstrap

**Files:**
- Modify: `internal/bootstrap/bootstrap.go`

- [ ] **Step 1: Add imports and wire registry**

Add imports:

```go
"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/extractor"
extpdf "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/extractor/pdf"
extdocx "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/extractor/docx"
extspreadsheet "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/extractor/spreadsheet"
```

Replace `extractor := plaintext.NewExtractor(storage)` with:

```go
plaintextExt := plaintext.NewExtractor(storage)
extractorRegistry := extractor.NewRegistry(plaintextExt)
extractorRegistry.Register("application/pdf", extpdf.NewExtractor(storage))
extractorRegistry.Register("application/vnd.openxmlformats-officedocument.wordprocessingml.document", extdocx.NewExtractor(storage))
extractorRegistry.Register("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", extspreadsheet.NewExtractor(storage))
extractorRegistry.Register("text/csv", extspreadsheet.NewExtractor(storage))
```

Update `NewProcessDocumentUseCase` and `NewEnrichDocumentUseCase` calls to pass `extractorRegistry` instead of `extractor`.

- [ ] **Step 2: Run build and tests**

Run: `go build ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/bootstrap/bootstrap.go
git commit -m "feat(bootstrap): wire ExtractorRegistry with PDF, DOCX, XLSX, CSV extractors"
```

---

### Task 8: Final verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1 -v 2>&1 | grep -E "FAIL|ok|--- FAIL"`
Expected: All PASS

- [ ] **Step 2: Run vet**

Run: `go vet ./...`
Expected: Clean

- [ ] **Step 3: Tidy modules**

Run: `go mod tidy`

- [ ] **Step 4: Commit if needed**

```bash
git add go.mod go.sum && git commit -m "chore: go mod tidy"
```
