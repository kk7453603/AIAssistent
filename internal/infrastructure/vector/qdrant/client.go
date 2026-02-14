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

	"github.com/google/uuid"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type Client struct {
	baseURL    string
	collection string
	httpClient *http.Client

	ensureMu          sync.Mutex
	ensuredCollection bool
	ensuredVectorSize int
}

func New(baseURL, collection string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		collection: collection,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) IndexChunks(ctx context.Context, doc *domain.Document, chunks []string, vectors [][]float32) error {
	if len(chunks) == 0 || len(vectors) == 0 {
		return nil
	}
	if len(chunks) != len(vectors) {
		return fmt.Errorf("chunks/vectors mismatch")
	}

	if err := c.ensureCollection(ctx, len(vectors[0])); err != nil {
		return err
	}

	type point struct {
		ID      string         `json:"id"`
		Vector  []float32      `json:"vector"`
		Payload map[string]any `json:"payload"`
	}

	points := make([]point, 0, len(chunks))
	for i := range chunks {
		points = append(points, point{
			ID:     uuid.NewString(),
			Vector: vectors[i],
			Payload: map[string]any{
				"doc_id":      doc.ID,
				"filename":    doc.Filename,
				"category":    doc.Category,
				"subcategory": doc.Subcategory,
				"chunk_index": i,
				"text":        chunks[i],
			},
		})
	}

	reqBody := map[string]any{"points": points}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal upsert body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points?wait=true", c.baseURL, c.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create upsert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant upsert request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant upsert status: %s", resp.Status)
	}
	return nil
}

func (c *Client) Search(
	ctx context.Context,
	queryVector []float32,
	limit int,
	filter domain.SearchFilter,
) ([]domain.RetrievedChunk, error) {
	reqBody := map[string]any{
		"vector":       queryVector,
		"limit":        limit,
		"with_payload": true,
	}
	if filter.Category != "" {
		reqBody["filter"] = map[string]any{
			"must": []map[string]any{
				{
					"key": "category",
					"match": map[string]any{
						"value": filter.Category,
					},
				},
			},
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal search body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", c.baseURL, c.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant search status: %s", resp.Status)
	}

	var searchResp struct {
		Result []struct {
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	out := make([]domain.RetrievedChunk, 0, len(searchResp.Result))
	for _, r := range searchResp.Result {
		out = append(out, domain.RetrievedChunk{
			DocumentID: getStringPayload(r.Payload, "doc_id"),
			Filename:   getStringPayload(r.Payload, "filename"),
			Category:   getStringPayload(r.Payload, "category"),
			Text:       getStringPayload(r.Payload, "text"),
			Score:      r.Score,
		})
	}
	return out, nil
}

func (c *Client) ensureCollection(ctx context.Context, vectorSize int) error {
	c.ensureMu.Lock()
	if c.ensuredCollection && c.ensuredVectorSize == vectorSize {
		c.ensureMu.Unlock()
		return nil
	}
	c.ensureMu.Unlock()

	reqBody := map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal create collection body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s", c.baseURL, c.collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create collection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant ensure collection request: %w", err)
	}
	defer resp.Body.Close()

	// 200/201 for create, 409 if already exists (depends on version/config).
	if resp.StatusCode == http.StatusConflict {
		c.markCollectionEnsured(vectorSize)
		return nil
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if msg := strings.TrimSpace(string(body)); msg != "" {
			return fmt.Errorf("qdrant ensure collection status: %s: %s", resp.Status, msg)
		}
		return fmt.Errorf("qdrant ensure collection status: %s", resp.Status)
	}
	c.markCollectionEnsured(vectorSize)
	return nil
}

func (c *Client) markCollectionEnsured(vectorSize int) {
	c.ensureMu.Lock()
	defer c.ensureMu.Unlock()
	c.ensuredCollection = true
	c.ensuredVectorSize = vectorSize
}

func getStringPayload(payload map[string]any, key string) string {
	v, ok := payload[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
