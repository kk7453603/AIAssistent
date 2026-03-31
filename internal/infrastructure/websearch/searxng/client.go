package searxng

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

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
	params.Set("categories", "general")

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

	return rankAndFilterResults(query, sr.Results, limit), nil
}

func rankAndFilterResults(query string, raw []rawResult, limit int) []domain.WebSearchResult {
	if limit <= 0 {
		limit = len(raw)
	}

	type scoredResult struct {
		result domain.WebSearchResult
		score  int
		index  int
	}

	terms := significantSearchTerms(query)
	scored := make([]scoredResult, 0, len(raw))
	seenURLs := make(map[string]struct{}, len(raw))

	for idx, r := range raw {
		result := domain.WebSearchResult{
			Title:   strings.TrimSpace(r.Title),
			URL:     strings.TrimSpace(r.URL),
			Snippet: strings.TrimSpace(r.Content),
		}
		if result.URL == "" {
			continue
		}
		key := normalizeURL(result.URL)
		if _, ok := seenURLs[key]; ok {
			continue
		}
		seenURLs[key] = struct{}{}
		scored = append(scored, scoredResult{
			result: result,
			score:  relevanceScore(terms, result),
			index:  idx,
		})
	}

	filtered := scored
	hasRelevant := false
	for _, item := range scored {
		if item.score > 0 {
			hasRelevant = true
			break
		}
	}
	if hasRelevant {
		filtered = filtered[:0]
		for _, item := range scored {
			if item.score > 0 {
				filtered = append(filtered, item)
			}
		}
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].score == filtered[j].score {
				return filtered[i].index < filtered[j].index
			}
			return filtered[i].score > filtered[j].score
		})
	}

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	results := make([]domain.WebSearchResult, 0, len(filtered))
	for _, item := range filtered {
		results = append(results, item.result)
	}
	return results
}

func relevanceScore(terms []string, result domain.WebSearchResult) int {
	if len(terms) == 0 {
		return 0
	}
	title := normalizeForMatch(result.Title)
	urlText := normalizeForMatch(result.URL)
	snippet := normalizeForMatch(result.Snippet)
	score := 0
	for _, term := range terms {
		if strings.Contains(title, term) {
			score += 4
		}
		if strings.Contains(urlText, term) {
			score += 3
		}
		if strings.Contains(snippet, term) {
			score++
		}
	}
	return score
}

func significantSearchTerms(query string) []string {
	stopwords := map[string]struct{}{
		"и": {}, "в": {}, "во": {}, "на": {}, "по": {}, "о": {}, "об": {}, "про": {}, "для": {},
		"что": {}, "кто": {}, "как": {}, "это": {}, "найди": {}, "найти": {}, "поищи": {}, "поиск": {},
		"информация": {}, "информацию": {}, "инфу": {}, "сети": {}, "интернете": {}, "онлайн": {},
		"the": {}, "a": {}, "an": {}, "about": {}, "find": {}, "search": {}, "look": {}, "lookup": {},
		"what": {}, "is": {}, "tell": {}, "me": {}, "information": {}, "online": {},
	}
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	terms := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := stopwords[field]; ok {
			continue
		}
		if len([]rune(field)) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		terms = append(terms, field)
	}
	return terms
}

func normalizeURL(rawURL string) string {
	normalized := strings.TrimSpace(strings.ToLower(rawURL))
	return strings.TrimSuffix(normalized, "/")
}

func normalizeForMatch(text string) string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	return strings.Join(fields, " ")
}
