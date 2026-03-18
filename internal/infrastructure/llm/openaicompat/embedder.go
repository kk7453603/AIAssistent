package openaicompat

import (
	"context"
	"fmt"
)

// Embedder implements ports.Embedder via an OpenAI-compatible /v1/embeddings endpoint.
type Embedder struct {
	client *Client
	model  string
}

func NewEmbedder(client *Client, model string) *Embedder {
	return &Embedder{client: client, model: model}
}

func (e *Embedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	return e.client.embedTexts(ctx, e.model, texts)
}

func (e *Embedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	vectors, err := e.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}
	return vectors[0], nil
}
