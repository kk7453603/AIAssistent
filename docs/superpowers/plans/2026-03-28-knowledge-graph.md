# Knowledge Graph (Foundation) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Neo4j-backed knowledge graph that extracts document links (wikilinks, shared tags, semantic similarity), boosts retrieval with graph traversal, rewrites queries using graph context, and exposes a graph API.

**Architecture:** `GraphStore` port with Neo4j implementation. `LinkExtractor` extracts wikilinks/markdown links from text. `ProcessDocumentUseCase` creates graph nodes + links at index time. `QueryUseCase` boosts results with graph-related documents. `AgentChatUseCase` rewrites queries using graph context. New `GET /v1/graph` endpoint.

**Tech Stack:** Go, Neo4j 5 Community (Docker), `github.com/neo4j/neo4j-go-driver/v5`.

**Spec:** `docs/superpowers/specs/2026-03-28-knowledge-graph.md`

---

### Task 1: Domain types for graph

**Files:**
- Create: `internal/core/domain/graph.go`

- [ ] **Step 1: Create graph domain types**

Create `internal/core/domain/graph.go`:

```go
package domain

// GraphNode represents a document node in the knowledge graph.
type GraphNode struct {
	ID         string `json:"id"`
	Filename   string `json:"filename"`
	SourceType string `json:"source_type"`
	Category   string `json:"category"`
	Title      string `json:"title"`
	Path       string `json:"path"`
}

// GraphRelation represents an edge between two documents.
type GraphRelation struct {
	SourceID string  `json:"source_id"`
	TargetID string  `json:"target_id"`
	Type     string  `json:"type"`   // "wikilink", "markdown_link", "shared_tag", "similar"
	Weight   float64 `json:"weight"` // similarity score or 1.0 for explicit
}

// GraphFilter controls graph queries.
type GraphFilter struct {
	SourceTypes []string `json:"source_types,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	MinScore    float64  `json:"min_score,omitempty"`
	MaxDepth    int      `json:"max_depth,omitempty"`
}

// Graph is a set of nodes and edges for visualization.
type Graph struct {
	Nodes []GraphNode    `json:"nodes"`
	Edges []GraphRelation `json:"edges"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/core/domain/graph.go
git commit -m "feat(domain): add knowledge graph types — GraphNode, GraphRelation, Graph"
```

---

### Task 2: GraphStore port

**Files:**
- Modify: `internal/core/ports/outbound.go`

- [ ] **Step 1: Add GraphStore interface**

Add at the end of `internal/core/ports/outbound.go`:

```go
// GraphStore manages the document knowledge graph.
type GraphStore interface {
	UpsertDocument(ctx context.Context, doc domain.GraphNode) error
	AddLink(ctx context.Context, sourceID, targetID string, linkType string) error
	AddSimilarity(ctx context.Context, sourceID, targetID string, score float64) error
	RemoveSimilarities(ctx context.Context, docID string) error
	GetRelated(ctx context.Context, docID string, maxDepth int, limit int) ([]domain.GraphRelation, error)
	FindByTitle(ctx context.Context, title string) ([]domain.GraphNode, error)
	GetGraph(ctx context.Context, filter domain.GraphFilter) (*domain.Graph, error)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/core/ports/outbound.go
git commit -m "feat(ports): add GraphStore interface for knowledge graph"
```

---

### Task 3: Neo4j client implementation

**Files:**
- Create: `internal/infrastructure/graph/neo4j/client.go`
- Create: `internal/infrastructure/graph/neo4j/client_test.go`

- [ ] **Step 1: Add dependency**

Run: `go get github.com/neo4j/neo4j-go-driver/v5`

- [ ] **Step 2: Implement Neo4j client**

Create `internal/infrastructure/graph/neo4j/client.go`:

```go
package neo4j

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// Client implements ports.GraphStore using Neo4j.
type Client struct {
	driver neo4j.DriverWithContext
}

func New(uri, username, password string) (*Client, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("neo4j connect: %w", err)
	}
	return &Client{driver: driver}, nil
}

func (c *Client) Close() error {
	return c.driver.Close(context.Background())
}

func (c *Client) UpsertDocument(ctx context.Context, doc domain.GraphNode) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MERGE (d:Document {id: $id})
		SET d.filename = $filename, d.source_type = $source_type,
		    d.category = $category, d.title = $title, d.path = $path
	`, map[string]any{
		"id": doc.ID, "filename": doc.Filename, "source_type": doc.SourceType,
		"category": doc.Category, "title": doc.Title, "path": doc.Path,
	})
	return err
}

func (c *Client) AddLink(ctx context.Context, sourceID, targetID, linkType string) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MATCH (a:Document {id: $source}), (b:Document {id: $target})
		MERGE (a)-[r:LINKS_TO {type: $type}]->(b)
	`, map[string]any{"source": sourceID, "target": targetID, "type": linkType})
	return err
}

func (c *Client) AddSimilarity(ctx context.Context, sourceID, targetID string, score float64) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MATCH (a:Document {id: $source}), (b:Document {id: $target})
		MERGE (a)-[r:SIMILAR]->(b)
		SET r.score = $score
	`, map[string]any{"source": sourceID, "target": targetID, "score": score})
	return err
}

func (c *Client) RemoveSimilarities(ctx context.Context, docID string) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MATCH (d:Document {id: $id})-[r:SIMILAR]-()
		DELETE r
	`, map[string]any{"id": docID})
	return err
}

func (c *Client) GetRelated(ctx context.Context, docID string, maxDepth int, limit int) ([]domain.GraphRelation, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	if maxDepth <= 0 {
		maxDepth = 1
	}
	if limit <= 0 {
		limit = 10
	}

	result, err := session.Run(ctx, fmt.Sprintf(`
		MATCH (d:Document {id: $id})-[r*1..%d]-(related:Document)
		WITH related, r
		UNWIND r AS rel
		WITH related, rel, startNode(rel) AS src, endNode(rel) AS tgt
		RETURN DISTINCT
			src.id AS source_id,
			tgt.id AS target_id,
			type(rel) AS rel_type,
			COALESCE(rel.score, rel.type, 1.0) AS weight
		LIMIT $limit
	`, maxDepth), map[string]any{"id": docID, "limit": limit})
	if err != nil {
		return nil, fmt.Errorf("neo4j get related: %w", err)
	}

	var relations []domain.GraphRelation
	for result.Next(ctx) {
		record := result.Record()
		sourceID, _ := record.Get("source_id")
		targetID, _ := record.Get("target_id")
		relType, _ := record.Get("rel_type")
		weight, _ := record.Get("weight")

		r := domain.GraphRelation{
			SourceID: fmt.Sprint(sourceID),
			TargetID: fmt.Sprint(targetID),
			Type:     fmt.Sprint(relType),
		}
		switch w := weight.(type) {
		case float64:
			r.Weight = w
		case int64:
			r.Weight = float64(w)
		default:
			r.Weight = 1.0
		}
		relations = append(relations, r)
	}
	return relations, nil
}

func (c *Client) FindByTitle(ctx context.Context, title string) ([]domain.GraphNode, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, `
		MATCH (d:Document)
		WHERE toLower(d.title) CONTAINS toLower($title)
		   OR toLower(d.filename) CONTAINS toLower($title)
		RETURN d.id AS id, d.filename AS filename, d.source_type AS source_type,
		       d.category AS category, d.title AS title, d.path AS path
		LIMIT 10
	`, map[string]any{"title": title})
	if err != nil {
		return nil, fmt.Errorf("neo4j find by title: %w", err)
	}

	var nodes []domain.GraphNode
	for result.Next(ctx) {
		r := result.Record()
		nodes = append(nodes, domain.GraphNode{
			ID:         strVal(r, "id"),
			Filename:   strVal(r, "filename"),
			SourceType: strVal(r, "source_type"),
			Category:   strVal(r, "category"),
			Title:      strVal(r, "title"),
			Path:       strVal(r, "path"),
		})
	}
	return nodes, nil
}

func (c *Client) GetGraph(ctx context.Context, filter domain.GraphFilter) (*domain.Graph, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	maxDepth := filter.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 2
	}
	minScore := filter.MinScore

	// Get all nodes and edges.
	nodeResult, err := session.Run(ctx, `
		MATCH (d:Document)
		RETURN d.id AS id, d.filename AS filename, d.source_type AS source_type,
		       d.category AS category, d.title AS title, d.path AS path
	`, nil)
	if err != nil {
		return nil, fmt.Errorf("neo4j get graph nodes: %w", err)
	}

	var nodes []domain.GraphNode
	for nodeResult.Next(ctx) {
		r := nodeResult.Record()
		node := domain.GraphNode{
			ID:         strVal(r, "id"),
			Filename:   strVal(r, "filename"),
			SourceType: strVal(r, "source_type"),
			Category:   strVal(r, "category"),
			Title:      strVal(r, "title"),
			Path:       strVal(r, "path"),
		}
		// Apply filters.
		if len(filter.SourceTypes) > 0 && !contains(filter.SourceTypes, node.SourceType) {
			continue
		}
		if len(filter.Categories) > 0 && !contains(filter.Categories, node.Category) {
			continue
		}
		nodes = append(nodes, node)
	}

	edgeResult, err := session.Run(ctx, `
		MATCH (a:Document)-[r]->(b:Document)
		RETURN a.id AS source_id, b.id AS target_id, type(r) AS rel_type,
		       COALESCE(r.score, 1.0) AS weight
	`, nil)
	if err != nil {
		return nil, fmt.Errorf("neo4j get graph edges: %w", err)
	}

	nodeIDs := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}

	var edges []domain.GraphRelation
	for edgeResult.Next(ctx) {
		r := edgeResult.Record()
		sourceID := strVal(r, "source_id")
		targetID := strVal(r, "target_id")

		// Only include edges between filtered nodes.
		if !nodeIDs[sourceID] || !nodeIDs[targetID] {
			continue
		}

		weight := 1.0
		if w, ok := r.Get("weight"); ok {
			switch v := w.(type) {
			case float64:
				weight = v
			case int64:
				weight = float64(v)
			}
		}

		if minScore > 0 && weight < minScore {
			continue
		}

		edges = append(edges, domain.GraphRelation{
			SourceID: sourceID,
			TargetID: targetID,
			Type:     strVal(r, "rel_type"),
			Weight:   weight,
		})
	}

	return &domain.Graph{Nodes: nodes, Edges: edges}, nil
}

func strVal(r *neo4j.Record, key string) string {
	v, _ := r.Get(key)
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Write basic test**

Create `internal/infrastructure/graph/neo4j/client_test.go`:

```go
package neo4j

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// Compile-time check that Client implements GraphStore.
var _ ports.GraphStore = (*Client)(nil)

func TestClientImplementsGraphStore(t *testing.T) {
	// This test verifies interface compliance at compile time.
	// Integration tests require a running Neo4j instance.
}
```

- [ ] **Step 4: Run build**

Run: `go build ./...`
Expected: Clean.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/graph/ go.mod go.sum
git commit -m "feat(graph/neo4j): Neo4j GraphStore implementation"
```

---

### Task 4: Noop GraphStore for GRAPH_ENABLED=false

**Files:**
- Create: `internal/infrastructure/graph/noop.go`

- [ ] **Step 1: Create noop implementation**

Create `internal/infrastructure/graph/noop.go`:

```go
package graph

import (
	"context"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// NoopStore is a no-op GraphStore for when graph features are disabled.
type NoopStore struct{}

func NewNoopStore() *NoopStore { return &NoopStore{} }

func (n *NoopStore) UpsertDocument(context.Context, domain.GraphNode) error             { return nil }
func (n *NoopStore) AddLink(context.Context, string, string, string) error              { return nil }
func (n *NoopStore) AddSimilarity(context.Context, string, string, float64) error       { return nil }
func (n *NoopStore) RemoveSimilarities(context.Context, string) error                   { return nil }
func (n *NoopStore) GetRelated(context.Context, string, int, int) ([]domain.GraphRelation, error) {
	return nil, nil
}
func (n *NoopStore) FindByTitle(context.Context, string) ([]domain.GraphNode, error) { return nil, nil }
func (n *NoopStore) GetGraph(context.Context, domain.GraphFilter) (*domain.Graph, error) {
	return &domain.Graph{}, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/infrastructure/graph/noop.go
git commit -m "feat(graph): noop GraphStore for disabled graph mode"
```

---

### Task 5: LinkExtractor — extract wikilinks and markdown links

**Files:**
- Create: `internal/core/usecase/links.go`
- Create: `internal/core/usecase/links_test.go`

- [ ] **Step 1: Write tests**

Create `internal/core/usecase/links_test.go`:

```go
package usecase

import "testing"

func TestExtractWikilinks(t *testing.T) {
	text := "This references [[Page One]] and also [[Another Page]] in the vault."
	links := extractWikilinks(text)
	if len(links) != 2 {
		t.Fatalf("expected 2 wikilinks, got %d: %v", len(links), links)
	}
	if links[0] != "Page One" || links[1] != "Another Page" {
		t.Errorf("links = %v", links)
	}
}

func TestExtractMarkdownLinks(t *testing.T) {
	text := "See [details](notes/details.md) and [guide](../guide.md) for more."
	links := extractMarkdownLinks(text)
	if len(links) != 2 {
		t.Fatalf("expected 2 md links, got %d: %v", len(links), links)
	}
	if links[0] != "notes/details.md" || links[1] != "../guide.md" {
		t.Errorf("links = %v", links)
	}
}

func TestExtractMarkdownLinks_IgnoresNonMd(t *testing.T) {
	text := "Visit [site](https://example.com) and [image](photo.png)."
	links := extractMarkdownLinks(text)
	if len(links) != 0 {
		t.Errorf("expected 0 md file links, got %v", links)
	}
}

func TestExtractWikilinks_Empty(t *testing.T) {
	links := extractWikilinks("no links here")
	if len(links) != 0 {
		t.Errorf("expected 0, got %v", links)
	}
}
```

- [ ] **Step 2: Implement LinkExtractor**

Create `internal/core/usecase/links.go`:

```go
package usecase

import "regexp"

var (
	wikilinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	mdLinkRe   = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+\.md)\)`)
)

// extractWikilinks returns all [[Page Name]] targets from text.
func extractWikilinks(text string) []string {
	matches := wikilinkRe.FindAllStringSubmatch(text, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if m[1] != "" {
			result = append(result, m[1])
		}
	}
	return result
}

// extractMarkdownLinks returns all [text](path.md) targets from text.
func extractMarkdownLinks(text string) []string {
	matches := mdLinkRe.FindAllStringSubmatch(text, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if m[2] != "" {
			result = append(result, m[2])
		}
	}
	return result
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/usecase/ -run "TestExtractWikilinks|TestExtractMarkdownLinks" -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/usecase/links.go internal/core/usecase/links_test.go
git commit -m "feat(usecase): link extraction for wikilinks and markdown links"
```

---

### Task 6: Integrate graph into ProcessDocumentUseCase

**Files:**
- Modify: `internal/core/usecase/process.go`
- Modify: `internal/core/usecase/process_test.go`

- [ ] **Step 1: Add graphStore field**

In `process.go`, add `graphStore ports.GraphStore` to struct and constructor:

```go
type ProcessDocumentUseCase struct {
	repo          ports.DocumentRepository
	extractors    ports.ExtractorRegistry
	metaExtractor ports.MetadataExtractor
	chunkers      ports.ChunkerRegistry
	embedder      ports.Embedder
	vectorDB      ports.VectorStore
	queue         ports.MessageQueue
	graphStore    ports.GraphStore
}
```

Update constructor to accept `graphStore ports.GraphStore` as last parameter.

- [ ] **Step 2: Add graph indexing to processPipeline**

After `uc.index(ctx, doc, chunks, vectors)` and before the enrich publish, add:

```go
// Index in knowledge graph (best-effort).
if uc.graphStore != nil {
	uc.indexGraph(ctx, doc, text, vectors)
}
```

Add the `indexGraph` method:

```go
func (uc *ProcessDocumentUseCase) indexGraph(ctx context.Context, doc *domain.Document, text string, vectors [][]float32) {
	// Upsert document node.
	node := domain.GraphNode{
		ID: doc.ID, Filename: doc.Filename, SourceType: doc.SourceType,
		Category: doc.Category, Title: doc.Title, Path: doc.Path,
	}
	if err := uc.graphStore.UpsertDocument(ctx, node); err != nil {
		slog.Warn("graph_upsert_failed", "document_id", doc.ID, "error", err)
		return
	}

	// Extract and resolve wikilinks.
	for _, target := range extractWikilinks(text) {
		nodes, err := uc.graphStore.FindByTitle(ctx, target)
		if err != nil || len(nodes) == 0 {
			continue
		}
		_ = uc.graphStore.AddLink(ctx, doc.ID, nodes[0].ID, "wikilink")
	}

	// Extract and resolve markdown links.
	for _, target := range extractMarkdownLinks(text) {
		nodes, err := uc.graphStore.FindByTitle(ctx, target)
		if err != nil || len(nodes) == 0 {
			continue
		}
		_ = uc.graphStore.AddLink(ctx, doc.ID, nodes[0].ID, "markdown_link")
	}

	// Semantic similarity via Qdrant (use first chunk embedding as doc embedding).
	if len(vectors) > 0 {
		similar, err := uc.vectorDB.Search(ctx, vectors[0], 10, domain.SearchFilter{})
		if err == nil {
			for _, s := range similar {
				if s.DocumentID != doc.ID && s.Score > 0.75 {
					_ = uc.graphStore.AddSimilarity(ctx, doc.ID, s.DocumentID, s.Score)
				}
			}
		}
	}
}
```

- [ ] **Step 3: Update process_test.go**

Add `graphStoreFake`:

```go
type graphStoreFake struct{}

func (f *graphStoreFake) UpsertDocument(context.Context, domain.GraphNode) error { return nil }
func (f *graphStoreFake) AddLink(context.Context, string, string, string) error  { return nil }
func (f *graphStoreFake) AddSimilarity(context.Context, string, string, float64) error { return nil }
func (f *graphStoreFake) RemoveSimilarities(context.Context, string) error { return nil }
func (f *graphStoreFake) GetRelated(context.Context, string, int, int) ([]domain.GraphRelation, error) {
	return nil, nil
}
func (f *graphStoreFake) FindByTitle(context.Context, string) ([]domain.GraphNode, error) {
	return nil, nil
}
func (f *graphStoreFake) GetGraph(context.Context, domain.GraphFilter) (*domain.Graph, error) {
	return &domain.Graph{}, nil
}
```

Update all `NewProcessDocumentUseCase` calls to pass `&graphStoreFake{}` as the last argument.

- [ ] **Step 4: Run tests**

Run: `go build ./... && go test ./internal/core/usecase/ -run TestProcessByID -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/usecase/process.go internal/core/usecase/process_test.go
git commit -m "feat(process): integrate knowledge graph — upsert nodes, extract links, find similar"
```

---

### Task 7: Retrieval boost in QueryUseCase

**Files:**
- Modify: `internal/core/usecase/query.go`

- [ ] **Step 1: Add graphStore field**

Add `graphStore ports.GraphStore` to `QueryUseCase` struct and `GraphStore ports.GraphStore` to `QueryOptions`. Wire in constructor.

- [ ] **Step 2: Add graph boost after retrieval**

In the `Answer` method, after `uc.retrieveChunks()` and before answer generation, add:

```go
// Graph retrieval boost — find related documents and add their chunks.
if uc.graphStore != nil && len(chunks) > 0 {
	chunks = uc.boostWithGraph(ctx, chunks, limit, filter)
}
```

Add the boost method:

```go
func (uc *QueryUseCase) boostWithGraph(ctx context.Context, chunks []domain.RetrievedChunk, limit int, filter domain.SearchFilter) []domain.RetrievedChunk {
	seen := make(map[string]bool)
	for _, c := range chunks {
		seen[c.DocumentID] = true
	}

	var boosted []domain.RetrievedChunk
	for _, c := range chunks {
		related, err := uc.graphStore.GetRelated(ctx, c.DocumentID, 1, 3)
		if err != nil {
			continue
		}
		for _, rel := range related {
			if seen[rel.TargetID] {
				continue
			}
			seen[rel.TargetID] = true
			// Search for chunks from related document.
			relFilter := filter
			relChunks, err := uc.vectorDB.Search(ctx, nil, 2, relFilter)
			if err != nil || len(relChunks) == 0 {
				continue
			}
			for i := range relChunks {
				if relChunks[i].DocumentID == rel.TargetID {
					relChunks[i].Score *= 0.7 // boost factor
					boosted = append(boosted, relChunks[i])
				}
			}
		}
	}

	result := append(chunks, boosted...)
	if len(result) > limit*2 {
		result = result[:limit*2]
	}
	return result
}
```

- [ ] **Step 3: Update query_test.go if needed**

Add `graphStore` field to `QueryOptions` in test calls. Pass `nil` for graph store in existing tests (no change to behavior).

- [ ] **Step 4: Run tests**

Run: `go build ./... && go test ./internal/core/usecase/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/usecase/query.go internal/core/usecase/query_test.go
git commit -m "feat(query): graph-based retrieval boost for related documents"
```

---

### Task 8: Graph-based query rewriting in AgentChatUseCase

**Files:**
- Modify: `internal/core/usecase/agent_chat.go`

- [ ] **Step 1: Add graphStore field and setter**

Add `graphStore ports.GraphStore` to `AgentChatUseCase` struct. Add setter:

```go
func (uc *AgentChatUseCase) SetGraphStore(g ports.GraphStore) {
	uc.graphStore = g
}
```

- [ ] **Step 2: Add query expansion with graph context**

In the `executeKnowledgeSearch` method (or wherever knowledge_search tool is executed), before the actual search call, add graph-based expansion:

```go
// Expand query with graph context.
if uc.graphStore != nil {
	expanded := uc.expandQueryWithGraph(ctx, query)
	if expanded != "" {
		query = query + " " + expanded
	}
}
```

Add the expansion method:

```go
func (uc *AgentChatUseCase) expandQueryWithGraph(ctx context.Context, query string) string {
	// Find documents matching query terms.
	tokens := strings.Fields(query)
	var expansions []string
	seen := make(map[string]bool)

	for _, token := range tokens {
		if len(token) < 3 {
			continue
		}
		nodes, err := uc.graphStore.FindByTitle(ctx, token)
		if err != nil || len(nodes) == 0 {
			continue
		}
		for _, node := range nodes[:min(len(nodes), 2)] {
			related, err := uc.graphStore.GetRelated(ctx, node.ID, 1, 3)
			if err != nil {
				continue
			}
			for _, rel := range related {
				relNodes, err := uc.graphStore.FindByTitle(ctx, rel.TargetID)
				if err != nil || len(relNodes) == 0 {
					continue
				}
				title := relNodes[0].Title
				if title != "" && !seen[title] {
					seen[title] = true
					expansions = append(expansions, title)
				}
			}
		}
	}

	if len(expansions) > 5 {
		expansions = expansions[:5]
	}
	return strings.Join(expansions, " ")
}
```

- [ ] **Step 3: Run tests**

Run: `go build ./... && go test ./internal/core/usecase/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/usecase/agent_chat.go
git commit -m "feat(agent): graph-based query rewriting with related document expansion"
```

---

### Task 9: Graph API endpoint + config + bootstrap

**Files:**
- Modify: `internal/adapters/http/router.go`
- Modify: `internal/config/config.go`
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Add config fields**

In `internal/config/config.go`, add to Config struct:

```go
Neo4jURL      string
Neo4jUser     string
Neo4jPassword string
GraphEnabled  bool
GraphSimilarityThreshold float64
GraphBoostFactor         float64
GraphRefreshIntervalHours int
```

Add to `Load()`:

```go
Neo4jURL:                  mustEnv("NEO4J_URL", "bolt://localhost:7687"),
Neo4jUser:                 mustEnv("NEO4J_USER", "neo4j"),
Neo4jPassword:             mustEnv("NEO4J_PASSWORD", "password"),
GraphEnabled:              mustEnvBool("GRAPH_ENABLED", false),
GraphSimilarityThreshold:  mustEnvFloat("GRAPH_SIMILARITY_THRESHOLD", 0.75),
GraphBoostFactor:          mustEnvFloat("GRAPH_BOOST_FACTOR", 0.7),
GraphRefreshIntervalHours: mustEnvInt("GRAPH_REFRESH_INTERVAL_HOURS", 24),
```

Add `mustEnvFloat` helper if it doesn't exist:

```go
func mustEnvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
```

- [ ] **Step 2: Add Neo4j to docker-compose.yml**

Add to `services:` section:

```yaml
  neo4j:
    image: neo4j:5-community
    ports:
      - "7474:7474"
      - "7687:7687"
    environment:
      NEO4J_AUTH: neo4j/password
    volumes:
      - neo4j_data:/data
```

Add to `volumes:` section:

```yaml
  neo4j_data:
```

- [ ] **Step 3: Wire in bootstrap**

In `internal/bootstrap/bootstrap.go`, add imports:

```go
graphpkg "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/graph"
graphneo4j "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/graph/neo4j"
```

After vectorDB/memoryVector setup, add:

```go
// Knowledge graph.
var graphStore ports.GraphStore
if cfg.GraphEnabled {
	neo4jClient, err := graphneo4j.New(cfg.Neo4jURL, cfg.Neo4jUser, cfg.Neo4jPassword)
	if err != nil {
		return nil, fmt.Errorf("connect neo4j: %w", err)
	}
	graphStore = neo4jClient
	// Add to closeFn later.
} else {
	graphStore = graphpkg.NewNoopStore()
}
```

Pass `graphStore` to `NewProcessDocumentUseCase` (as last arg), add `GraphStore: graphStore` to `QueryOptions`, and call `agentUC.SetGraphStore(graphStore)`.

Add `GraphStore ports.GraphStore` to the `App` struct for the close function.

- [ ] **Step 4: Add GET /v1/graph endpoint**

In `internal/adapters/http/router.go`, add a handler:

```go
mux.HandleFunc("GET /v1/graph", rt.handleGetGraph)
```

Add the handler method:

```go
func (rt *Router) handleGetGraph(w http.ResponseWriter, r *http.Request) {
	filter := domain.GraphFilter{
		MaxDepth: 2,
		MinScore: 0.5,
	}
	if st := r.URL.Query().Get("source_types"); st != "" {
		filter.SourceTypes = strings.Split(st, ",")
	}
	if ms := r.URL.Query().Get("min_score"); ms != "" {
		if v, err := strconv.ParseFloat(ms, 64); err == nil {
			filter.MinScore = v
		}
	}
	if md := r.URL.Query().Get("max_depth"); md != "" {
		if v, err := strconv.Atoi(md); err == nil {
			filter.MaxDepth = v
		}
	}

	graph, err := rt.graphStore.GetGraph(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(graph)
}
```

Add `graphStore ports.GraphStore` field to Router struct and constructor.

- [ ] **Step 5: Run build and tests**

Run: `go build ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: wire knowledge graph — config, Neo4j docker, bootstrap, GET /v1/graph endpoint"
```

---

### Task 10: Final verification + push

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1 -v 2>&1 | grep -E "FAIL|ok"`
Expected: All PASS

- [ ] **Step 2: Run vet**

Run: `go vet ./...`

- [ ] **Step 3: Tidy modules**

Run: `go mod tidy`

- [ ] **Step 4: Push**

```bash
git push origin main
```
