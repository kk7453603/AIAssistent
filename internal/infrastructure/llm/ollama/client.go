package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type Client struct {
	baseURL    string
	genModel   string
	embedModel string
	httpClient *http.Client
}

func New(baseURL, genModel, embedModel string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		genModel:   genModel,
		embedModel: embedModel,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

type Classifier struct {
	client *Client
}

func NewClassifier(client *Client) *Classifier {
	return &Classifier{client: client}
}

func (c *Classifier) Classify(ctx context.Context, text string) (domain.Classification, error) {
	respText, err := c.client.generateJSON(ctx, buildClassificationPrompt(text))
	if err != nil {
		return domain.Classification{}, err
	}

	var result domain.Classification
	if err := json.Unmarshal([]byte(extractJSONObject(respText)), &result); err != nil {
		return domain.Classification{}, fmt.Errorf("parse classification json: %w", err)
	}
	if result.Tags == nil {
		result.Tags = []string{}
	}
	return result, nil
}

type Embedder struct {
	client *Client
}

func NewEmbedder(client *Client) *Embedder {
	return &Embedder{client: client}
}

func (e *Embedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	request := map[string]any{
		"model": e.client.embedModel,
		"input": texts,
	}

	var response struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := e.client.postJSON(ctx, "/api/embed", request, &response, "embed"); err != nil {
		return nil, err
	}
	return response.Embeddings, nil
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

type Generator struct {
	client *Client
}

func NewGenerator(client *Client) *Generator {
	return &Generator{client: client}
}

func (g *Generator) GenerateAnswer(ctx context.Context, question string, chunks []domain.RetrievedChunk) (string, error) {
	return g.client.generateText(ctx, buildAnswerPrompt(question, chunks))
}

func (g *Generator) GenerateFromPrompt(ctx context.Context, prompt string) (string, error) {
	return g.client.generateText(ctx, prompt)
}

func (c *Client) generateJSON(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]any{
		"model":  c.genModel,
		"prompt": prompt,
		"stream": false,
		"format": "json",
	}
	return c.generate(ctx, reqBody)
}

func (c *Client) generateText(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]any{
		"model":  c.genModel,
		"prompt": prompt,
		"stream": false,
	}
	return c.generate(ctx, reqBody)
}

func (c *Client) generate(ctx context.Context, reqBody map[string]any) (string, error) {
	var response struct {
		Response string `json:"response"`
	}
	if err := c.postJSON(ctx, "/api/generate", reqBody, &response, "generate"); err != nil {
		return "", err
	}
	return strings.TrimSpace(response.Response), nil
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}
	return raw
}
