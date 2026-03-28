# Deterministic Ingest + Async LLM Enrichment — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace fragile LLM classification in the ingest pipeline with deterministic metadata extraction, add async LLM enrichment via a separate NATS queue so documents are immediately searchable.

**Architecture:** The sync pipeline extracts metadata from frontmatter/filename/mime, chunks, embeds, indexes, and marks `ready`. After indexing, it publishes to `documents.enrich` NATS subject. A new `EnrichDocumentUseCase` subscribes, runs LLM classification, merges results, and updates Postgres + Qdrant payload.

**Tech Stack:** Go, NATS (plain pub/sub), Qdrant REST API, PostgreSQL, YAML frontmatter parsing.

**Spec:** `docs/superpowers/specs/2026-03-28-deterministic-ingest-async-enrichment.md`

---

### Task 1: Extend domain model — `DocumentMetadata` + new `Document` fields

**Files:**
- Modify: `internal/core/domain/document.go`

- [ ] **Step 1: Write the test for DocumentMetadata struct**

Create `internal/core/domain/document_metadata_test.go`:

```go
package domain

import "testing"

func TestDocumentMetadataDefaults(t *testing.T) {
	meta := DocumentMetadata{}
	if meta.SourceType != "" {
		t.Fatalf("expected empty SourceType, got %q", meta.SourceType)
	}
	if meta.Tags == nil {
		// Tags should be nil by default (zero value for slice)
	}
}
```

Run: `go test ./internal/core/domain/ -run TestDocumentMetadataDefaults -v`
Expected: FAIL — `DocumentMetadata` not defined.

- [ ] **Step 2: Add DocumentMetadata and new Document fields**

In `internal/core/domain/document.go`, add `DocumentMetadata` struct and extend `Document`:

```go
type DocumentMetadata struct {
	SourceType string   `json:"source_type"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Headers    []string `json:"headers"`
	Path       string   `json:"path"`
}

// Add new fields to Document struct (after existing fields, before Status):
//   SourceType string   `json:"source_type"`
//   Title      string   `json:"title"`
//   Headers    []string `json:"headers,omitempty"`
//   Path       string   `json:"path"`
```

The full `Document` struct becomes:

```go
type Document struct {
	ID          string         `json:"id"`
	Filename    string         `json:"filename"`
	MimeType    string         `json:"mime_type"`
	StoragePath string         `json:"storage_path"`
	Category    string         `json:"category,omitempty"`
	Subcategory string         `json:"subcategory,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Confidence  float64        `json:"confidence,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	SourceType  string         `json:"source_type"`
	Title       string         `json:"title"`
	Headers     []string       `json:"headers,omitempty"`
	Path        string         `json:"path"`
	Status      DocumentStatus `json:"status"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/domain/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/domain/document.go internal/core/domain/document_metadata_test.go
git commit -m "feat(domain): add DocumentMetadata and extend Document with source_type, title, headers, path"
```

---

### Task 2: Add `MetadataExtractor` port + extend `VectorStore` and `MessageQueue` ports

**Files:**
- Modify: `internal/core/ports/outbound.go`

- [ ] **Step 1: Add new interfaces and methods to outbound.go**

Add `MetadataExtractor` interface:

```go
// MetadataExtractor extracts document metadata deterministically (no LLM).
type MetadataExtractor interface {
	ExtractMetadata(ctx context.Context, doc *domain.Document, text string) (domain.DocumentMetadata, error)
}
```

Add to `VectorStore` interface:

```go
UpdateChunksPayload(ctx context.Context, docID string, payload map[string]any) error
```

Add to `MessageQueue` interface:

```go
PublishDocumentEnrich(ctx context.Context, documentID string) error
SubscribeDocumentEnrich(ctx context.Context, handler func(context.Context, string) error) error
```

- [ ] **Step 2: Run vet to check compilation**

Run: `go vet ./internal/core/ports/...`
Expected: Compiler errors in implementations that don't satisfy new interface methods yet. This is expected — we'll fix them in later tasks.

- [ ] **Step 3: Commit**

```bash
git add internal/core/ports/outbound.go
git commit -m "feat(ports): add MetadataExtractor, extend VectorStore and MessageQueue interfaces"
```

---

### Task 3: Implement deterministic metadata extractor

**Files:**
- Create: `internal/infrastructure/extractor/metadata/extractor.go`
- Create: `internal/infrastructure/extractor/metadata/extractor_test.go`
- Create: `internal/infrastructure/extractor/metadata/frontmatter.go`
- Create: `internal/infrastructure/extractor/metadata/frontmatter_test.go`

- [ ] **Step 1: Write frontmatter parser tests**

Create `internal/infrastructure/extractor/metadata/frontmatter_test.go`:

```go
package metadata

import "testing"

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantCat  string
		wantTags []string
		wantTitle string
		wantBody string
	}{
		{
			name:     "full frontmatter",
			text:     "---\ncategory: programming\ntags:\n  - go\n  - patterns\ntitle: Clean Architecture\n---\n\nBody text here.",
			wantCat:  "programming",
			wantTags: []string{"go", "patterns"},
			wantTitle: "Clean Architecture",
			wantBody: "Body text here.",
		},
		{
			name:     "no frontmatter",
			text:     "Just plain text\nwith multiple lines.",
			wantCat:  "",
			wantTags: nil,
			wantTitle: "",
			wantBody: "Just plain text\nwith multiple lines.",
		},
		{
			name:     "empty frontmatter",
			text:     "---\n---\nBody.",
			wantCat:  "",
			wantTags: nil,
			wantTitle: "",
			wantBody: "Body.",
		},
		{
			name:     "tags as inline list",
			text:     "---\ntags: [go, rust]\n---\nContent.",
			wantTags: []string{"go", "rust"},
			wantBody: "Content.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := parseFrontmatter(tt.text)
			if fm.Category != tt.wantCat {
				t.Errorf("category = %q, want %q", fm.Category, tt.wantCat)
			}
			if tt.wantTitle != "" && fm.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", fm.Title, tt.wantTitle)
			}
			if tt.wantTags != nil {
				if len(fm.Tags) != len(tt.wantTags) {
					t.Errorf("tags = %v, want %v", fm.Tags, tt.wantTags)
				}
			}
			if tt.wantBody != "" && body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}
```

Run: `go test ./internal/infrastructure/extractor/metadata/ -run TestParseFrontmatter -v`
Expected: FAIL — package/function not defined.

- [ ] **Step 2: Implement frontmatter parser**

Create `internal/infrastructure/extractor/metadata/frontmatter.go`:

```go
package metadata

import (
	"strings"

	"gopkg.in/yaml.v3"
)

type frontmatterData struct {
	Category string   `yaml:"category"`
	Tags     []string `yaml:"tags"`
	Title    string   `yaml:"title"`
}

// parseFrontmatter extracts YAML frontmatter (between --- delimiters) and returns
// parsed data + remaining body text. If no frontmatter found, returns zero data and original text.
func parseFrontmatter(text string) (frontmatterData, string) {
	if !strings.HasPrefix(text, "---\n") {
		return frontmatterData{}, text
	}

	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return frontmatterData{}, text
	}

	yamlBlock := text[4 : 4+end]
	body := strings.TrimSpace(text[4+end+4:])

	var fm frontmatterData
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return frontmatterData{}, text
	}

	return fm, body
}
```

Run: `go test ./internal/infrastructure/extractor/metadata/ -run TestParseFrontmatter -v`
Expected: PASS (may need `go get gopkg.in/yaml.v3` first).

- [ ] **Step 3: Write metadata extractor tests**

Create `internal/infrastructure/extractor/metadata/extractor_test.go`:

```go
package metadata

import (
	"context"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestExtractMetadata_MarkdownWithFrontmatter(t *testing.T) {
	ext := New()
	doc := &domain.Document{
		Filename: "notes/programming/go-patterns.md",
		MimeType: "text/markdown",
	}
	text := "---\ncategory: programming\ntags:\n  - go\ntitle: Go Patterns\n---\n\n# Go Patterns\n\nSome content about Go patterns and best practices."

	meta, err := ext.ExtractMetadata(context.Background(), doc, text)
	if err != nil {
		t.Fatalf("ExtractMetadata() error = %v", err)
	}
	if meta.SourceType != "markdown" {
		t.Errorf("SourceType = %q, want %q", meta.SourceType, "markdown")
	}
	if meta.Category != "programming" {
		t.Errorf("Category = %q, want %q", meta.Category, "programming")
	}
	if meta.Title != "Go Patterns" {
		t.Errorf("Title = %q, want %q", meta.Title, "Go Patterns")
	}
	if len(meta.Tags) != 1 || meta.Tags[0] != "go" {
		t.Errorf("Tags = %v, want [go]", meta.Tags)
	}
	if len(meta.Headers) == 0 || meta.Headers[0] != "Go Patterns" {
		t.Errorf("Headers = %v, want [Go Patterns]", meta.Headers)
	}
}

func TestExtractMetadata_PlainText(t *testing.T) {
	ext := New()
	doc := &domain.Document{
		Filename: "report.txt",
		MimeType: "text/plain",
	}
	text := "This is a plain text report about something important.\n\nSecond paragraph."

	meta, err := ext.ExtractMetadata(context.Background(), doc, text)
	if err != nil {
		t.Fatalf("ExtractMetadata() error = %v", err)
	}
	if meta.SourceType != "text" {
		t.Errorf("SourceType = %q, want %q", meta.SourceType, "text")
	}
	if meta.Title != "report" {
		t.Errorf("Title = %q, want %q", meta.Title, "report")
	}
	if meta.Summary == "" {
		t.Error("Summary should not be empty")
	}
}

func TestExtractMetadata_MarkdownNoFrontmatter_UsesH1(t *testing.T) {
	ext := New()
	doc := &domain.Document{
		Filename: "notes/ideas.md",
		MimeType: "text/markdown",
	}
	text := "# My Great Idea\n\nSome content here.\n\n## Section 2\n\nMore content."

	meta, err := ext.ExtractMetadata(context.Background(), doc, text)
	if err != nil {
		t.Fatalf("ExtractMetadata() error = %v", err)
	}
	if meta.Title != "My Great Idea" {
		t.Errorf("Title = %q, want %q", meta.Title, "My Great Idea")
	}
	if meta.Category != "notes" {
		t.Errorf("Category = %q, want %q", meta.Category, "notes")
	}
	if len(meta.Headers) != 2 {
		t.Errorf("Headers count = %d, want 2", len(meta.Headers))
	}
}

func TestExtractMetadata_CategoryFromPath(t *testing.T) {
	ext := New()
	doc := &domain.Document{
		Filename: "vault/science/physics/quantum.md",
		MimeType: "text/markdown",
	}
	text := "# Quantum Mechanics\n\nContent."

	meta, err := ext.ExtractMetadata(context.Background(), doc, text)
	if err != nil {
		t.Fatalf("ExtractMetadata() error = %v", err)
	}
	// Category from first meaningful dir in path
	if meta.Category != "science" {
		t.Errorf("Category = %q, want %q", meta.Category, "science")
	}
	if meta.Path != "vault/science/physics/quantum.md" {
		t.Errorf("Path = %q, want %q", meta.Path, "vault/science/physics/quantum.md")
	}
}
```

Run: `go test ./internal/infrastructure/extractor/metadata/ -run TestExtractMetadata -v`
Expected: FAIL — `New()` and `ExtractMetadata` not defined.

- [ ] **Step 4: Implement metadata extractor**

Create `internal/infrastructure/extractor/metadata/extractor.go`:

```go
package metadata

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

var markdownHeaderRe = regexp.MustCompile(`(?m)^#{1,3}\s+(.+)$`)

const maxSummaryLen = 200

// Extractor implements ports.MetadataExtractor using deterministic rules.
type Extractor struct{}

func New() *Extractor {
	return &Extractor{}
}

func (e *Extractor) ExtractMetadata(_ context.Context, doc *domain.Document, text string) (domain.DocumentMetadata, error) {
	meta := domain.DocumentMetadata{
		SourceType: detectSourceType(doc.Filename, doc.MimeType),
		Path:       doc.Filename,
	}

	bodyText := text

	// Parse frontmatter for markdown files.
	if meta.SourceType == "markdown" {
		fm, body := parseFrontmatter(text)
		bodyText = body
		if fm.Category != "" {
			meta.Category = fm.Category
		}
		if len(fm.Tags) > 0 {
			meta.Tags = fm.Tags
		}
		if fm.Title != "" {
			meta.Title = fm.Title
		}
	}

	// Extract headers from markdown.
	if meta.SourceType == "markdown" {
		meta.Headers = extractHeaders(bodyText)
	}

	// Title fallback: first H1 header, then filename without extension.
	if meta.Title == "" && len(meta.Headers) > 0 {
		meta.Title = meta.Headers[0]
	}
	if meta.Title == "" {
		meta.Title = filenameWithoutExt(doc.Filename)
	}

	// Category fallback: first meaningful directory from path.
	if meta.Category == "" {
		meta.Category = categoryFromPath(doc.Filename)
	}

	// Summary: first N characters of body text up to double newline.
	meta.Summary = truncateSummary(bodyText, maxSummaryLen)

	return meta, nil
}

func detectSourceType(filename, mimeType string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown":
		return "markdown"
	case ".txt":
		return "text"
	}
	if strings.Contains(mimeType, "markdown") {
		return "markdown"
	}
	if strings.HasPrefix(mimeType, "text/") {
		return "text"
	}
	return "unknown"
}

func extractHeaders(text string) []string {
	matches := markdownHeaderRe.FindAllStringSubmatch(text, -1)
	headers := make([]string, 0, len(matches))
	for _, m := range matches {
		if h := strings.TrimSpace(m[1]); h != "" {
			headers = append(headers, h)
		}
	}
	return headers
}

func filenameWithoutExt(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	if ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}

func categoryFromPath(filename string) string {
	dir := filepath.Dir(filename)
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(dir), "/")
	// Skip common vault-root prefixes.
	for _, p := range parts {
		p = strings.ToLower(p)
		if p == "" || p == "." || p == "vault" || p == "notes" || p == "docs" {
			continue
		}
		return p
	}
	return ""
}

func truncateSummary(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	// Cut at first double newline.
	if idx := strings.Index(text, "\n\n"); idx > 0 && idx < maxLen {
		text = text[:idx]
	}
	runes := []rune(text)
	if len(runes) > maxLen {
		runes = runes[:maxLen]
	}
	return strings.TrimSpace(string(runes))
}
```

- [ ] **Step 5: Add gopkg.in/yaml.v3 dependency if missing**

Run: `go get gopkg.in/yaml.v3`

- [ ] **Step 6: Run tests**

Run: `go test ./internal/infrastructure/extractor/metadata/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/infrastructure/extractor/metadata/
git commit -m "feat(extractor): deterministic metadata extractor with frontmatter parsing"
```

---

### Task 4: Extend NATS queue — enrich subject publish/subscribe

**Files:**
- Modify: `internal/infrastructure/queue/nats/queue.go`
- Create: `internal/infrastructure/queue/nats/queue_test.go` (if not exists)

- [ ] **Step 1: Add enrichSubject field and new methods**

In `internal/infrastructure/queue/nats/queue.go`:

Add `enrichSubject` field to `Queue` struct:

```go
type Queue struct {
	conn           *nats.Conn
	subject        string
	enrichSubject  string
	executor       *resilience.Executor
}
```

Set `enrichSubject` in constructor — derive from base subject by appending `.enrich`:

```go
// In NewWithOptions, after creating Queue:
return &Queue{
	conn:          conn,
	subject:       subject,
	enrichSubject: subject + ".enrich",
	executor:      options.ResilienceExecutor,
}, nil
```

Add `PublishDocumentEnrich`:

```go
func (q *Queue) PublishDocumentEnrich(ctx context.Context, documentID string) error {
	call := func(_ context.Context) error {
		if err := q.conn.Publish(q.enrichSubject, []byte(documentID)); err != nil {
			return fmt.Errorf("nats publish enrich: %w", err)
		}
		return nil
	}

	var err error
	if q.executor != nil {
		err = q.executor.Execute(ctx, "nats.publish_enrich", call, classifyNATSError)
	} else {
		err = call(ctx)
	}
	if err != nil {
		return wrapTemporaryIfNeeded(err)
	}
	return nil
}
```

Add `SubscribeDocumentEnrich` (same pattern as `SubscribeDocumentIngested` but with `enrichSubject` and queue group `"enrichers"`):

```go
func (q *Queue) SubscribeDocumentEnrich(ctx context.Context, handler func(context.Context, string) error) error {
	sub, err := q.conn.QueueSubscribe(q.enrichSubject, "enrichers", func(msg *nats.Msg) {
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}

		handlerCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		if err := handler(handlerCtx, string(msg.Data)); err != nil {
			log.Printf("enrichment handler error for doc=%s: %v", string(msg.Data), err)
		}
	})
	if err != nil {
		return fmt.Errorf("nats subscribe enrich: %w", err)
	}

	if err := q.conn.Flush(); err != nil {
		return fmt.Errorf("nats flush enrich: %w", err)
	}

	<-ctx.Done()
	if err := sub.Drain(); err != nil {
		return fmt.Errorf("nats drain enrich subscription: %w", err)
	}
	if err := q.conn.FlushTimeout(5 * time.Second); err != nil {
		return fmt.Errorf("nats flush after enrich drain: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Run vet**

Run: `go vet ./internal/infrastructure/queue/nats/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/queue/nats/queue.go
git commit -m "feat(nats): add enrich subject publish/subscribe for async LLM enrichment"
```

---

### Task 5: Add `UpdateChunksPayload` to Qdrant client

**Files:**
- Modify: `internal/infrastructure/vector/qdrant/client.go`
- Create: `internal/infrastructure/vector/qdrant/client_update_test.go`

- [ ] **Step 1: Write test for UpdateChunksPayload**

Create `internal/infrastructure/vector/qdrant/client_update_test.go`:

```go
package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateChunksPayload_BuildsCorrectRequest(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"completed"}}`))
	}))
	defer server.Close()

	client := New(server.URL, "test-collection")
	err := client.UpdateChunksPayload(context.Background(), "doc-123", map[string]any{
		"category": "science",
		"tags":     []string{"physics"},
	})
	if err != nil {
		t.Fatalf("UpdateChunksPayload() error = %v", err)
	}

	// Verify filter has doc_id match.
	filter, ok := gotBody["filter"].(map[string]any)
	if !ok {
		t.Fatal("expected filter in body")
	}
	must, ok := filter["must"].([]any)
	if !ok || len(must) == 0 {
		t.Fatal("expected must array in filter")
	}

	// Verify payload is present.
	payload, ok := gotBody["payload"]
	if !ok || payload == nil {
		t.Fatal("expected payload in body")
	}
}
```

Run: `go test ./internal/infrastructure/vector/qdrant/ -run TestUpdateChunksPayload -v`
Expected: FAIL — method not defined.

- [ ] **Step 2: Implement UpdateChunksPayload**

In `internal/infrastructure/vector/qdrant/client.go`, add:

```go
func (c *Client) UpdateChunksPayload(ctx context.Context, docID string, payload map[string]any) error {
	reqBody := map[string]any{
		"payload": payload,
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "doc_id",
					"match": map[string]any{
						"value": docID,
					},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal set_payload body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/payload?wait=true", c.baseURL, c.collection)
	resp, err := c.doRequest(ctx, "set_payload", http.MethodPost, url, body, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if msg := strings.TrimSpace(string(respBody)); msg != "" {
			return fmt.Errorf("qdrant set_payload status: %s: %s", resp.Status, msg)
		}
		return fmt.Errorf("qdrant set_payload status: %s", resp.Status)
	}
	return nil
}
```

- [ ] **Step 3: Extend IndexChunks payload with new metadata fields**

In `IndexChunks`, extend the payload map to include new fields from `doc`:

```go
Payload: map[string]any{
	"doc_id":      doc.ID,
	"filename":    doc.Filename,
	"category":    doc.Category,
	"subcategory": doc.Subcategory,
	"chunk_index": i,
	"text":        chunks[i],
	"source_type": doc.SourceType,
	"title":       doc.Title,
	"path":        doc.Path,
	"tags":        doc.Tags,
},
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/infrastructure/vector/qdrant/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/vector/qdrant/client.go internal/infrastructure/vector/qdrant/client_update_test.go
git commit -m "feat(qdrant): add UpdateChunksPayload and extend IndexChunks with metadata fields"
```

---

### Task 6: Add DB migration + update Postgres repository

**Files:**
- Create: `db/migrations/002_add_metadata.sql`
- Modify: `internal/infrastructure/repository/postgres/document_repository.go`

- [ ] **Step 1: Create migration file**

Create `db/migrations/002_add_metadata.sql`:

```sql
ALTER TABLE documents ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS headers JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS path TEXT NOT NULL DEFAULT '';
```

- [ ] **Step 2: Update EnsureSchema DDL**

In `internal/infrastructure/repository/postgres/document_repository.go`, after the main `CREATE TABLE IF NOT EXISTS documents` block and its indexes, add the new ALTER TABLE statements to the DDL string:

```sql
ALTER TABLE documents ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS headers JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS path TEXT NOT NULL DEFAULT '';
```

- [ ] **Step 3: Update Create method**

Update the `Create` method to include new columns. The INSERT statement becomes:

```go
_, err = r.db.ExecContext(ctx, `
INSERT INTO documents (
	id, filename, mime_type, storage_path, category, subcategory, tags, confidence, summary,
	source_type, title, headers, path,
	status, error_message, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
`,
	doc.ID, doc.Filename, doc.MimeType, doc.StoragePath, doc.Category, doc.Subcategory, tagsJSON,
	doc.Confidence, doc.Summary,
	doc.SourceType, doc.Title, headersJSON, doc.Path,
	string(doc.Status), doc.Error, doc.CreatedAt, doc.UpdatedAt,
)
```

Marshal `headersJSON` before the query (same pattern as `tagsJSON`):

```go
headersJSON, err := json.Marshal(doc.Headers)
if err != nil {
	return fmt.Errorf("marshal headers: %w", err)
}
```

Handle nil `Headers` — initialize to `[]string{}` if nil before marshal.

- [ ] **Step 4: Update GetByID method**

Add new fields to SELECT and Scan:

```go
row := r.db.QueryRowContext(ctx, `
SELECT id, filename, mime_type, storage_path, category, subcategory, tags, confidence, summary,
	source_type, title, headers, path,
	status, error_message, created_at, updated_at
FROM documents
WHERE id = $1
`, id)

var headersRaw []byte
err := row.Scan(
	&doc.ID, &doc.Filename, &doc.MimeType, &doc.StoragePath, &doc.Category, &doc.Subcategory,
	&tagsRaw, &doc.Confidence, &doc.Summary,
	&doc.SourceType, &doc.Title, &headersRaw, &doc.Path,
	&status, &doc.Error, &doc.CreatedAt, &doc.UpdatedAt,
)
```

After existing `json.Unmarshal` for tags, add:

```go
if err := json.Unmarshal(headersRaw, &doc.Headers); err != nil {
	return nil, fmt.Errorf("unmarshal headers: %w", err)
}
```

- [ ] **Step 5: Add SaveMetadata method**

Add a new method for the enrichment use case to update metadata after LLM enrichment:

```go
func (r *DocumentRepository) SaveMetadata(ctx context.Context, id string, meta domain.DocumentMetadata) error {
	tagsJSON, err := json.Marshal(meta.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	headersJSON, err := json.Marshal(meta.Headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE documents
SET category = COALESCE(NULLIF($2, ''), category),
    tags = $3,
    summary = COALESCE(NULLIF($4, ''), summary),
    headers = $5,
    source_type = COALESCE(NULLIF($6, ''), source_type),
    title = COALESCE(NULLIF($7, ''), title),
    path = COALESCE(NULLIF($8, ''), path),
    updated_at = $9
WHERE id = $1
`, id, meta.Category, tagsJSON, meta.Summary, headersJSON, meta.SourceType, meta.Title, meta.Path, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("save metadata: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for save metadata: %w", err)
	}
	if rows == 0 {
		return domain.WrapError(domain.ErrDocumentNotFound, "save metadata", fmt.Errorf("id=%s", id))
	}
	return nil
}
```

- [ ] **Step 6: Run vet**

Run: `go vet ./internal/infrastructure/repository/postgres/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add db/migrations/002_add_metadata.sql internal/infrastructure/repository/postgres/document_repository.go
git commit -m "feat(postgres): add metadata columns migration and update repository CRUD"
```

---

### Task 7: Refactor ProcessDocumentUseCase — replace classifier with metadata extractor

**Files:**
- Modify: `internal/core/usecase/process.go`
- Modify: `internal/core/usecase/process_test.go`

- [ ] **Step 1: Update process_test.go — replace classifierFake with metadataExtractorFake**

In `internal/core/usecase/process_test.go`:

Remove `classifierFake`. Add `metadataExtractorFake` and `queueFake`:

```go
type metadataExtractorFake struct {
	meta domain.DocumentMetadata
	err  error
}

func (f *metadataExtractorFake) ExtractMetadata(context.Context, *domain.Document, string) (domain.DocumentMetadata, error) {
	if f.err != nil {
		return domain.DocumentMetadata{}, f.err
	}
	return f.meta, nil
}

type queueFake struct {
	publishedEnrichIDs []string
	publishErr         error
}

func (f *queueFake) PublishDocumentIngested(context.Context, string) error { return nil }
func (f *queueFake) SubscribeDocumentIngested(context.Context, func(context.Context, string) error) error {
	return nil
}
func (f *queueFake) PublishDocumentEnrich(_ context.Context, docID string) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	f.publishedEnrichIDs = append(f.publishedEnrichIDs, docID)
	return nil
}
func (f *queueFake) SubscribeDocumentEnrich(context.Context, func(context.Context, string) error) error {
	return nil
}
func (f *queueFake) Close() {}
```

Also add `UpdateChunksPayload` stub to `vectorFake`:

```go
func (f *vectorFake) UpdateChunksPayload(context.Context, string, map[string]any) error { return nil }
```

Update `TestProcessByIDSuccess`:

```go
func TestProcessByIDSuccess(t *testing.T) {
	repo := &processRepoFake{doc: &domain.Document{ID: "doc-1", Filename: "test.md"}}
	q := &queueFake{}
	uc := NewProcessDocumentUseCase(
		repo,
		&extractorFake{text: "Some text"},
		&metadataExtractorFake{meta: domain.DocumentMetadata{Category: "general", SourceType: "markdown"}},
		&chunkerFake{chunks: []string{"a", "b"}},
		&embedderFake{vectors: [][]float32{{1}, {2}}},
		&vectorFake{},
		q,
	)

	if err := uc.ProcessByID(context.Background(), "doc-1"); err != nil {
		t.Fatalf("ProcessByID() error = %v", err)
	}
	if len(repo.statusCalls) != 2 {
		t.Fatalf("expected 2 status calls, got %d", len(repo.statusCalls))
	}
	if repo.statusCalls[0].status != domain.StatusProcessing || repo.statusCalls[1].status != domain.StatusReady {
		t.Fatalf("unexpected status sequence: %+v", repo.statusCalls)
	}
	if len(q.publishedEnrichIDs) != 1 || q.publishedEnrichIDs[0] != "doc-1" {
		t.Fatalf("expected enrich publish for doc-1, got %v", q.publishedEnrichIDs)
	}
}
```

Update `TestProcessByIDMarksFailedOnExtractError` and `TestProcessByIDMarksFailedOnVectorMismatch` to use new constructor signature (replace `&classifierFake{}` with `&metadataExtractorFake{}`, add `&queueFake{}`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/core/usecase/ -run TestProcessByID -v`
Expected: FAIL — constructor signature mismatch.

- [ ] **Step 3: Refactor ProcessDocumentUseCase**

In `internal/core/usecase/process.go`:

Replace `classifier ports.DocumentClassifier` with `metaExtractor ports.MetadataExtractor` and add `queue ports.MessageQueue`:

```go
type ProcessDocumentUseCase struct {
	repo          ports.DocumentRepository
	extractor     ports.TextExtractor
	metaExtractor ports.MetadataExtractor
	chunker       ports.Chunker
	embedder      ports.Embedder
	vectorDB      ports.VectorStore
	queue         ports.MessageQueue
}

func NewProcessDocumentUseCase(
	repo ports.DocumentRepository,
	extractor ports.TextExtractor,
	metaExtractor ports.MetadataExtractor,
	chunker ports.Chunker,
	embedder ports.Embedder,
	vectorDB ports.VectorStore,
	queue ports.MessageQueue,
) *ProcessDocumentUseCase {
	return &ProcessDocumentUseCase{
		repo:          repo,
		extractor:     extractor,
		metaExtractor: metaExtractor,
		chunker:       chunker,
		embedder:      embedder,
		vectorDB:      vectorDB,
		queue:         queue,
	}
}
```

Replace `processPipeline` — remove classify, add metadata extraction and enrich publish:

```go
func (uc *ProcessDocumentUseCase) processPipeline(ctx context.Context, documentID string) (*domain.Document, error) {
	doc, err := uc.loadDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}

	text, err := uc.extractText(ctx, doc)
	if err != nil {
		return nil, err
	}

	meta, err := uc.extractMetadata(ctx, doc, text)
	if err != nil {
		return nil, err
	}

	chunks, err := uc.chunk(ctx, text)
	if err != nil {
		return nil, err
	}

	vectors, err := uc.embed(ctx, chunks)
	if err != nil {
		return nil, err
	}

	uc.applyMetadata(doc, meta)

	if err := uc.index(ctx, doc, chunks, vectors); err != nil {
		return nil, err
	}

	// Publish enrichment event (best-effort — don't fail the pipeline).
	if pubErr := uc.queue.PublishDocumentEnrich(ctx, doc.ID); pubErr != nil {
		slog.Warn("publish_enrich_failed", "document_id", doc.ID, "error", pubErr)
	}

	return doc, nil
}
```

Add `extractMetadata` method:

```go
func (uc *ProcessDocumentUseCase) extractMetadata(ctx context.Context, doc *domain.Document, text string) (domain.DocumentMetadata, error) {
	meta, err := uc.metaExtractor.ExtractMetadata(ctx, doc, text)
	if err != nil {
		return domain.DocumentMetadata{}, fmt.Errorf("extract metadata: %w", err)
	}
	return meta, nil
}
```

Replace `applyClassification` with `applyMetadata`:

```go
func (uc *ProcessDocumentUseCase) applyMetadata(doc *domain.Document, meta domain.DocumentMetadata) {
	doc.SourceType = meta.SourceType
	doc.Category = meta.Category
	doc.Tags = meta.Tags
	doc.Title = meta.Title
	doc.Summary = meta.Summary
	doc.Headers = meta.Headers
	doc.Path = meta.Path
}
```

Update `ProcessByID` — remove `persistClassification`, `processPipeline` now returns only `(*domain.Document, error)`:

```go
func (uc *ProcessDocumentUseCase) ProcessByID(ctx context.Context, documentID string) error {
	if err := uc.markStatus(ctx, documentID, domain.StatusProcessing, ""); err != nil {
		return fmt.Errorf("set status=processing: %w", err)
	}

	_, err := uc.processPipeline(ctx, documentID)
	if err != nil {
		if failErr := uc.markFailed(ctx, documentID, err); failErr != nil {
			return fmt.Errorf("%w; mark failed status: %v", err, failErr)
		}
		return err
	}

	if err := uc.markStatus(ctx, documentID, domain.StatusReady, ""); err != nil {
		return fmt.Errorf("set status=ready: %w", err)
	}

	return nil
}
```

Remove `classify`, `persistClassification`, and `applyClassification` methods.

Add `"log/slog"` to imports.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/core/usecase/ -run TestProcessByID -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/usecase/process.go internal/core/usecase/process_test.go
git commit -m "refactor(process): replace LLM classifier with deterministic metadata extractor"
```

---

### Task 8: Create EnrichDocumentUseCase

**Files:**
- Create: `internal/core/usecase/enrich.go`
- Create: `internal/core/usecase/enrich_test.go`

- [ ] **Step 1: Write enrich tests**

Create `internal/core/usecase/enrich_test.go`:

```go
package usecase

import (
	"context"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type enrichRepoFake struct {
	doc              *domain.Document
	getErr           error
	savedCls         domain.Classification
	savedClsID       string
	saveClsErr       error
}

func (f *enrichRepoFake) Create(context.Context, *domain.Document) error       { return nil }
func (f *enrichRepoFake) GetByID(context.Context, string) (*domain.Document, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	copyDoc := *f.doc
	return &copyDoc, nil
}
func (f *enrichRepoFake) UpdateStatus(context.Context, string, domain.DocumentStatus, string) error {
	return nil
}
func (f *enrichRepoFake) SaveClassification(_ context.Context, id string, cls domain.Classification) error {
	if f.saveClsErr != nil {
		return f.saveClsErr
	}
	f.savedClsID = id
	f.savedCls = cls
	return nil
}

type enrichVectorFake struct {
	updatedDocID  string
	updatedPayload map[string]any
	updateErr     error
}

func (f *enrichVectorFake) IndexChunks(context.Context, *domain.Document, []string, [][]float32) error {
	return nil
}
func (f *enrichVectorFake) Search(context.Context, []float32, int, domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	return nil, nil
}
func (f *enrichVectorFake) SearchLexical(context.Context, string, int, domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	return nil, nil
}
func (f *enrichVectorFake) UpdateChunksPayload(_ context.Context, docID string, payload map[string]any) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updatedDocID = docID
	f.updatedPayload = payload
	return nil
}

type classifierFake struct {
	cls domain.Classification
	err error
}

func (f *classifierFake) Classify(context.Context, string) (domain.Classification, error) {
	if f.err != nil {
		return domain.Classification{}, f.err
	}
	return f.cls, nil
}

func TestEnrichByID_Success(t *testing.T) {
	doc := &domain.Document{
		ID:       "doc-1",
		Filename: "test.md",
		Category: "existing-cat",
		Tags:     []string{"existing-tag"},
	}
	repo := &enrichRepoFake{doc: doc}
	vectorDB := &enrichVectorFake{}
	cls := &classifierFake{cls: domain.Classification{
		Category: "llm-cat",
		Tags:     []string{"llm-tag"},
		Summary:  "LLM summary",
		Confidence: 0.85,
	}}

	uc := NewEnrichDocumentUseCase(repo, &extractorFake{text: "some text"}, cls, vectorDB)

	err := uc.EnrichByID(context.Background(), "doc-1")
	if err != nil {
		t.Fatalf("EnrichByID() error = %v", err)
	}

	// Category should keep deterministic value (existing-cat), not LLM.
	if repo.savedCls.Category != "existing-cat" {
		t.Errorf("category = %q, want %q (deterministic wins)", repo.savedCls.Category, "existing-cat")
	}
	// Tags should be union.
	if len(repo.savedCls.Tags) != 2 {
		t.Errorf("tags = %v, want union of [existing-tag] and [llm-tag]", repo.savedCls.Tags)
	}
	// Summary should be LLM.
	if repo.savedCls.Summary != "LLM summary" {
		t.Errorf("summary = %q, want %q", repo.savedCls.Summary, "LLM summary")
	}
	// Qdrant payload updated.
	if vectorDB.updatedDocID != "doc-1" {
		t.Errorf("qdrant update doc_id = %q, want %q", vectorDB.updatedDocID, "doc-1")
	}
}

func TestEnrichByID_ClassifierError_NoFail(t *testing.T) {
	doc := &domain.Document{ID: "doc-1", Filename: "test.md", Tags: []string{}}
	repo := &enrichRepoFake{doc: doc}
	cls := &classifierFake{err: fmt.Errorf("LLM timeout")}

	uc := NewEnrichDocumentUseCase(repo, &extractorFake{text: "text"}, cls, &enrichVectorFake{})

	err := uc.EnrichByID(context.Background(), "doc-1")
	// Should NOT return error — enrichment failure is non-fatal.
	if err != nil {
		t.Fatalf("EnrichByID() should not fail on classifier error, got %v", err)
	}
}
```

Add `"fmt"` to the test file imports (needed for `fmt.Errorf` in `classifierFake`).

Run: `go test ./internal/core/usecase/ -run TestEnrichByID -v`
Expected: FAIL — `NewEnrichDocumentUseCase` not defined.

- [ ] **Step 2: Implement EnrichDocumentUseCase**

Create `internal/core/usecase/enrich.go`:

```go
package usecase

import (
	"context"
	"log/slog"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type EnrichDocumentUseCase struct {
	repo       ports.DocumentRepository
	extractor  ports.TextExtractor
	classifier ports.DocumentClassifier
	vectorDB   ports.VectorStore
}

func NewEnrichDocumentUseCase(
	repo ports.DocumentRepository,
	extractor ports.TextExtractor,
	classifier ports.DocumentClassifier,
	vectorDB ports.VectorStore,
) *EnrichDocumentUseCase {
	return &EnrichDocumentUseCase{
		repo:       repo,
		extractor:  extractor,
		classifier: classifier,
		vectorDB:   vectorDB,
	}
}

func (uc *EnrichDocumentUseCase) EnrichByID(ctx context.Context, documentID string) error {
	doc, err := uc.repo.GetByID(ctx, documentID)
	if err != nil {
		slog.Warn("enrich_load_doc_failed", "document_id", documentID, "error", err)
		return nil // non-fatal
	}

	text, err := uc.extractor.Extract(ctx, doc)
	if err != nil {
		slog.Warn("enrich_extract_text_failed", "document_id", documentID, "error", err)
		return nil
	}

	classification, err := uc.classifier.Classify(ctx, text)
	if err != nil {
		slog.Warn("enrich_classify_failed", "document_id", documentID, "error", err)
		return nil
	}

	merged := mergeClassification(doc, classification)

	if err := uc.repo.SaveClassification(ctx, doc.ID, merged); err != nil {
		slog.Warn("enrich_save_classification_failed", "document_id", documentID, "error", err)
		return nil
	}

	payload := map[string]any{
		"category":    merged.Category,
		"subcategory": merged.Subcategory,
		"tags":        merged.Tags,
	}
	if err := uc.vectorDB.UpdateChunksPayload(ctx, doc.ID, payload); err != nil {
		slog.Warn("enrich_update_qdrant_failed", "document_id", documentID, "error", err)
	}

	slog.Info("enrich_completed", "document_id", documentID, "confidence", merged.Confidence)
	return nil
}

// mergeClassification merges LLM classification with existing deterministic metadata.
// Deterministic category wins if present; tags are unioned; LLM summary wins if present.
func mergeClassification(doc *domain.Document, llm domain.Classification) domain.Classification {
	merged := domain.Classification{
		Confidence: llm.Confidence,
	}

	// Category: deterministic wins.
	if doc.Category != "" {
		merged.Category = doc.Category
	} else {
		merged.Category = llm.Category
	}

	merged.Subcategory = llm.Subcategory

	// Summary: LLM wins.
	if llm.Summary != "" {
		merged.Summary = llm.Summary
	} else {
		merged.Summary = doc.Summary
	}

	// Tags: union.
	tagSet := make(map[string]struct{})
	for _, t := range doc.Tags {
		tagSet[t] = struct{}{}
	}
	for _, t := range llm.Tags {
		tagSet[t] = struct{}{}
	}
	merged.Tags = make([]string, 0, len(tagSet))
	for t := range tagSet {
		merged.Tags = append(merged.Tags, t)
	}

	return merged
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/usecase/ -run TestEnrichByID -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/usecase/enrich.go internal/core/usecase/enrich_test.go
git commit -m "feat(usecase): add EnrichDocumentUseCase with merge strategy for async LLM enrichment"
```

---

### Task 9: Update bootstrap wiring + worker to subscribe to enrich

**Files:**
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `cmd/worker/main.go`
- Modify: `internal/core/ports/inbound.go`

- [ ] **Step 1: Add EnrichUC to ports and App**

In `internal/core/ports/inbound.go`, add:

```go
// DocumentEnricher is the inbound contract for async document enrichment.
type DocumentEnricher interface {
	EnrichByID(ctx context.Context, documentID string) error
}
```

In `internal/bootstrap/bootstrap.go`, add `EnrichUC` field to `App`:

```go
EnrichUC ports.DocumentEnricher
```

- [ ] **Step 2: Wire EnrichUC in bootstrap.New**

In `internal/bootstrap/bootstrap.go`, after `processUC` creation, add:

```go
enrichUC := usecase.NewEnrichDocumentUseCase(repo, extractor, classifier, vectorDB)
```

Update `processUC` constructor call — replace `classifier` with `metadataExtractor` and add `queue`:

```go
metadataExtractor := metadata.New()
processUC := usecase.NewProcessDocumentUseCase(repo, extractor, metadataExtractor, chunker, embedder, vectorDB, queue)
```

Add import for metadata package:

```go
"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/extractor/metadata"
```

Add `EnrichUC: enrichUC` to returned `App`.

- [ ] **Step 3: Update worker to subscribe to enrich subject**

In `cmd/worker/main.go`, after the existing `SubscribeDocumentIngested` block (before `if err != nil` check), add a goroutine for enrich subscription:

```go
go func() {
	logger.Info("worker_enrich_subscribed")
	enrichErr := app.Queue.SubscribeDocumentEnrich(ctx, func(handlerCtx context.Context, documentID string) error {
		start := time.Now()
		logger.Info("document_enrichment_started", "document_id", documentID)

		enrichCtx, cancel := context.WithTimeout(handlerCtx, 3*time.Minute)
		defer cancel()

		err := app.EnrichUC.EnrichByID(enrichCtx, documentID)
		if err != nil {
			logger.Error("document_enrichment_failed",
				"document_id", documentID,
				"duration_ms", float64(time.Since(start).Microseconds())/1000.0,
				"error", err,
			)
			return err
		}

		logger.Info("document_enrichment_completed",
			"document_id", documentID,
			"duration_ms", float64(time.Since(start).Microseconds())/1000.0,
		)
		return nil
	})
	if enrichErr != nil {
		logger.Error("worker_enrich_subscribe_error", "error", enrichErr)
	}
}()
```

Note: `SubscribeDocumentEnrich` blocks on `<-ctx.Done()`, so it runs in a goroutine. The existing `SubscribeDocumentIngested` also blocks — move it into a goroutine too, and use a `select{}` or `<-ctx.Done()` in main to wait:

```go
go func() {
	logger.Info("worker_subscribed", "subject", cfg.NATSSubject)
	if err := app.Queue.SubscribeDocumentIngested(ctx, func(handlerCtx context.Context, documentID string) error {
		// ... existing handler code (unchanged) ...
	}); err != nil {
		logger.Error("worker_subscribe_error", "error", err)
	}
}()

go func() {
	logger.Info("worker_enrich_subscribed")
	if err := app.Queue.SubscribeDocumentEnrich(ctx, func(handlerCtx context.Context, documentID string) error {
		start := time.Now()
		logger.Info("document_enrichment_started", "document_id", documentID)

		enrichCtx, cancel := context.WithTimeout(handlerCtx, 3*time.Minute)
		defer cancel()

		if err := app.EnrichUC.EnrichByID(enrichCtx, documentID); err != nil {
			logger.Error("document_enrichment_failed",
				"document_id", documentID,
				"duration_ms", float64(time.Since(start).Microseconds())/1000.0,
				"error", err,
			)
			return err
		}
		logger.Info("document_enrichment_completed",
			"document_id", documentID,
			"duration_ms", float64(time.Since(start).Microseconds())/1000.0,
		)
		return nil
	}); err != nil {
		logger.Error("worker_enrich_subscribe_error", "error", err)
	}
}()

<-ctx.Done()
```

- [ ] **Step 4: Run vet and build**

Run: `go vet ./... && go build ./cmd/worker/ && go build ./cmd/api/`
Expected: PASS (no errors).

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/bootstrap/bootstrap.go cmd/worker/main.go internal/core/ports/inbound.go
git commit -m "feat(worker): wire EnrichDocumentUseCase and subscribe to enrich NATS subject"
```

---

### Task 10: Update existing tests and verify full pipeline

**Files:**
- Verify all existing tests pass with new signatures.

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 2: Run vet**

Run: `go vet ./...`
Expected: PASS

- [ ] **Step 3: Fix any remaining compilation issues**

If any tests or packages fail due to the interface changes (e.g., test fakes missing new methods), add the stub methods. Common fixes:

- Add `UpdateChunksPayload` stub to any `VectorStore` test fakes.
- Add `PublishDocumentEnrich` / `SubscribeDocumentEnrich` stubs to any `MessageQueue` test fakes.
- Remove references to `classifierFake` in `process_test.go` if any remain.

- [ ] **Step 4: Commit fixes if needed**

```bash
git add -A
git commit -m "fix: update test fakes for new VectorStore and MessageQueue interface methods"
```
