# Knowledge Graph (Spec A — Foundation)

**Дата:** 2026-03-28
**Этап:** 6 — Уровень 2.4: Качество и память
**Статус:** approved

## Проблема

RAG ищет только по прямому совпадению запроса с чанками. Нет понимания связей между документами — wikilinks, общие теги, семантическая близость. Пользователь не видит структуру своих знаний.

## Решение

Neo4j knowledge graph: извлечение связей при индексации (wikilinks, shared tags, semantic similarity), retrieval boost через graph traversal, graph-based query rewriting, API для визуализации.

## Neo4j в Docker Compose

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

## Граф-модель

### Nodes

`(:Document {id, filename, source_type, category, title, path})`

### Relationships

- `(:Document)-[:LINKS_TO {type: "wikilink"|"markdown_link"|"url"}]->(:Document)` — explicit links
- `(:Document)-[:SHARES_TAG {tag: "go"}]->(:Document)` — shared tags
- `(:Document)-[:SIMILAR {score: 0.85}]->(:Document)` — semantic similarity

## Domain types

```go
type GraphNode struct {
    ID         string
    Filename   string
    SourceType string
    Category   string
    Title      string
    Path       string
}

type GraphRelation struct {
    SourceID string
    TargetID string
    Type     string   // "wikilink", "shared_tag", "similar"
    Weight   float64  // similarity score or 1.0 for explicit
}

type GraphFilter struct {
    SourceTypes []string
    Categories  []string
    MinScore    float64
    MaxDepth    int
}

type Graph struct {
    Nodes []GraphNode
    Edges []GraphRelation
}
```

## GraphStore порт

```go
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

## Link extraction при индексации

В `ProcessDocumentUseCase`, после metadata extraction:

1. **Wikilinks:** regex `\[\[([^\]]+)\]\]` → `LINKS_TO` с type "wikilink"
2. **Markdown links:** regex `\[([^\]]*)\]\(([^)]+\.md)\)` → `LINKS_TO` с type "markdown_link"
3. **Shared tags:** при upsert node, Neo4j создаёт `SHARES_TAG` для документов с общими tags

Link targets резолвятся по title/filename через `GraphStore.FindByTitle`.

## Semantic similarity

### Instant (при индексации)

После embed + index в Qdrant:
1. `Qdrant.Search(embedding, limit=10)` для нового документа
2. Для каждого с `score > threshold` → `Neo4j.AddSimilarity(docID, relatedID, score)`

### Background refresh (cron)

Периодический job в worker (каждые N часов):
1. Для каждого документа → Qdrant search top-K similar
2. `RemoveSimilarities(docID)` + create new
3. Refresh shared_tag relationships

## Retrieval boost

В `QueryUseCase`, после Qdrant results:
1. Для каждого найденного документа → `Neo4j.GetRelated(docID, depth=1, limit=3)`
2. Связанные документы → дополнительный Qdrant search по их ID
3. Merge с пониженным score (boost factor)

## Graph-based query rewriting

В `AgentChatUseCase`, перед основным поиском:
1. Tokenize запрос, ищем matching документы через `Neo4j.FindByTitle(tokens)`
2. Если найдены → `GetRelated(depth=1)` → получить связанные документы
3. Добавить titles/tags связанных документов как дополнительный контекст к запросу
4. Расширенный контекст передаётся в knowledge_search tool

## API для визуализации

```
GET /v1/graph?source_types=obsidian&min_score=0.7&max_depth=2
```

Возвращает `{nodes: [...], edges: [...]}` — формат для force-directed graph.

## Файловая структура

```
internal/core/domain/graph.go                     # GraphNode, GraphRelation, Graph, GraphFilter
internal/core/ports/outbound.go                   # GraphStore interface
internal/infrastructure/graph/neo4j/client.go     # Neo4j implementation
internal/infrastructure/graph/neo4j/client_test.go
internal/core/usecase/links.go                    # LinkExtractor (wikilinks, md links, regex)
internal/core/usecase/links_test.go
internal/core/usecase/process.go                  # extractLinks step + similarity
internal/core/usecase/query.go                    # graph retrieval boost
internal/core/usecase/agent_chat.go               # graph-based query rewriting
internal/adapters/http/router.go                  # GET /v1/graph endpoint
internal/bootstrap/bootstrap.go                   # wire Neo4j
docker-compose.yml                                # neo4j сервис
internal/config/config.go                         # Neo4j config fields
```

## Конфигурация

```env
NEO4J_URL=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=password
GRAPH_SIMILARITY_THRESHOLD=0.75
GRAPH_BOOST_FACTOR=0.7
GRAPH_REFRESH_INTERVAL_HOURS=24
GRAPH_ENABLED=true
```

`GRAPH_ENABLED=false` — graph features disabled, все GraphStore calls no-op. Позволяет работать без Neo4j.

## Что НЕ входит (Spec B — следующий)

- Tauri graph visualization UI
- Community detection / clustering (Neo4j GDS Louvain)
- Graph analytics (betweenness centrality, PageRank)
- Neo4j GDS plugin
