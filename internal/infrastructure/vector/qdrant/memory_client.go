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
)

type MemoryClient struct {
	baseURL    string
	collection string
	httpClient *http.Client

	ensureMu          sync.Mutex
	ensuredCollection bool
	ensuredVectorSize int
}

func NewMemoryClient(baseURL, collection string) *MemoryClient {
	return &MemoryClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		collection: collection,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create memory upsert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("memory upsert request: %w", err)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create memory query request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("memory query request: %w", err)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create memory ensure collection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("memory ensure collection request: %w", err)
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
