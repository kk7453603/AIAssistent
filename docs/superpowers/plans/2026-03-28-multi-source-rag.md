# Multi-Source Ready RAG — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Isolate source_types into separate Qdrant collections with cascading search, extend SearchFilter for multi-field filtering, and make chunking configurable per source.

**Architecture:** `MultiCollectionStore` wraps per-source `qdrant.Client` instances, routes writes by `doc.SourceType`, and performs cascading reads by priority. `ChunkerRegistry` maps source_type to chunker config. `SearchFilter` expanded to support source/tags/path filtering.

**Tech Stack:** Go, Qdrant REST API (multi-collection), env-based JSON config for per-source chunking.

**Spec:** `docs/superpowers/specs/2026-03-28-multi-source-rag.md`

---

### Task 1: Extend SearchFilter domain type

**Files:**
- Modify: `internal/core/domain/retrieval.go`

- [ ] **Step 1: Update SearchFilter**

In `internal/core/domain/retrieval.go`, replace:

```go
type SearchFilter struct {
	Category string
}
```

with:

```go
type SearchFilter struct {
	SourceTypes []string // filter by source_type (empty = all)
	Categories  []string // filter by category (empty = all)
	Tags        []string // any-match on tags (empty = all)
	PathPrefix  string   // prefix match on path
}
```

- [ ] **Step 2: Fix all callers that set `Category`**

Search for `filter.Category` and `Category:` in SearchFilter literals. Replace:
- `filter.Category != ""` → `len(filter.Categories) > 0`
- `SearchFilter{Category: "x"}` → `SearchFilter{Categories: []string{"x"}}`

Key files to update:
- `internal/infrastructure/vector/qdrant/client.go` — `buildCategoryFilter` → `buildFilter`
- `internal/core/usecase/query.go` or agent code — any place passing SearchFilter
- `internal/adapters/http/router.go` — parsing category from query params
- Test files with `SearchFilter{Category: ...}`

- [ ] **Step 3: Replace buildCategoryFilter with buildFilter**

In `internal/infrastructure/vector/qdrant/client.go`, replace `buildCategoryFilter` with:

```go
func buildFilter(filter domain.SearchFilter) map[string]any {
	var must []map[string]any

	if len(filter.SourceTypes) > 0 {
		must = append(must, map[string]any{
			"key":   "source_type",
			"match": map[string]any{"any": filter.SourceTypes},
		})
	}

	if len(filter.Categories) > 0 {
		must = append(must, map[string]any{
			"key":   "category",
			"match": map[string]any{"any": filter.Categories},
		})
	}

	if len(filter.Tags) > 0 {
		must = append(must, map[string]any{
			"key":   "tags",
			"match": map[string]any{"any": filter.Tags},
		})
	}

	if filter.PathPrefix != "" {
		must = append(must, map[string]any{
			"key":   "path",
			"match": map[string]any{"value": filter.PathPrefix},
		})
	}

	if len(must) == 0 {
		return nil
	}
	return map[string]any{"must": must}
}
```

Update `Search` and `SearchLexical` callers:

```go
if f := buildFilter(filter); f != nil {
	reqBody["filter"] = f
}
```

- [ ] **Step 4: Run tests and fix compilation**

Run: `go build ./... && go test ./... -count=1`

Fix any remaining references to `filter.Category` or `SearchFilter{Category: ...}`.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(domain): extend SearchFilter with SourceTypes, Categories, Tags, PathPrefix"
```

---

### Task 2: Extend VectorStore port — add sourceType to UpdateChunksPayload

**Files:**
- Modify: `internal/core/ports/outbound.go`
- Modify: `internal/infrastructure/vector/qdrant/client.go`
- Modify: `internal/core/usecase/enrich.go`
- Modify test fakes with `UpdateChunksPayload`

- [ ] **Step 1: Update VectorStore interface**

In `internal/core/ports/outbound.go`, change `UpdateChunksPayload` signature:

```go
UpdateChunksPayload(ctx context.Context, docID string, sourceType string, payload map[string]any) error
```

- [ ] **Step 2: Update qdrant.Client implementation**

In `internal/infrastructure/vector/qdrant/client.go`, update `UpdateChunksPayload` to accept `sourceType string` (for now ignore it — single-client doesn't need routing, but signature must match):

```go
func (c *Client) UpdateChunksPayload(ctx context.Context, docID string, sourceType string, payload map[string]any) error {
```

- [ ] **Step 3: Update EnrichDocumentUseCase caller**

In `internal/core/usecase/enrich.go`, change the call:

```go
if err := uc.vectorDB.UpdateChunksPayload(ctx, doc.ID, doc.SourceType, payload); err != nil {
```

- [ ] **Step 4: Update all test fakes**

Search for `UpdateChunksPayload` in test files and update signatures:

```go
func (f *vectorFake) UpdateChunksPayload(context.Context, string, string, map[string]any) error { return nil }
```

Key files: `process_test.go`, `query_test.go`, `enrich_test.go`, `router_openai_test.go`, `client_update_test.go`.

- [ ] **Step 5: Run tests**

Run: `go build ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(ports): add sourceType param to UpdateChunksPayload"
```

---

### Task 3: Add ChunkerRegistry port and implementation

**Files:**
- Modify: `internal/core/ports/outbound.go`
- Create: `internal/infrastructure/chunking/registry.go`
- Create: `internal/infrastructure/chunking/registry_test.go`

- [ ] **Step 1: Add ChunkerRegistry port**

In `internal/core/ports/outbound.go`, add:

```go
// ChunkerRegistry selects a Chunker based on source type.
type ChunkerRegistry interface {
	ForSource(sourceType string) Chunker
}
```

- [ ] **Step 2: Write registry tests**

Create `internal/infrastructure/chunking/registry_test.go`:

```go
package chunking

import (
	"testing"
)

func TestRegistry_ForSource_ReturnsRegistered(t *testing.T) {
	fallback := NewSplitter(900, 100)
	mdChunker := NewMarkdownSplitter(1200, 150)

	reg := NewRegistry(fallback)
	reg.Register("obsidian", mdChunker)

	got := reg.ForSource("obsidian")
	// Verify it returns the markdown splitter by checking it splits markdown differently
	text := "# Header\n\nContent under header.\n\n# Second\n\nMore content."
	chunks := got.Split(text)
	if len(chunks) < 2 {
		t.Errorf("expected markdown splitter (>=2 chunks for headers), got %d chunks", len(chunks))
	}
}

func TestRegistry_ForSource_ReturnsFallback(t *testing.T) {
	fallback := NewSplitter(900, 100)
	reg := NewRegistry(fallback)

	got := reg.ForSource("unknown")
	if got == nil {
		t.Fatal("expected fallback chunker, got nil")
	}
	chunks := got.Split("hello world")
	if len(chunks) != 1 || chunks[0] != "hello world" {
		t.Errorf("unexpected chunks from fallback: %v", chunks)
	}
}
```

Run: `go test ./internal/infrastructure/chunking/ -run TestRegistry -v`
Expected: FAIL

- [ ] **Step 3: Implement Registry**

Create `internal/infrastructure/chunking/registry.go`:

```go
package chunking

import "github.com/kirillkom/personal-ai-assistant/internal/core/ports"

// Registry selects a Chunker based on source type, falling back to a default.
type Registry struct {
	chunkers map[string]ports.Chunker
	fallback ports.Chunker
}

func NewRegistry(fallback ports.Chunker) *Registry {
	return &Registry{
		chunkers: make(map[string]ports.Chunker),
		fallback: fallback,
	}
}

func (r *Registry) Register(sourceType string, chunker ports.Chunker) {
	r.chunkers[sourceType] = chunker
}

func (r *Registry) ForSource(sourceType string) ports.Chunker {
	if c, ok := r.chunkers[sourceType]; ok {
		return c
	}
	return r.fallback
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/infrastructure/chunking/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/ports/outbound.go internal/infrastructure/chunking/registry.go internal/infrastructure/chunking/registry_test.go
git commit -m "feat(chunking): add ChunkerRegistry port and Registry implementation"
```

---

### Task 4: Wire ChunkerRegistry into ProcessDocumentUseCase

**Files:**
- Modify: `internal/core/usecase/process.go`
- Modify: `internal/core/usecase/process_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/bootstrap/bootstrap.go`

- [ ] **Step 1: Add ChunkConfig to config**

In `internal/config/config.go`, add field to Config struct:

```go
ChunkConfig string // JSON: {"obsidian":{"strategy":"markdown","chunk_size":1200,"overlap":150}}
```

Add to `Load()`:

```go
ChunkConfig: os.Getenv("CHUNK_CONFIG"),
```

Add domain type in `internal/core/domain/source.go`:

```go
type ChunkConfig struct {
	Strategy string `json:"strategy"`
	Size     int    `json:"chunk_size"`
	Overlap  int    `json:"overlap"`
}
```

Add parser in `internal/config/config.go`:

```go
func ParseChunkConfig(raw string) map[string]domain.ChunkConfig {
	if raw == "" {
		return nil
	}
	var result map[string]domain.ChunkConfig
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}
```

Add `"encoding/json"` and domain import to config.go.

- [ ] **Step 2: Update ProcessDocumentUseCase**

In `internal/core/usecase/process.go`, replace `chunker ports.Chunker` with `chunkers ports.ChunkerRegistry`:

```go
type ProcessDocumentUseCase struct {
	repo          ports.DocumentRepository
	extractor     ports.TextExtractor
	metaExtractor ports.MetadataExtractor
	chunkers      ports.ChunkerRegistry
	embedder      ports.Embedder
	vectorDB      ports.VectorStore
	queue         ports.MessageQueue
}
```

Update constructor:

```go
func NewProcessDocumentUseCase(
	repo ports.DocumentRepository,
	extractor ports.TextExtractor,
	metaExtractor ports.MetadataExtractor,
	chunkers ports.ChunkerRegistry,
	embedder ports.Embedder,
	vectorDB ports.VectorStore,
	queue ports.MessageQueue,
) *ProcessDocumentUseCase {
```

Update `chunk` method to use source-aware chunker:

```go
func (uc *ProcessDocumentUseCase) chunk(_ context.Context, text string, sourceType string) ([]string, error) {
	chunker := uc.chunkers.ForSource(sourceType)
	chunks := chunker.Split(text)
```

Update call in `processPipeline`:

```go
chunks, err := uc.chunk(ctx, text, meta.SourceType)
```

- [ ] **Step 3: Update process_test.go**

Add `chunkerRegistryFake`:

```go
type chunkerRegistryFake struct {
	chunks []string
}

func (f *chunkerRegistryFake) ForSource(string) ports.Chunker {
	return &chunkerFake{chunks: f.chunks}
}
```

Update all `NewProcessDocumentUseCase` calls to use `&chunkerRegistryFake{chunks: ...}` instead of `&chunkerFake{chunks: ...}`.

- [ ] **Step 4: Update bootstrap wiring**

In `internal/bootstrap/bootstrap.go`, replace single chunker with registry:

```go
// Build chunker registry.
defaultChunker := ports.Chunker(chunking.NewSplitter(cfg.ChunkSize, cfg.ChunkOverlap))
switch chunkStrategy {
case "markdown", "md":
	defaultChunker = chunking.NewMarkdownSplitter(cfg.ChunkSize, cfg.ChunkOverlap)
}

chunkerRegistry := chunking.NewRegistry(defaultChunker)
if chunkConfigs := config.ParseChunkConfig(cfg.ChunkConfig); chunkConfigs != nil {
	for sourceType, cc := range chunkConfigs {
		size := cc.Size
		if size <= 0 {
			size = cfg.ChunkSize
		}
		overlap := cc.Overlap
		if overlap < 0 {
			overlap = cfg.ChunkOverlap
		}
		switch strings.ToLower(cc.Strategy) {
		case "markdown", "md":
			chunkerRegistry.Register(sourceType, chunking.NewMarkdownSplitter(size, overlap))
		default:
			chunkerRegistry.Register(sourceType, chunking.NewSplitter(size, overlap))
		}
	}
}

processUC := usecase.NewProcessDocumentUseCase(repo, extractor, metaExtractor, chunkerRegistry, embedder, vectorDB, queue)
```

Remove old `var chunker ports.Chunker` and its switch.

- [ ] **Step 5: Run tests**

Run: `go build ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(chunking): per-source chunking via ChunkerRegistry and CHUNK_CONFIG"
```

---

### Task 5: Implement MultiCollectionStore

**Files:**
- Create: `internal/infrastructure/vector/qdrant/multi_collection.go`
- Create: `internal/infrastructure/vector/qdrant/multi_collection_test.go`

- [ ] **Step 1: Write tests**

Create `internal/infrastructure/vector/qdrant/multi_collection_test.go`:

```go
package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestMultiCollectionStore_IndexChunksRoutesToCorrectCollection(t *testing.T) {
	var mu sync.Mutex
	requestedURLs := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestedURLs = append(requestedURLs, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":{"status":"completed"}}`))
	}))
	defer server.Close()

	store := NewMultiCollectionStore(server.URL, "docs", []string{"upload", "web"}, []string{"upload", "web"}, Options{})

	doc := &domain.Document{ID: "d1", SourceType: "web", Filename: "page.html", Tags: []string{}}
	err := store.IndexChunks(context.Background(), doc, []string{"chunk1"}, [][]float32{{0.1, 0.2}})
	if err != nil {
		t.Fatalf("IndexChunks() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, u := range requestedURLs {
		if u == "/collections/docs_web/points" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected request to docs_web collection, got URLs: %v", requestedURLs)
	}
}

func TestMultiCollectionStore_SearchCascadesInOrder(t *testing.T) {
	callOrder := []string{}
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callOrder = append(callOrder, r.URL.Path)
		mu.Unlock()

		// Return one result per collection
		result := map[string]any{
			"result": map[string]any{
				"points": []map[string]any{
					{
						"score": 0.9,
						"payload": map[string]any{
							"doc_id": "d1", "filename": "f.txt", "category": "", "chunk_index": 0, "text": "chunk",
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	store := NewMultiCollectionStore(server.URL, "docs", []string{"obsidian", "upload", "web"}, []string{"obsidian", "upload", "web"}, Options{})

	// Search with limit=1 — should stop after first collection returns enough
	results, err := store.Search(context.Background(), []float32{0.1, 0.2}, 1, domain.SearchFilter{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}

	mu.Lock()
	defer mu.Unlock()
	// First query should be to obsidian (highest priority)
	if len(callOrder) == 0 {
		t.Fatal("expected at least one query")
	}
	// With early stop and limit=1, only the first collection should be queried
	queryCalls := 0
	for _, u := range callOrder {
		if u == "/collections/docs_obsidian/points/query" {
			queryCalls++
		}
	}
	if queryCalls == 0 {
		t.Errorf("expected query to docs_obsidian, got: %v", callOrder)
	}
}
```

Run: `go test ./internal/infrastructure/vector/qdrant/ -run TestMultiCollection -v`
Expected: FAIL

- [ ] **Step 2: Implement MultiCollectionStore**

Create `internal/infrastructure/vector/qdrant/multi_collection.go`:

```go
package qdrant

import (
	"context"
	"fmt"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// MultiCollectionStore routes operations to per-source Qdrant collections
// and performs cascading search across collections in priority order.
type MultiCollectionStore struct {
	clients     map[string]*Client // source_type → Client
	searchOrder []string           // priority for cascading search
}

func NewMultiCollectionStore(
	baseURL string,
	baseCollection string,
	sourceTypes []string,
	searchOrder []string,
	options Options,
) *MultiCollectionStore {
	clients := make(map[string]*Client, len(sourceTypes))
	for _, st := range sourceTypes {
		collName := fmt.Sprintf("%s_%s", baseCollection, st)
		clients[st] = NewWithOptions(baseURL, collName, options)
	}
	return &MultiCollectionStore{
		clients:     clients,
		searchOrder: searchOrder,
	}
}

func (m *MultiCollectionStore) EnsureCollections(ctx context.Context, vectorSize int) error {
	for _, c := range m.clients {
		if err := c.EnsureCollection(ctx, vectorSize); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiCollectionStore) IndexChunks(ctx context.Context, doc *domain.Document, chunks []string, vectors [][]float32) error {
	client, ok := m.clients[doc.SourceType]
	if !ok {
		return fmt.Errorf("no collection for source_type %q", doc.SourceType)
	}
	return client.IndexChunks(ctx, doc, chunks, vectors)
}

func (m *MultiCollectionStore) Search(ctx context.Context, queryVector []float32, limit int, filter domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	return m.cascadeSearch(func(client *Client) ([]domain.RetrievedChunk, error) {
		return client.Search(ctx, queryVector, limit, filter)
	}, limit, filter.SourceTypes)
}

func (m *MultiCollectionStore) SearchLexical(ctx context.Context, queryText string, limit int, filter domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	return m.cascadeSearch(func(client *Client) ([]domain.RetrievedChunk, error) {
		return client.SearchLexical(ctx, queryText, limit, filter)
	}, limit, filter.SourceTypes)
}

func (m *MultiCollectionStore) UpdateChunksPayload(ctx context.Context, docID string, sourceType string, payload map[string]any) error {
	client, ok := m.clients[sourceType]
	if !ok {
		return fmt.Errorf("no collection for source_type %q", sourceType)
	}
	return client.UpdateChunksPayload(ctx, docID, sourceType, payload)
}

func (m *MultiCollectionStore) cascadeSearch(
	searchFn func(*Client) ([]domain.RetrievedChunk, error),
	limit int,
	sourceTypeFilter []string,
) ([]domain.RetrievedChunk, error) {
	allowed := make(map[string]bool, len(sourceTypeFilter))
	for _, st := range sourceTypeFilter {
		allowed[st] = true
	}

	var results []domain.RetrievedChunk
	for _, st := range m.searchOrder {
		if len(allowed) > 0 && !allowed[st] {
			continue
		}
		client, ok := m.clients[st]
		if !ok {
			continue
		}

		chunks, err := searchFn(client)
		if err != nil {
			continue // skip failed collection, try next
		}
		results = append(results, chunks...)

		if len(results) >= limit {
			results = results[:limit]
			break // early stop
		}
	}
	return results, nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/infrastructure/vector/qdrant/ -run TestMultiCollection -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/vector/qdrant/multi_collection.go internal/infrastructure/vector/qdrant/multi_collection_test.go
git commit -m "feat(qdrant): MultiCollectionStore with cascading search"
```

---

### Task 6: Add config + wire MultiCollectionStore in bootstrap

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/bootstrap/bootstrap.go`

- [ ] **Step 1: Add search order config**

In `internal/config/config.go`, add to Config:

```go
QdrantSearchOrder string // comma-separated: "obsidian,upload,web"
```

Add to `Load()`:

```go
QdrantSearchOrder: getEnvDefault("QDRANT_SEARCH_ORDER", "upload,web,obsidian"),
```

- [ ] **Step 2: Wire MultiCollectionStore in bootstrap**

In `internal/bootstrap/bootstrap.go`, replace single `qdrant.Client` for documents with `MultiCollectionStore`:

```go
// Parse source types from search order.
searchOrderStr := strings.TrimSpace(cfg.QdrantSearchOrder)
var searchOrder []string
for _, s := range strings.Split(searchOrderStr, ",") {
	s = strings.TrimSpace(s)
	if s != "" {
		searchOrder = append(searchOrder, s)
	}
}
if len(searchOrder) == 0 {
	searchOrder = []string{"upload"}
}

vectorDB := qdrant.NewMultiCollectionStore(cfg.QdrantURL, cfg.QdrantCollection, searchOrder, searchOrder, qdrant.Options{
	ResilienceExecutor: resilienceExecutor,
})

if cfg.QdrantEmbedDim > 0 {
	if err := vectorDB.EnsureCollections(ctx, cfg.QdrantEmbedDim); err != nil {
		return nil, fmt.Errorf("ensure qdrant document collections: %w", err)
	}
	// Memory collection stays as single collection.
	if err := memoryVector.EnsureCollection(ctx, cfg.QdrantEmbedDim); err != nil {
		return nil, fmt.Errorf("ensure qdrant memory collection: %w", err)
	}
}
```

Note: `vectorDB` type changes from `*qdrant.Client` to `*qdrant.MultiCollectionStore`. Both satisfy `ports.VectorStore`.

- [ ] **Step 3: Run build and tests**

Run: `go build ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go internal/bootstrap/bootstrap.go
git commit -m "feat(bootstrap): wire MultiCollectionStore with configurable search order"
```

---

### Task 7: Final verification

**Files:** None (verification only).

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1 -v 2>&1 | grep -E "FAIL|ok|--- FAIL"`
Expected: All PASS, no FAIL.

- [ ] **Step 2: Run vet**

Run: `go vet ./...`
Expected: Clean.

- [ ] **Step 3: Run build**

Run: `go build ./cmd/api/ && go build ./cmd/worker/`
Expected: Clean.

- [ ] **Step 4: Verify config defaults**

Check `.env.example` has new vars documented. If it exists, add:

```
QDRANT_SEARCH_ORDER=obsidian,upload,web
CHUNK_CONFIG=
```
