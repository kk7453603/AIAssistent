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
