package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	const maxSnippet = 4000
	snippet := text
	if len(snippet) > maxSnippet {
		snippet = snippet[:maxSnippet]
	}

	prompt := `You are a document classifier.
Return strict JSON object with keys:
category (string), subcategory (string), tags (array of strings), confidence (number from 0 to 1), summary (string).
No markdown, no extra keys.

Document:
` + snippet

	respText, err := c.client.generateJSON(ctx, prompt)
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

	reqBody := map[string]any{
		"model": e.client.embedModel,
		"input": texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.client.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, formatOllamaHTTPError("embed", resp)
	}

	var embedResp struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	return embedResp.Embeddings, nil
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
	var contextBuilder strings.Builder
	for idx, c := range chunks {
		contextBuilder.WriteString(fmt.Sprintf(
			"[%d] file=%s category=%s score=%.3f\n%s\n\n",
			idx+1,
			c.Filename,
			c.Category,
			c.Score,
			c.Text,
		))
	}

	prompt := fmt.Sprintf(`Answer user question only from context below.
If context is insufficient, say it directly.

Question:
%s

Context:
%s
`, question, contextBuilder.String())

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
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal generate request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create generate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama generate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", formatOllamaHTTPError("generate", resp)
	}

	var generateResp struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&generateResp); err != nil {
		return "", fmt.Errorf("decode generate response: %w", err)
	}
	return strings.TrimSpace(generateResp.Response), nil
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}
	return raw
}

func formatOllamaHTTPError(operation string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return fmt.Errorf("ollama %s status: %s", operation, resp.Status)
	}
	return fmt.Errorf("ollama %s status: %s: %s", operation, resp.Status, msg)
}
