package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/resilience"
)

const (
	denseVectorName  = "dense"
	sparseVectorName = "text"
)

type Client struct {
	baseURL    string
	collection string
	httpClient *http.Client
	executor   *resilience.Executor

	ensureMu          sync.Mutex
	ensuredCollection bool
	ensuredVectorSize int
}

func New(baseURL, collection string) *Client {
	return NewWithOptions(baseURL, collection, Options{})
}

func NewWithOptions(baseURL, collection string, options Options) *Client {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		collection: collection,
		httpClient: httpClient,
		executor:   options.ResilienceExecutor,
	}
}

func (c *Client) doRequest(
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
			return fmt.Errorf("create %s request: %w", operation, err)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("qdrant %s request: %w", operation, err)
		}
		if isRetryableHTTPStatus(resp.StatusCode) {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			return &HTTPStatusError{
				Operation:  operation,
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
		err = c.executor.Execute(ctx, "qdrant."+operation, call, classifyQdrantError)
	} else {
		err = call(ctx)
	}
	if err != nil {
		return nil, wrapTemporaryIfNeeded("qdrant "+operation, err)
	}
	return response, nil
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
		Vector  map[string]any `json:"vector"`
		Payload map[string]any `json:"payload"`
	}

	points := make([]point, 0, len(chunks))
	for i := range chunks {
		sparse := encodeSparseDocument(chunks[i], doc.Filename)
		points = append(points, point{
			ID: uuid.NewString(),
			Vector: map[string]any{
				denseVectorName:  vectors[i],
				sparseVectorName: sparse,
			},
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
	resp, err := c.doRequest(ctx, "upsert_points", http.MethodPut, url, body, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if msg := strings.TrimSpace(string(body)); msg != "" {
			return fmt.Errorf("qdrant upsert status: %s: %s", resp.Status, msg)
		}
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
		"query":        queryVector,
		"using":        denseVectorName,
		"limit":        limit,
		"with_payload": true,
	}
	if filter.Category != "" {
		reqBody["filter"] = buildCategoryFilter(filter.Category)
	}

	return c.queryPoints(ctx, reqBody)
}

func (c *Client) SearchLexical(
	ctx context.Context,
	queryText string,
	limit int,
	filter domain.SearchFilter,
) ([]domain.RetrievedChunk, error) {
	sparse := encodeSparseQuery(queryText)
	if len(sparse.Indices) == 0 {
		return nil, nil
	}

	reqBody := map[string]any{
		"query":        sparse,
		"using":        sparseVectorName,
		"limit":        limit,
		"with_payload": true,
	}
	if filter.Category != "" {
		reqBody["filter"] = buildCategoryFilter(filter.Category)
	}

	return c.queryPoints(ctx, reqBody)
}

func (c *Client) queryPoints(ctx context.Context, reqBody map[string]any) ([]domain.RetrievedChunk, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal query body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/query", c.baseURL, c.collection)
	resp, err := c.doRequest(ctx, "query_points", http.MethodPost, url, body, "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if msg := strings.TrimSpace(string(body)); msg != "" {
			return nil, fmt.Errorf("qdrant query status: %s: %s", resp.Status, msg)
		}
		return nil, fmt.Errorf("qdrant query status: %s", resp.Status)
	}

	points, err := decodeQueryPoints(resp.Body)
	if err != nil {
		return nil, err
	}

	out := make([]domain.RetrievedChunk, 0, len(points))
	for _, r := range points {
		out = append(out, domain.RetrievedChunk{
			DocumentID: getStringPayload(r.Payload, "doc_id"),
			Filename:   getStringPayload(r.Payload, "filename"),
			Category:   getStringPayload(r.Payload, "category"),
			ChunkIndex: getIntPayload(r.Payload, "chunk_index"),
			Text:       getStringPayload(r.Payload, "text"),
			Score:      r.Score,
		})
	}
	return out, nil
}

type queryPoint struct {
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

func decodeQueryPoints(r io.Reader) ([]queryPoint, error) {
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(r).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode query response: %w", err)
	}

	var nested struct {
		Points []queryPoint `json:"points"`
	}
	if err := json.Unmarshal(envelope.Result, &nested); err == nil && nested.Points != nil {
		return nested.Points, nil
	}

	var flat []queryPoint
	if err := json.Unmarshal(envelope.Result, &flat); err == nil {
		return flat, nil
	}

	return nil, fmt.Errorf("decode query response: unexpected result shape")
}

func buildCategoryFilter(category string) map[string]any {
	return map[string]any{
		"must": []map[string]any{
			{
				"key": "category",
				"match": map[string]any{
					"value": category,
				},
			},
		},
	}
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
			denseVectorName: map[string]any{
				"size":     vectorSize,
				"distance": "Cosine",
			},
		},
		"sparse_vectors": map[string]any{
			sparseVectorName: map[string]any{
				"modifier": "idf",
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal create collection body: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s", c.baseURL, c.collection)
	resp, err := c.doRequest(ctx, "ensure_collection", http.MethodPut, url, body, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		if err := c.verifyCollectionSchema(ctx, vectorSize); err != nil {
			return err
		}
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

func (c *Client) verifyCollectionSchema(ctx context.Context, expectedVectorSize int) error {
	url := fmt.Sprintf("%s/collections/%s", c.baseURL, c.collection)
	resp, err := c.doRequest(ctx, "verify_collection", http.MethodGet, url, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if msg := strings.TrimSpace(string(body)); msg != "" {
			return fmt.Errorf("verify collection status: %s: %s", resp.Status, msg)
		}
		return fmt.Errorf("verify collection status: %s", resp.Status)
	}

	var payload struct {
		Result struct {
			Config struct {
				Params map[string]any `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("decode verify collection response: %w", err)
	}

	if err := verifyDenseVectorConfig(payload.Result.Config.Params, expectedVectorSize); err != nil {
		return err
	}
	if err := verifySparseVectorConfig(payload.Result.Config.Params); err != nil {
		return err
	}

	return nil
}

func verifyDenseVectorConfig(params map[string]any, expectedVectorSize int) error {
	vectorsRaw, ok := params["vectors"]
	if !ok {
		return fmt.Errorf("qdrant collection %q is missing vectors config", denseVectorName)
	}
	vectors, ok := vectorsRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("qdrant vectors config has unexpected shape")
	}

	namedDenseRaw, named := vectors[denseVectorName]
	if !named {
		if _, oldStyle := vectors["size"]; oldStyle {
			return fmt.Errorf("qdrant collection uses old single-vector schema; create a new collection with named dense+sparse vectors")
		}
		return fmt.Errorf("qdrant collection is missing vector %q", denseVectorName)
	}
	namedDense, ok := namedDenseRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("qdrant dense vector config has unexpected shape")
	}
	if expectedVectorSize > 0 {
		size, ok := asInt(namedDense["size"])
		if !ok {
			return fmt.Errorf("qdrant dense vector size is missing")
		}
		if size != expectedVectorSize {
			return fmt.Errorf("qdrant dense vector size mismatch: expected=%d actual=%d", expectedVectorSize, size)
		}
	}
	return nil
}

func verifySparseVectorConfig(params map[string]any) error {
	sparseRaw, ok := params["sparse_vectors"]
	if !ok {
		return fmt.Errorf("qdrant collection is missing sparse_vectors config")
	}
	sparse, ok := sparseRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("qdrant sparse_vectors config has unexpected shape")
	}
	if _, ok := sparse[sparseVectorName]; !ok {
		return fmt.Errorf("qdrant collection is missing sparse vector %q", sparseVectorName)
	}
	return nil
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
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

func getIntPayload(payload map[string]any, key string) int {
	v, ok := payload[key]
	if !ok {
		return -1
	}
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			return -1
		}
		return int(n)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return -1
		}
		return n
	default:
		return -1
	}
}
