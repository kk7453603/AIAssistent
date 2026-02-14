package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama %s request: %w", operation, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return formatOllamaHTTPError(operation, resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, err)
	}
	return nil
}

func formatOllamaHTTPError(operation string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return fmt.Errorf("ollama %s status: %s", operation, resp.Status)
	}
	return fmt.Errorf("ollama %s status: %s: %s", operation, resp.Status, msg)
}
