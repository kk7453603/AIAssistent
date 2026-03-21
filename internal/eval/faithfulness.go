package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// FaithfulnessScorer evaluates whether the generated answer is grounded in
// the retrieved context. It uses an LLM to extract claims from the answer
// and then verifies each claim against the context.
type FaithfulnessScorer struct {
	llm ports.AnswerGenerator
}

// NewFaithfulnessScorer creates a new faithfulness scorer.
func NewFaithfulnessScorer(llm ports.AnswerGenerator) *FaithfulnessScorer {
	return &FaithfulnessScorer{llm: llm}
}

func (s *FaithfulnessScorer) Name() string { return "faithfulness" }

func (s *FaithfulnessScorer) Score(ctx context.Context, input ScorerInput) (float64, error) {
	if input.GeneratedAnswer == "" || len(input.RetrievedChunks) == 0 {
		return 0, nil
	}

	// Step 1: Extract atomic claims from the answer.
	claims, err := s.extractClaims(ctx, input.GeneratedAnswer)
	if err != nil {
		return 0, fmt.Errorf("extract claims: %w", err)
	}
	if len(claims) == 0 {
		return 1, nil // no claims to verify
	}

	// Step 2: Build context string from chunks.
	contextText := buildContextText(input.RetrievedChunks)

	// Step 3: Verify each claim against the context.
	supported := 0
	for _, claim := range claims {
		ok, err := s.verifyClaim(ctx, claim, contextText)
		if err != nil {
			return 0, fmt.Errorf("verify claim: %w", err)
		}
		if ok {
			supported++
		}
	}

	return float64(supported) / float64(len(claims)), nil
}

func (s *FaithfulnessScorer) extractClaims(ctx context.Context, answer string) ([]string, error) {
	prompt := fmt.Sprintf(`Extract all atomic factual claims from the following answer.
Return ONLY a JSON array of strings. Each string should be one standalone factual claim.

Answer: %s`, answer)

	resp, err := s.llm.GenerateJSONFromPrompt(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var claims []string
	if err := json.Unmarshal([]byte(resp), &claims); err != nil {
		// Try extracting JSON array from response.
		start := strings.Index(resp, "[")
		end := strings.LastIndex(resp, "]")
		if start >= 0 && end > start {
			if err2 := json.Unmarshal([]byte(resp[start:end+1]), &claims); err2 != nil {
				return nil, fmt.Errorf("parse claims JSON: %w", err)
			}
		} else {
			return nil, fmt.Errorf("parse claims JSON: %w", err)
		}
	}
	return claims, nil
}

func (s *FaithfulnessScorer) verifyClaim(ctx context.Context, claim, contextText string) (bool, error) {
	prompt := fmt.Sprintf(`Given the following context, determine if the statement is supported by the context.
Answer with ONLY "yes" or "no".

Context:
%s

Statement: %s`, contextText, claim)

	resp, err := s.llm.GenerateFromPrompt(ctx, prompt)
	if err != nil {
		return false, err
	}

	return strings.Contains(strings.ToLower(strings.TrimSpace(resp)), "yes"), nil
}

func buildContextText(chunks []domain.RetrievedChunk) string {
	var sb strings.Builder
	for i, c := range chunks {
		fmt.Fprintf(&sb, "[Chunk %d - %s]\n%s\n\n", i+1, c.Filename, c.Text)
	}
	return sb.String()
}
