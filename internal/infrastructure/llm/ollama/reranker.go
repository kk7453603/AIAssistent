package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// LLMReranker implements ports.Reranker using Ollama's /api/generate for pointwise scoring.
type LLMReranker struct {
	client      *Client
	concurrency int
}

func NewReranker(client *Client) *LLMReranker {
	return &LLMReranker{client: client, concurrency: 3}
}

func (r *LLMReranker) Rerank(ctx context.Context, query string, chunks []domain.RetrievedChunk, topN int) ([]domain.RetrievedChunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}
	if topN <= 0 || topN > len(chunks) {
		topN = len(chunks)
	}

	head := make([]domain.RetrievedChunk, topN)
	copy(head, chunks[:topN])

	type scored struct {
		idx   int
		score float64
		err   error
	}

	results := make([]scored, topN)
	sem := make(chan struct{}, r.concurrency)
	var wg sync.WaitGroup

	for i := range head {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			score, err := r.scoreChunk(ctx, query, head[idx])
			results[idx] = scored{idx: idx, score: score, err: err}
		}(i)
	}
	wg.Wait()

	for _, res := range results {
		if res.err != nil {
			return chunks, nil
		}
		head[res.idx].Score = res.score
	}

	sort.SliceStable(head, func(i, j int) bool {
		return head[i].Score > head[j].Score
	})

	if topN == len(chunks) {
		return head, nil
	}
	out := make([]domain.RetrievedChunk, 0, len(chunks))
	out = append(out, head...)
	out = append(out, chunks[topN:]...)
	return out, nil
}

func (r *LLMReranker) scoreChunk(ctx context.Context, query string, chunk domain.RetrievedChunk) (float64, error) {
	const maxText = 1500
	text := chunk.Text
	if len(text) > maxText {
		text = text[:maxText]
	}

	prompt := fmt.Sprintf(`Rate the relevance of the following document chunk to the query on a scale from 0 to 10.
Return ONLY a JSON object: {"score": <number>}

Query: %s

Document chunk (file: %s):
%s`, query, chunk.Filename, text)

	respText, err := r.client.generateJSON(ctx, prompt)
	if err != nil {
		return 0, err
	}

	jsonStr := extractScoreJSON(respText)
	var result struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return 0, fmt.Errorf("parse rerank score: %w", err)
	}
	return result.Score / 10.0, nil
}

func extractScoreJSON(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}
	return raw
}
