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
	Type     string  `json:"type"`
	Weight   float64 `json:"weight"`
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
	Nodes []GraphNode     `json:"nodes"`
	Edges []GraphRelation `json:"edges"`
}
