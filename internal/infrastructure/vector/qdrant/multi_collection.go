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
