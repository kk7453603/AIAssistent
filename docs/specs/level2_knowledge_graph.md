# SPEC: Knowledge Graph

## Goal
Build a graph of relationships between documents/notes (links, topics, entities) to enable graph-augmented retrieval and note exploration.

## Current State
- Documents are independent chunks in Qdrant with category/subcategory metadata
- No cross-document linking or entity extraction
- Obsidian notes may contain `[[wikilinks]]` but these are not parsed

## Architecture

### New Package: `internal/infrastructure/graph/`

```
internal/infrastructure/graph/
  store.go          — GraphStore interface + in-memory implementation
  postgres.go       — PostgreSQL-backed graph store
  extractor.go      — entity/relation extraction from text (LLM-based)
  builder.go        — graph builder (processes documents → nodes + edges)
  query.go          — graph traversal queries (neighbors, paths, subgraph)
  store_test.go
  extractor_test.go
```

### New Port

```go
// ports/outbound.go

type GraphStore interface {
    AddNode(ctx context.Context, node GraphNode) error
    AddEdge(ctx context.Context, edge GraphEdge) error
    GetNeighbors(ctx context.Context, nodeID string, depth int) ([]GraphNode, []GraphEdge, error)
    FindByEntity(ctx context.Context, entity string) ([]GraphNode, error)
    SearchSubgraph(ctx context.Context, query string, maxNodes int) ([]GraphNode, []GraphEdge, error)
}

type GraphNode struct {
    ID         string            `json:"id"`          // document_id or entity_id
    Type       string            `json:"type"`        // "document", "entity", "topic"
    Label      string            `json:"label"`
    Properties map[string]string `json:"properties"`
}

type GraphEdge struct {
    SourceID string `json:"source_id"`
    TargetID string `json:"target_id"`
    Type     string `json:"type"`     // "references", "related_to", "mentions", "wikilink"
    Weight   float64 `json:"weight"`
}
```

### DB Migration

```sql
CREATE TABLE graph_nodes (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    label TEXT NOT NULL,
    properties JSONB DEFAULT '{}'
);

CREATE TABLE graph_edges (
    source_id TEXT REFERENCES graph_nodes(id),
    target_id TEXT REFERENCES graph_nodes(id),
    type TEXT NOT NULL,
    weight DOUBLE PRECISION DEFAULT 1.0,
    PRIMARY KEY (source_id, target_id, type)
);

CREATE INDEX idx_graph_edges_target ON graph_edges(target_id);
CREATE INDEX idx_graph_nodes_type ON graph_nodes(type);
CREATE INDEX idx_graph_nodes_label_trgm ON graph_nodes USING gin(label gin_trgm_ops);
```

### Entity Extraction
- Parse `[[wikilinks]]` from Obsidian markdown
- LLM-based entity extraction: people, concepts, technologies, projects
- Run during document processing (`process.go`) after chunking

### Graph-Augmented Retrieval
In `query.go`, after vector retrieval, optionally expand results:
1. For each retrieved document, find graph neighbors (depth=1)
2. Include neighbor chunks as additional context (weighted by edge weight)
3. New retrieval mode: `graph+semantic`

### Config
```
KNOWLEDGE_GRAPH_ENABLED=false
KNOWLEDGE_GRAPH_EXTRACT_ENTITIES=true
KNOWLEDGE_GRAPH_MAX_DEPTH=2
KNOWLEDGE_GRAPH_NEIGHBOR_WEIGHT=0.5
```

### MCP Tool
New built-in tool `knowledge_graph` for agent:
- `explore_connections(document_id)` — show related documents
- `find_entity(name)` — find all documents mentioning entity
