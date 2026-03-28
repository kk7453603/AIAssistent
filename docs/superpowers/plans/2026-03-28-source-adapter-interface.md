# SourceAdapter/Ingestor Interface — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce `SourceAdapter` interface to decouple ingest from HTTP upload and implement a working `WebAdapter` for URL-based document ingestion.

**Architecture:** `SourceAdapter` normalizes content from any source into `IngestResult`. `IngestDocumentUseCase` selects adapter by `source_type`, calls `Ingest()`, then saves to storage → creates Document → publishes to NATS. `Upload()` becomes a thin wrapper around `IngestFromSource()`.

**Tech Stack:** Go, `golang.org/x/net/html` for HTML text extraction, `net/http` for web fetching.

**Spec:** `docs/superpowers/specs/2026-03-28-source-adapter-interface.md`

---

### Task 1: Add domain types — SourceRequest + IngestResult

**Files:**
- Create: `internal/core/domain/source.go`
- Create: `internal/core/domain/source_test.go`

- [ ] **Step 1: Write test**

Create `internal/core/domain/source_test.go`:

```go
package domain

import (
	"strings"
	"testing"
)

func TestSourceRequestDefaults(t *testing.T) {
	req := SourceRequest{SourceType: "upload", Filename: "test.txt"}
	if req.SourceType != "upload" {
		t.Fatalf("expected upload, got %q", req.SourceType)
	}
}

func TestIngestResultBodyReadable(t *testing.T) {
	body := strings.NewReader("hello")
	result := IngestResult{
		Filename:   "test.txt",
		MimeType:   "text/plain",
		Body:       body,
		SourceType: "upload",
		Path:       "test.txt",
	}
	if result.Body == nil {
		t.Fatal("expected body to be set")
	}
}
```

Run: `go test ./internal/core/domain/ -run TestSourceRequest -v`
Expected: FAIL — types not defined.

- [ ] **Step 2: Implement domain types**

Create `internal/core/domain/source.go`:

```go
package domain

import "io"

// SourceRequest carries input from any ingest initiator.
type SourceRequest struct {
	SourceType string            // "upload", "obsidian", "web"
	Filename   string            // original filename or path
	MimeType   string            // MIME type if known
	Body       io.Reader         // content (for upload/file sources)
	URL        string            // URL (for web scraping)
	VaultID    string            // vault ID (for Obsidian)
	Path       string            // path in vault/fs
	Meta       map[string]string // arbitrary source metadata
}

// IngestResult is the normalized output from a SourceAdapter.
type IngestResult struct {
	Filename   string
	MimeType   string
	Body       io.Reader
	SourceType string
	Path       string
	ExtraMeta  map[string]string
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/domain/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/domain/source.go internal/core/domain/source_test.go
git commit -m "feat(domain): add SourceRequest and IngestResult types"
```

---

### Task 2: Add SourceAdapter port

**Files:**
- Modify: `internal/core/ports/outbound.go`

- [ ] **Step 1: Add SourceAdapter interface**

Add at the end of `internal/core/ports/outbound.go`:

```go
// SourceAdapter normalizes content from any source into an ingestable document.
type SourceAdapter interface {
	Ingest(ctx context.Context, req domain.SourceRequest) (*domain.IngestResult, error)
	SourceType() string
}
```

- [ ] **Step 2: Extend DocumentIngestor inbound port**

In `internal/core/ports/inbound.go`, add `IngestFromSource` to `DocumentIngestor`:

```go
type DocumentIngestor interface {
	Upload(ctx context.Context, filename, mimeType string, body io.Reader) (*domain.Document, error)
	IngestFromSource(ctx context.Context, req domain.SourceRequest) (*domain.Document, error)
}
```

- [ ] **Step 3: Run vet**

Run: `go vet ./internal/core/ports/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/ports/outbound.go internal/core/ports/inbound.go
git commit -m "feat(ports): add SourceAdapter interface and extend DocumentIngestor"
```

---

### Task 3: Implement UploadAdapter

**Files:**
- Create: `internal/infrastructure/source/upload/adapter.go`
- Create: `internal/infrastructure/source/upload/adapter_test.go`

- [ ] **Step 1: Write test**

Create `internal/infrastructure/source/upload/adapter_test.go`:

```go
package upload

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestUploadAdapter_SourceType(t *testing.T) {
	a := New()
	if a.SourceType() != "upload" {
		t.Fatalf("expected 'upload', got %q", a.SourceType())
	}
}

func TestUploadAdapter_Ingest_Success(t *testing.T) {
	a := New()
	req := domain.SourceRequest{
		SourceType: "upload",
		Filename:   "report.txt",
		MimeType:   "text/plain",
		Body:       bytes.NewBufferString("hello world"),
	}

	result, err := a.Ingest(context.Background(), req)
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if result.Filename != "report.txt" {
		t.Errorf("Filename = %q, want %q", result.Filename, "report.txt")
	}
	if result.MimeType != "text/plain" {
		t.Errorf("MimeType = %q, want %q", result.MimeType, "text/plain")
	}
	if result.SourceType != "upload" {
		t.Errorf("SourceType = %q, want %q", result.SourceType, "upload")
	}
	body, _ := io.ReadAll(result.Body)
	if string(body) != "hello world" {
		t.Errorf("Body = %q, want %q", string(body), "hello world")
	}
}

func TestUploadAdapter_Ingest_NilBody(t *testing.T) {
	a := New()
	req := domain.SourceRequest{
		SourceType: "upload",
		Filename:   "report.txt",
	}

	_, err := a.Ingest(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nil body")
	}
}
```

Run: `go test ./internal/infrastructure/source/upload/ -v`
Expected: FAIL

- [ ] **Step 2: Implement UploadAdapter**

Create `internal/infrastructure/source/upload/adapter.go`:

```go
package upload

import (
	"context"
	"errors"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// Adapter handles direct file uploads (HTTP multipart).
type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) SourceType() string { return "upload" }

func (a *Adapter) Ingest(_ context.Context, req domain.SourceRequest) (*domain.IngestResult, error) {
	if req.Body == nil {
		return nil, errors.New("upload: body is required")
	}
	return &domain.IngestResult{
		Filename:   req.Filename,
		MimeType:   req.MimeType,
		Body:       req.Body,
		SourceType: "upload",
		Path:       req.Filename,
	}, nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/infrastructure/source/upload/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/source/upload/
git commit -m "feat(source): implement UploadAdapter"
```

---

### Task 4: Implement WebAdapter — HTML text extraction

**Files:**
- Create: `internal/infrastructure/source/web/html.go`
- Create: `internal/infrastructure/source/web/html_test.go`

- [ ] **Step 1: Write HTML extraction tests**

Create `internal/infrastructure/source/web/html_test.go`:

```go
package web

import "testing"

func TestExtractTextFromHTML(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "simple page",
			html:      `<html><head><title>My Page</title></head><body><h1>Hello</h1><p>World</p></body></html>`,
			wantTitle: "My Page",
			wantBody:  "Hello\nWorld",
		},
		{
			name:      "strips scripts and styles",
			html:      `<html><head><style>body{}</style></head><body><script>alert(1)</script><p>Content</p></body></html>`,
			wantTitle: "",
			wantBody:  "Content",
		},
		{
			name:      "preserves whitespace sanely",
			html:      `<p>Line one</p><p>Line two</p>`,
			wantTitle: "",
			wantBody:  "Line one\nLine two",
		},
		{
			name:      "empty html",
			html:      ``,
			wantTitle: "",
			wantBody:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, body := extractTextFromHTML(tt.html)
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}
```

Run: `go test ./internal/infrastructure/source/web/ -run TestExtractTextFromHTML -v`
Expected: FAIL

- [ ] **Step 2: Implement HTML text extraction**

Create `internal/infrastructure/source/web/html.go`:

```go
package web

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// extractTextFromHTML parses HTML and returns (title, body text).
// Strips <script>, <style>, <noscript> content. Adds newlines between block elements.
func extractTextFromHTML(s string) (string, string) {
	if s == "" {
		return "", ""
	}
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return "", s
	}

	var title string
	var body strings.Builder
	var inTitle bool

	skipTags := map[atom.Atom]bool{
		atom.Script:   true,
		atom.Style:    true,
		atom.Noscript: true,
	}

	blockTags := map[atom.Atom]bool{
		atom.P: true, atom.Div: true, atom.H1: true, atom.H2: true,
		atom.H3: true, atom.H4: true, atom.H5: true, atom.H6: true,
		atom.Li: true, atom.Br: true, atom.Blockquote: true,
		atom.Pre: true, atom.Article: true, atom.Section: true,
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if skipTags[n.DataAtom] {
				return
			}
			if n.DataAtom == atom.Title {
				inTitle = true
			}
			if blockTags[n.DataAtom] && body.Len() > 0 {
				body.WriteString("\n")
			}
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if inTitle {
					title = text
				} else {
					if body.Len() > 0 {
						last := body.String()
						if !strings.HasSuffix(last, "\n") {
							body.WriteString(" ")
						}
					}
					body.WriteString(text)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		if n.Type == html.ElementNode && n.DataAtom == atom.Title {
			inTitle = false
		}
	}

	walk(doc)

	// Clean up multiple newlines and trim.
	result := body.String()
	for strings.Contains(result, "\n\n") {
		result = strings.ReplaceAll(result, "\n\n", "\n")
	}
	result = strings.TrimSpace(result)

	return title, result
}
```

- [ ] **Step 3: Add golang.org/x/net dependency if missing**

Run: `go get golang.org/x/net`

- [ ] **Step 4: Run tests**

Run: `go test ./internal/infrastructure/source/web/ -run TestExtractTextFromHTML -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/source/web/html.go internal/infrastructure/source/web/html_test.go
git commit -m "feat(source/web): HTML text extraction with title and body"
```

---

### Task 5: Implement WebAdapter

**Files:**
- Create: `internal/infrastructure/source/web/adapter.go`
- Create: `internal/infrastructure/source/web/adapter_test.go`

- [ ] **Step 1: Write adapter tests**

Create `internal/infrastructure/source/web/adapter_test.go`:

```go
package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestWebAdapter_SourceType(t *testing.T) {
	a := New(nil)
	if a.SourceType() != "web" {
		t.Fatalf("expected 'web', got %q", a.SourceType())
	}
}

func TestWebAdapter_Ingest_HTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Test Page</title></head><body><p>Hello world</p></body></html>`))
	}))
	defer server.Close()

	a := New(server.Client())
	result, err := a.Ingest(context.Background(), domain.SourceRequest{
		SourceType: "web",
		URL:        server.URL + "/page.html",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if result.SourceType != "web" {
		t.Errorf("SourceType = %q, want %q", result.SourceType, "web")
	}
	if result.Filename != "Test Page" {
		t.Errorf("Filename = %q, want %q", result.Filename, "Test Page")
	}
	if result.Path != server.URL+"/page.html" {
		t.Errorf("Path = %q, want %q", result.Path, server.URL+"/page.html")
	}
	body, _ := io.ReadAll(result.Body)
	if string(body) == "" {
		t.Error("expected non-empty body")
	}
}

func TestWebAdapter_Ingest_Markdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte("# Title\n\nSome content"))
	}))
	defer server.Close()

	a := New(server.Client())
	result, err := a.Ingest(context.Background(), domain.SourceRequest{
		SourceType: "web",
		URL:        server.URL + "/doc.md",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	body, _ := io.ReadAll(result.Body)
	if string(body) != "# Title\n\nSome content" {
		t.Errorf("Body = %q, want markdown passthrough", string(body))
	}
}

func TestWebAdapter_Ingest_EmptyURL(t *testing.T) {
	a := New(nil)
	_, err := a.Ingest(context.Background(), domain.SourceRequest{SourceType: "web"})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestWebAdapter_Ingest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := New(server.Client())
	_, err := a.Ingest(context.Background(), domain.SourceRequest{
		SourceType: "web",
		URL:        server.URL,
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
```

Run: `go test ./internal/infrastructure/source/web/ -run TestWebAdapter -v`
Expected: FAIL

- [ ] **Step 2: Implement WebAdapter**

Create `internal/infrastructure/source/web/adapter.go`:

```go
package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// Adapter fetches content from a URL and normalizes it for ingestion.
type Adapter struct {
	client *http.Client
}

func New(client *http.Client) *Adapter {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Adapter{client: client}
}

func (a *Adapter) SourceType() string { return "web" }

func (a *Adapter) Ingest(ctx context.Context, req domain.SourceRequest) (*domain.IngestResult, error) {
	if req.URL == "" {
		return nil, errors.New("web: url is required")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("web: create request: %w", err)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("web: fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("web: fetch returned status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("web: read response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(raw)
	filename := filenameFromURL(req.URL)
	mimeType := contentType

	// If HTML, extract text; if markdown or plain text, pass through.
	if isHTML(contentType) {
		title, body := extractTextFromHTML(content)
		content = body
		if title != "" {
			filename = title
		}
		mimeType = "text/plain"
	}

	return &domain.IngestResult{
		Filename:   filename,
		MimeType:   mimeType,
		Body:       strings.NewReader(content),
		SourceType: "web",
		Path:       req.URL,
	}, nil
}

func isHTML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

func filenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "web_document"
	}
	base := path.Base(u.Path)
	if base == "" || base == "/" || base == "." {
		return u.Host
	}
	return base
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/infrastructure/source/web/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/source/web/adapter.go internal/infrastructure/source/web/adapter_test.go
git commit -m "feat(source/web): WebAdapter with URL fetch and HTML text extraction"
```

---

### Task 6: Implement ObsidianAdapter stub

**Files:**
- Create: `internal/infrastructure/source/obsidian/adapter.go`

- [ ] **Step 1: Create stub**

Create `internal/infrastructure/source/obsidian/adapter.go`:

```go
package obsidian

import (
	"context"
	"errors"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// Adapter is a stub for Obsidian vault sync. Will be implemented when vault sync is built.
type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) SourceType() string { return "obsidian" }

func (a *Adapter) Ingest(_ context.Context, _ domain.SourceRequest) (*domain.IngestResult, error) {
	return nil, errors.New("obsidian source adapter not implemented")
}
```

- [ ] **Step 2: Run vet**

Run: `go vet ./internal/infrastructure/source/obsidian/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/source/obsidian/adapter.go
git commit -m "feat(source/obsidian): stub adapter for future vault sync"
```

---

### Task 7: Refactor IngestDocumentUseCase — adapter-based IngestFromSource

**Files:**
- Modify: `internal/core/usecase/ingest.go`
- Modify: `internal/core/usecase/ingest_test.go`

- [ ] **Step 1: Update ingest_test.go — add adapter fake and IngestFromSource tests**

Add to `internal/core/usecase/ingest_test.go`:

```go
type sourceAdapterFake struct {
	result *domain.IngestResult
	err    error
}

func (f *sourceAdapterFake) SourceType() string { return "fake" }
func (f *sourceAdapterFake) Ingest(context.Context, domain.SourceRequest) (*domain.IngestResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}
```

Add test:

```go
func TestIngestFromSourceSuccess(t *testing.T) {
	repo := &ingestRepoFake{}
	storage := &ingestStorageFake{}
	queue := &ingestQueueFake{}
	adapter := &sourceAdapterFake{
		result: &domain.IngestResult{
			Filename:   "page.html",
			MimeType:   "text/plain",
			Body:       bytes.NewBufferString("extracted text"),
			SourceType: "web",
			Path:       "https://example.com/page.html",
		},
	}
	adapters := map[string]ports.SourceAdapter{
		"web": adapter,
	}
	uc := NewIngestDocumentUseCase(repo, storage, queue, adapters)

	doc, err := uc.IngestFromSource(context.Background(), domain.SourceRequest{
		SourceType: "web",
		URL:        "https://example.com/page.html",
	})
	if err != nil {
		t.Fatalf("IngestFromSource() error = %v", err)
	}
	if doc.SourceType != "web" {
		t.Errorf("SourceType = %q, want %q", doc.SourceType, "web")
	}
	if doc.Path != "https://example.com/page.html" {
		t.Errorf("Path = %q, want %q", doc.Path, "https://example.com/page.html")
	}
	if storage.savedBody != "extracted text" {
		t.Errorf("saved body = %q, want %q", storage.savedBody, "extracted text")
	}
	if queue.documentID != doc.ID {
		t.Errorf("queued doc id = %q, want %q", queue.documentID, doc.ID)
	}
}

func TestIngestFromSourceUnknownAdapter(t *testing.T) {
	uc := NewIngestDocumentUseCase(&ingestRepoFake{}, &ingestStorageFake{}, &ingestQueueFake{}, nil)

	_, err := uc.IngestFromSource(context.Background(), domain.SourceRequest{
		SourceType: "unknown",
	})
	if err == nil {
		t.Fatal("expected error for unknown source type")
	}
}
```

Add `ports` import:

```go
"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
```

Update existing tests to pass `nil` as adapters map (backward compat):

Change both `NewIngestDocumentUseCase(repo, storage, queue)` calls to `NewIngestDocumentUseCase(repo, storage, queue, nil)`.

Run: `go test ./internal/core/usecase/ -run TestIngest -v`
Expected: FAIL — constructor signature mismatch.

- [ ] **Step 2: Refactor IngestDocumentUseCase**

In `internal/core/usecase/ingest.go`:

```go
type IngestDocumentUseCase struct {
	repo     ports.DocumentRepository
	storage  ports.ObjectStorage
	queue    ports.MessageQueue
	adapters map[string]ports.SourceAdapter
}

func NewIngestDocumentUseCase(
	repo ports.DocumentRepository,
	storage ports.ObjectStorage,
	queue ports.MessageQueue,
	adapters map[string]ports.SourceAdapter,
) *IngestDocumentUseCase {
	return &IngestDocumentUseCase{
		repo:     repo,
		storage:  storage,
		queue:    queue,
		adapters: adapters,
	}
}

func (uc *IngestDocumentUseCase) Upload(
	ctx context.Context,
	filename, mimeType string,
	body io.Reader,
) (*domain.Document, error) {
	return uc.IngestFromSource(ctx, domain.SourceRequest{
		SourceType: "upload",
		Filename:   filename,
		MimeType:   mimeType,
		Body:       body,
	})
}

func (uc *IngestDocumentUseCase) IngestFromSource(
	ctx context.Context,
	req domain.SourceRequest,
) (*domain.Document, error) {
	adapter, ok := uc.adapters[req.SourceType]
	if !ok {
		return nil, fmt.Errorf("unknown source type: %q", req.SourceType)
	}

	result, err := adapter.Ingest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("source adapter %q: %w", req.SourceType, err)
	}

	id := uuid.NewString()
	storageKey := fmt.Sprintf("%s_%s", id, sanitizeFilename(result.Filename))
	now := time.Now().UTC()

	if err := uc.storage.Save(ctx, storageKey, result.Body); err != nil {
		return nil, fmt.Errorf("save to object storage: %w", err)
	}

	doc := &domain.Document{
		ID:          id,
		Filename:    result.Filename,
		MimeType:    result.MimeType,
		StoragePath: storageKey,
		SourceType:  result.SourceType,
		Path:        result.Path,
		Status:      domain.StatusUploaded,
		Tags:        []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := uc.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("create document metadata: %w", err)
	}

	if err := uc.queue.PublishDocumentIngested(ctx, doc.ID); err != nil {
		return nil, fmt.Errorf("publish ingestion event: %w", err)
	}

	return doc, nil
}
```

Remove the old `Upload` method body (it's now a wrapper). Keep `sanitizeFilename` as is.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/usecase/ -run TestIngest -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/usecase/ingest.go internal/core/usecase/ingest_test.go
git commit -m "refactor(ingest): adapter-based IngestFromSource with Upload as wrapper"
```

---

### Task 8: Update bootstrap wiring + verify full build

**Files:**
- Modify: `internal/bootstrap/bootstrap.go`

- [ ] **Step 1: Wire adapters in bootstrap**

In `internal/bootstrap/bootstrap.go`, add imports:

```go
"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/source/obsidian"
sourceupload "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/source/upload"
sourceweb "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/source/web"
```

Before `ingestUC` creation, build adapters map:

```go
sourceAdapters := map[string]ports.SourceAdapter{
	"upload":   sourceupload.New(),
	"obsidian": obsidian.New(),
	"web":      sourceweb.New(nil),
}
ingestUC := usecase.NewIngestDocumentUseCase(repo, storage, queue, sourceAdapters)
```

Note: use aliased imports to avoid collision with existing `obsidian` if any. The obsidian source package is `internal/infrastructure/source/obsidian`, distinct from any existing obsidian packages.

- [ ] **Step 2: Run full build and tests**

Run: `go build ./... && go test ./... -count=1`
Expected: all PASS, no compilation errors.

- [ ] **Step 3: Commit**

```bash
git add internal/bootstrap/bootstrap.go
git commit -m "feat(bootstrap): wire SourceAdapters for upload, web, and obsidian stub"
```
