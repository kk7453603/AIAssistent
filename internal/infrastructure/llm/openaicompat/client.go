package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to any OpenAI-compatible API (Groq, Together, Cerebras, OpenRouter, etc.).
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type Options struct {
	HTTPClient *http.Client
}

func New(baseURL, apiKey, model string, opts ...Options) *Client {
	httpClient := &http.Client{Timeout: 120 * time.Second}
	if len(opts) > 0 && opts[0].HTTPClient != nil {
		httpClient = opts[0].HTTPClient
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: httpClient,
	}
}

func (c *Client) Model() string { return c.model }

// chatCompletion sends a request to /v1/chat/completions.
func (c *Client) chatCompletion(ctx context.Context, messages []chatMessage, jsonMode bool) (string, error) {
	reqBody := chatRequest{
		Model:    c.model,
		Messages: messages,
	}
	if jsonMode {
		reqBody.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	var resp chatResponse
	if err := c.postJSON(ctx, "/v1/chat/completions", reqBody, &resp, "chat"); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openaicompat chat: empty choices")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// embedTexts sends a request to /v1/embeddings.
func (c *Client) embedTexts(ctx context.Context, model string, texts []string) ([][]float32, error) {
	reqBody := embedRequest{
		Model: model,
		Input: texts,
	}

	var resp embedResponse
	if err := c.postJSON(ctx, "/v1/embeddings", reqBody, &resp, "embed"); err != nil {
		return nil, err
	}

	vectors := make([][]float32, len(resp.Data))
	for _, item := range resp.Data {
		if item.Index >= len(vectors) {
			continue
		}
		vectors[item.Index] = item.Embedding
	}
	return vectors, nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload any, out any, operation string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create %s request: %w", operation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openaicompat %s request: %w", operation, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("openaicompat %s status %d: %s", operation, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, err)
	}
	return nil
}

// --- request / response types ---

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat  `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}
