package searxng

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type rawResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// Client interacts with the SearXNG JSON API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	limit      int
}

func New(baseURL string, limit int) *Client {
	if limit <= 0 {
		limit = 5
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		limit: limit,
	}
}

type searchResponse struct {
	Results []rawResult `json:"results"`
}

// Search performs a web search and returns up to limit results.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]domain.WebSearchResult, error) {
	if limit <= 0 {
		limit = c.limit
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("language", "ru-RU")

	reqURL := fmt.Sprintf("%s/search?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create searxng request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searxng request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("searxng returned status %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decode searxng response: %w", err)
	}

	raw := sr.Results
	if len(raw) > limit {
		raw = raw[:limit]
	}

	results := make([]domain.WebSearchResult, 0, len(raw))
	for _, r := range raw {
		results = append(results, domain.WebSearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}
