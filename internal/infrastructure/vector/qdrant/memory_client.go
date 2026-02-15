package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/resilience"
)

type MemoryClient struct {
	baseURL    string
	collection string
	httpClient *http.Client
	executor   *resilience.Executor

	ensureMu          sync.Mutex
	ensuredCollection bool
	ensuredVectorSize int
}

func NewMemoryClient(baseURL, collection string) *MemoryClient {
	return NewMemoryClientWithOptions(baseURL, collection, Options{})
}

func NewMemoryClientWithOptions(baseURL, collection string, options Options) *MemoryClient {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &MemoryClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		collection: collection,
		httpClient: httpClient,
		executor:   options.ResilienceExecutor,
	}
}

func (c *MemoryClient) doRequest(
	ctx context.Context,
	operation string,
	method string,
	url string,
	body []byte,
	contentType string,
) (*http.Response, error) {
	var response *http.Response
	call := func(callCtx context.Context) error {
		var payload io.Reader
		if len(body) > 0 {
			payload = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(callCtx, method, url, payload)
		if err != nil {
			return fmt.Errorf("create memory %s request: %w", operation, err)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("memory %s request: %w", operation, err)
		}
		if isRetryableHTTPStatus(resp.StatusCode) {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			return &HTTPStatusError{
				Operation:  "memory " + operation,
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Body:       strings.TrimSpace(string(body)),
			}
		}

		response = resp
		return nil
	}

	var err error
	if c.executor != nil {
		err = c.executor.Execute(ctx, "qdrant.memory."+operation, call, classifyQdrantError)
	} else {
		err = call(ctx)
	}
	if err != nil {
		return nil, wrapTemporaryIfNeeded("qdrant memory "+operation, err)
	}
	return response, nil
}

func (c *MemoryClient) IndexSummary(ctx context.Context, summary domain.MemorySummary, vector []float32) error {
	if len(vector) == 0 {
		return nil
	}
	if err := c.ensureCollection(ctx, len(vector)); err != nil {
		return err
	}

	body, err := json.Marshal(map[string]interface{}{
		"points": []map[string]interface{}{
			{
				"id":     summary.ID,
				"vector": vector,
				"payload": map[string]interface{}{
					"user_id":         summary.UserID,
					"conversation_id": summary.ConversationID,
					"summary_id":      summary.ID,
					"turn_from":       summary.TurnFrom,
					"turn_to":         summary.TurnTo,
					"text":            summary.Summary,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("marshal memory upsert body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points?wait=true", c.baseURL, c.collection)
	resp, err := c.doRequest(ctx, "upsert_points", http.MethodPut, url, body, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("memory upsert status: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *MemoryClient) SearchSummaries(
	ctx context.Context,
	userID, conversationID string,
	queryVector []float32,
	limit int,
) ([]domain.MemoryHit, error) {
	if len(queryVector) == 0 || strings.TrimSpace(userID) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 4
	}

	reqBody := map[string]interface{}{
		"query":        queryVector,
		"limit":        limit,
		"with_payload": true,
		"filter":       buildMemoryFilter(userID, conversationID),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal memory query body: %w", err)
	}
	url := fmt.Sprintf("%s/collections/%s/points/query", c.baseURL, c.collection)
	resp, err := c.doRequest(ctx, "query_points", http.MethodPost, url, body, "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("memory query status: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	points, err := decodeQueryPoints(resp.Body)
	if err != nil {
		return nil, err
	}
	out := make([]domain.MemoryHit, 0, len(points))
	for _, p := range points {
		out = append(out, domain.MemoryHit{
			Score: p.Score,
			Summary: domain.MemorySummary{
				ID:             getStringPayload(p.Payload, "summary_id"),
				UserID:         getStringPayload(p.Payload, "user_id"),
				ConversationID: getStringPayload(p.Payload, "conversation_id"),
				TurnFrom:       getIntPayload(p.Payload, "turn_from"),
				TurnTo:         getIntPayload(p.Payload, "turn_to"),
				Summary:        getStringPayload(p.Payload, "text"),
			},
		})
	}
	return out, nil
}

func buildMemoryFilter(userID, conversationID string) map[string]interface{} {
	must := []map[string]interface{}{
		{
			"key": "user_id",
			"match": map[string]interface{}{
				"value": userID,
			},
		},
	}
	if strings.TrimSpace(conversationID) != "" {
		must = append(must, map[string]interface{}{
			"key": "conversation_id",
			"match": map[string]interface{}{
				"value": conversationID,
			},
		})
	}
	return map[string]interface{}{"must": must}
}

func (c *MemoryClient) ensureCollection(ctx context.Context, vectorSize int) error {
	c.ensureMu.Lock()
	if c.ensuredCollection && c.ensuredVectorSize == vectorSize {
		c.ensureMu.Unlock()
		return nil
	}
	c.ensureMu.Unlock()

	body, err := json.Marshal(map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	})
	if err != nil {
		return fmt.Errorf("marshal memory ensure collection body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s", c.baseURL, c.collection)
	resp, err := c.doRequest(ctx, "ensure_collection", http.MethodPut, url, body, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("memory ensure collection status: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	c.ensureMu.Lock()
	c.ensuredCollection = true
	c.ensuredVectorSize = vectorSize
	c.ensureMu.Unlock()
	return nil
}
