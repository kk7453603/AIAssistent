package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/resilience"
)

type Client struct {
	baseURL      string
	mu           sync.RWMutex
	genModel     string
	embedModel   string
	plannerModel string
	thinkEnabled bool
	httpClient   *http.Client
	executor     *resilience.Executor
}

func New(baseURL, genModel, embedModel string) *Client {
	return NewWithOptions(baseURL, genModel, embedModel, Options{})
}

type Options struct {
	PlannerModel       string
	ThinkEnabled       bool
	HTTPClient         *http.Client
	ResilienceExecutor *resilience.Executor
}

func NewWithOptions(baseURL, genModel, embedModel string, options Options) *Client {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		genModel:     genModel,
		embedModel:   embedModel,
		plannerModel: strings.TrimSpace(options.PlannerModel),
		thinkEnabled: options.ThinkEnabled,
		httpClient:   httpClient,
		executor:     options.ResilienceExecutor,
	}
}

func (c *Client) SetPlannerModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.plannerModel = strings.TrimSpace(model)
}

func (c *Client) GetRuntimeModelConfig() ports.RuntimeModelConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return ports.RuntimeModelConfig{
		GenerationModel: c.genModel,
		PlannerModel:    c.plannerModel,
		EmbeddingModel:  c.embedModel,
	}
}

func (c *Client) SetRuntimeModelConfig(config ports.RuntimeModelConfig) error {
	genModel := strings.TrimSpace(config.GenerationModel)
	embedModel := strings.TrimSpace(config.EmbeddingModel)
	plannerModel := strings.TrimSpace(config.PlannerModel)
	if genModel == "" {
		return errors.New("generation model is required")
	}
	if embedModel == "" {
		return errors.New("embedding model is required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.genModel = genModel
	c.plannerModel = plannerModel
	c.embedModel = embedModel
	return nil
}

func (c *Client) runtimeSnapshot() (genModel, plannerModel, embedModel string, thinkEnabled bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.genModel, c.plannerModel, c.embedModel, c.thinkEnabled
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
	_, _, embedModel, _ := e.client.runtimeSnapshot()

	request := map[string]any{
		"model": embedModel,
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

func (g *Generator) GenerateJSONFromPrompt(ctx context.Context, prompt string) (string, error) {
	return g.client.generateJSON(ctx, prompt)
}

func (c *Client) generateJSON(ctx context.Context, prompt string) (string, error) {
	genModel, plannerModel, _, _ := c.runtimeSnapshot()
	model := genModel
	if plannerModel != "" {
		model = plannerModel
	}

	reqBody := map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
		"format": "json",
		"think":  false,
	}
	return c.generate(ctx, reqBody)
}

func (c *Client) generateText(ctx context.Context, prompt string) (string, error) {
	genModel, _, _, _ := c.runtimeSnapshot()
	reqBody := map[string]any{
		"model":  genModel,
		"prompt": prompt,
		"stream": false,
		"think":  false,
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
