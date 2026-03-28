package neo4j

import (
	"context"
	"fmt"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Client is a Neo4j-backed GraphStore implementation.
type Client struct {
	driver neo4jdriver.DriverWithContext
}

// New creates a new Client and verifies connectivity.
func New(uri, username, password string) (*Client, error) {
	driver, err := neo4jdriver.NewDriverWithContext(
		uri,
		neo4jdriver.BasicAuth(username, password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("neo4j: create driver: %w", err)
	}

	ctx := context.Background()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("neo4j: verify connectivity: %w", err)
	}

	return &Client{driver: driver}, nil
}

// Close closes the underlying Neo4j driver.
func (c *Client) Close() error {
	return c.driver.Close(context.Background())
}

// UpsertDocument creates or updates a Document node.
func (c *Client) UpsertDocument(ctx context.Context, doc domain.GraphNode) error {
	session := c.driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		query := `
MERGE (d:Document {id: $id})
SET d.filename    = $filename,
    d.source_type = $source_type,
    d.category    = $category,
    d.title       = $title,
    d.path        = $path`
		params := map[string]any{
			"id":          doc.ID,
			"filename":    doc.Filename,
			"source_type": doc.SourceType,
			"category":    doc.Category,
			"title":       doc.Title,
			"path":        doc.Path,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("neo4j: upsert document %q: %w", doc.ID, err)
	}
	return nil
}

// AddLink creates a LINKS_TO relationship between two Document nodes.
func (c *Client) AddLink(ctx context.Context, sourceID, targetID string, linkType string) error {
	session := c.driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		query := `
MATCH (src:Document {id: $source_id})
MATCH (tgt:Document {id: $target_id})
MERGE (src)-[r:LINKS_TO {type: $link_type}]->(tgt)`
		params := map[string]any{
			"source_id": sourceID,
			"target_id": targetID,
			"link_type": linkType,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("neo4j: add link %q -> %q: %w", sourceID, targetID, err)
	}
	return nil
}

// AddSimilarity creates or updates a SIMILAR relationship with a score.
func (c *Client) AddSimilarity(ctx context.Context, sourceID, targetID string, score float64) error {
	session := c.driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		query := `
MATCH (src:Document {id: $source_id})
MATCH (tgt:Document {id: $target_id})
MERGE (src)-[r:SIMILAR]-(tgt)
SET r.score = $score`
		params := map[string]any{
			"source_id": sourceID,
			"target_id": targetID,
			"score":     score,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("neo4j: add similarity %q <-> %q: %w", sourceID, targetID, err)
	}
	return nil
}

// RemoveSimilarities deletes all SIMILAR relationships for the given document.
func (c *Client) RemoveSimilarities(ctx context.Context, docID string) error {
	session := c.driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		query := `
MATCH (d:Document {id: $id})-[r:SIMILAR]-()
DELETE r`
		_, err := tx.Run(ctx, query, map[string]any{"id": docID})
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("neo4j: remove similarities for %q: %w", docID, err)
	}
	return nil
}

// GetRelated returns documents reachable from docID within maxDepth hops.
func (c *Client) GetRelated(ctx context.Context, docID string, maxDepth int, limit int) ([]domain.GraphRelation, error) {
	if maxDepth < 1 {
		maxDepth = 1
	}
	if limit < 1 {
		limit = 50
	}

	session := c.driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer session.Close(ctx)

	query := fmt.Sprintf(`
MATCH (d:Document {id: $id})-[r*1..%d]-(related:Document)
UNWIND r AS rel
RETURN DISTINCT
    startNode(rel).id        AS source_id,
    endNode(rel).id          AS target_id,
    type(rel)                AS rel_type,
    COALESCE(rel.score, 1.0) AS weight
LIMIT $limit`, maxDepth)

	result, err := session.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, runErr := tx.Run(ctx, query, map[string]any{
			"id":    docID,
			"limit": limit,
		})
		records, err := neo4jdriver.CollectWithContext(ctx, res, runErr)
		if err != nil {
			return nil, err
		}

		relations := make([]domain.GraphRelation, 0, len(records))
		for _, rec := range records {
			rel, err := recordToRelation(rec)
			if err != nil {
				continue
			}
			relations = append(relations, rel)
		}
		return relations, nil
	})
	if err != nil {
		return nil, fmt.Errorf("neo4j: get related for %q: %w", docID, err)
	}

	if result == nil {
		return nil, nil
	}
	return result.([]domain.GraphRelation), nil
}

// FindByTitle finds documents whose title or filename contains the query (case-insensitive).
func (c *Client) FindByTitle(ctx context.Context, title string) ([]domain.GraphNode, error) {
	session := c.driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer session.Close(ctx)

	query := `
MATCH (d:Document)
WHERE toLower(d.title)    CONTAINS toLower($title)
   OR toLower(d.filename) CONTAINS toLower($title)
RETURN d.id          AS id,
       d.filename    AS filename,
       d.source_type AS source_type,
       d.category    AS category,
       d.title       AS title,
       d.path        AS path
LIMIT 10`

	result, err := session.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, runErr := tx.Run(ctx, query, map[string]any{"title": title})
		records, err := neo4jdriver.CollectWithContext(ctx, res, runErr)
		if err != nil {
			return nil, err
		}

		nodes := make([]domain.GraphNode, 0, len(records))
		for _, rec := range records {
			node, err := recordToNode(rec)
			if err != nil {
				continue
			}
			nodes = append(nodes, node)
		}
		return nodes, nil
	})
	if err != nil {
		return nil, fmt.Errorf("neo4j: find by title %q: %w", title, err)
	}

	if result == nil {
		return nil, nil
	}
	return result.([]domain.GraphNode), nil
}

// GetGraph returns all nodes and edges, with optional filters applied.
func (c *Client) GetGraph(ctx context.Context, filter domain.GraphFilter) (*domain.Graph, error) {
	session := c.driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	defer session.Close(ctx)

	// Build WHERE clauses for node filters.
	var whereClauses []string
	params := map[string]any{}

	if len(filter.SourceTypes) > 0 {
		whereClauses = append(whereClauses, "d.source_type IN $source_types")
		params["source_types"] = filter.SourceTypes
	}
	if len(filter.Categories) > 0 {
		whereClauses = append(whereClauses, "d.category IN $categories")
		params["categories"] = filter.Categories
	}

	nodeWhere := ""
	if len(whereClauses) > 0 {
		nodeWhere = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Fetch nodes.
	nodeQuery := fmt.Sprintf(`
MATCH (d:Document)
%s
RETURN d.id          AS id,
       d.filename    AS filename,
       d.source_type AS source_type,
       d.category    AS category,
       d.title       AS title,
       d.path        AS path`, nodeWhere)

	nodesResult, err := session.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, runErr := tx.Run(ctx, nodeQuery, params)
		records, err := neo4jdriver.CollectWithContext(ctx, res, runErr)
		if err != nil {
			return nil, err
		}

		nodes := make([]domain.GraphNode, 0, len(records))
		for _, rec := range records {
			n, err := recordToNode(rec)
			if err != nil {
				continue
			}
			nodes = append(nodes, n)
		}
		return nodes, nil
	})
	if err != nil {
		return nil, fmt.Errorf("neo4j: get graph nodes: %w", err)
	}

	// Build node ID set for edge filtering.
	nodeSet := map[string]bool{}
	var graphNodes []domain.GraphNode
	if nodesResult != nil {
		graphNodes = nodesResult.([]domain.GraphNode)
		for _, n := range graphNodes {
			nodeSet[n.ID] = true
		}
	}

	// Fetch edges — apply MinScore filter for SIMILAR relationships.
	edgeQuery := `
MATCH (src:Document)-[r]->(tgt:Document)
WHERE (NOT type(r) = 'SIMILAR') OR (r.score >= $min_score)
RETURN startNode(r).id           AS source_id,
       endNode(r).id             AS target_id,
       type(r)                   AS rel_type,
       COALESCE(r.score, 1.0)    AS weight`

	edgesResult, err := session.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, runErr := tx.Run(ctx, edgeQuery, map[string]any{"min_score": filter.MinScore})
		records, err := neo4jdriver.CollectWithContext(ctx, res, runErr)
		if err != nil {
			return nil, err
		}

		edges := make([]domain.GraphRelation, 0, len(records))
		for _, rec := range records {
			rel, err := recordToRelation(rec)
			if err != nil {
				continue
			}
			// Keep only edges where both endpoints are in the filtered node set.
			if len(nodeSet) > 0 && (!nodeSet[rel.SourceID] || !nodeSet[rel.TargetID]) {
				continue
			}
			edges = append(edges, rel)
		}
		return edges, nil
	})
	if err != nil {
		return nil, fmt.Errorf("neo4j: get graph edges: %w", err)
	}

	var graphEdges []domain.GraphRelation
	if edgesResult != nil {
		graphEdges = edgesResult.([]domain.GraphRelation)
	}

	return &domain.Graph{
		Nodes: graphNodes,
		Edges: graphEdges,
	}, nil
}

// recordToNode maps a Neo4j record to domain.GraphNode.
func recordToNode(rec *neo4jdriver.Record) (domain.GraphNode, error) {
	id, err := stringField(rec, "id")
	if err != nil {
		return domain.GraphNode{}, err
	}
	filename, _ := stringField(rec, "filename")
	sourceType, _ := stringField(rec, "source_type")
	category, _ := stringField(rec, "category")
	title, _ := stringField(rec, "title")
	path, _ := stringField(rec, "path")

	return domain.GraphNode{
		ID:         id,
		Filename:   filename,
		SourceType: sourceType,
		Category:   category,
		Title:      title,
		Path:       path,
	}, nil
}

// recordToRelation maps a Neo4j record to domain.GraphRelation.
func recordToRelation(rec *neo4jdriver.Record) (domain.GraphRelation, error) {
	sourceID, err := stringField(rec, "source_id")
	if err != nil {
		return domain.GraphRelation{}, err
	}
	targetID, err := stringField(rec, "target_id")
	if err != nil {
		return domain.GraphRelation{}, err
	}
	relType, _ := stringField(rec, "rel_type")
	weight := floatField(rec, "weight")

	return domain.GraphRelation{
		SourceID: sourceID,
		TargetID: targetID,
		Type:     relType,
		Weight:   weight,
	}, nil
}

// stringField retrieves a string value from a record by key.
func stringField(rec *neo4jdriver.Record, key string) (string, error) {
	val, ok := rec.Get(key)
	if !ok || val == nil {
		return "", fmt.Errorf("field %q not found", key)
	}
	return fmt.Sprint(val), nil
}

// floatField retrieves a float64 value from a record by key.
func floatField(rec *neo4jdriver.Record, key string) float64 {
	val, ok := rec.Get(key)
	if !ok || val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	default:
		return 0
	}
}
