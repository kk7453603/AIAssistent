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

	call := func(callCtx context.Context) error {
		req, err := http.NewRequestWithContext(callCtx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create %s request: %w", operation, err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("ollama %s request: %w", operation, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode >= 300 {
			return formatOllamaHTTPError(operation, resp)
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode %s response: %w", operation, err)
		}
		return nil
	}

	if c.executor != nil {
		err = c.executor.Execute(ctx, "ollama."+operation, call, classifyOllamaError)
	} else {
		err = call(ctx)
	}
	if err != nil {
		return wrapTemporaryIfNeeded("ollama "+operation, err)
	}
	return nil
}

// streamChunkCallback is called for each NDJSON chunk during streaming.
type streamChunkCallback func(chunk json.RawMessage) error

// postStreamJSON sends a POST and reads NDJSON streaming response line by line.
func (c *Client) postStreamJSON(ctx context.Context, path string, payload any, operation string, onChunk streamChunkCallback) error {
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
		return fmt.Errorf("ollama %s stream request: %w", operation, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return formatOllamaHTTPError(operation, resp)
	}

	dec := json.NewDecoder(resp.Body)
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decode %s stream chunk: %w", operation, err)
		}
		if err := onChunk(raw); err != nil {
			return err
		}
	}
	return nil
}

func formatOllamaHTTPError(operation string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	msg := strings.TrimSpace(string(body))
	return &HTTPStatusError{
		Operation:  operation,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       msg,
	}
}
