# Level 3 — Knowledge Graph

## Описание

Граф знаний на Neo4j. Извлечение wikilinks из документов, вычисление семантической близости, расширение поисковых запросов через связи графа. Опциональный компонент (GRAPH_ENABLED).

## Component Diagram

```mermaid
flowchart TB
    subgraph GraphSystem["Knowledge Graph"]
        Neo4jStore["Neo4jGraphStore\n─────────\nUpsertDocument()\nAddLink() / AddSimilarity()\nGetRelated() / FindByTitle()\nGetGraph()"]
        LinkExtractor["LinkExtractor\n─────────\nParseWikilinks()\nfrom document text"]
        QueryExpander["expandQueryWithGraph()\n─────────\nFindByTitle → GetRelated\nadd related titles to query"]
        GraphAPI["Graph API\n─────────\nGET /v1/graph\nfilter by source_type,\ncategory, min_score"]
    end

    ProcessUC["ProcessUseCase\n(Worker)"] -->|"extract links"| LinkExtractor
    ProcessUC -->|"upsert node + edges"| Neo4jStore
    AgentChat["AgentChatUseCase\n(API)"] -->|"expand query"| QueryExpander
    QueryExpander --> Neo4jStore

    GraphAPI --> Neo4jStore

    Neo4jStore --> Neo4j["Neo4j DB"]
    UI["Tauri UI\nGraphPage"] -->|"HTTP"| GraphAPI
```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| Neo4jGraphStore | `internal/infrastructure/graph/neo4j/client.go` |
| LinkExtractor | `internal/core/usecase/links.go` |
| Query expansion | `internal/core/usecase/agent_chat.go:expandQueryWithGraph` |
| NoopGraphStore | `internal/infrastructure/graph/noop.go` |
| Graph API handler | `internal/adapters/http/router.go` |
