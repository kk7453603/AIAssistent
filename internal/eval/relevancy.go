package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// AnswerRelevancyScorer evaluates how relevant the generated answer is to
// the original question. It generates N questions from the answer, embeds
// them, and computes cosine similarity with the original question embedding.
type AnswerRelevancyScorer struct {
	llm      ports.AnswerGenerator
	embedder ports.Embedder
	numQ     int // number of questions to generate from the answer
}

// NewAnswerRelevancyScorer creates a new answer relevancy scorer.
// numQuestions controls how many questions to generate from the answer (default 3).
func NewAnswerRelevancyScorer(llm ports.AnswerGenerator, embedder ports.Embedder, numQuestions int) *AnswerRelevancyScorer {
	if numQuestions <= 0 {
		numQuestions = 3
	}
	return &AnswerRelevancyScorer{llm: llm, embedder: embedder, numQ: numQuestions}
}

func (s *AnswerRelevancyScorer) Name() string { return "answer_relevancy" }

func (s *AnswerRelevancyScorer) Score(ctx context.Context, input ScorerInput) (float64, error) {
	if input.GeneratedAnswer == "" || input.Question == "" {
		return 0, nil
	}

	// Step 1: Generate questions from the answer.
	questions, err := s.generateQuestions(ctx, input.GeneratedAnswer)
	if err != nil {
		return 0, fmt.Errorf("generate questions: %w", err)
	}
	if len(questions) == 0 {
		return 0, nil
	}

	// Step 2: Embed the original question.
	origEmb, err := s.embedder.EmbedQuery(ctx, input.Question)
	if err != nil {
		return 0, fmt.Errorf("embed original question: %w", err)
	}

	// Step 3: Embed the generated questions.
	genEmbs, err := s.embedder.Embed(ctx, questions)
	if err != nil {
		return 0, fmt.Errorf("embed generated questions: %w", err)
	}

	// Step 4: Compute mean cosine similarity.
	totalSim := 0.0
	for _, emb := range genEmbs {
		totalSim += cosineSimilarity(origEmb, emb)
	}

	return totalSim / float64(len(genEmbs)), nil
}

func (s *AnswerRelevancyScorer) generateQuestions(ctx context.Context, answer string) ([]string, error) {
	prompt := fmt.Sprintf(`Generate %d questions that the following answer could be responding to.
Return ONLY a JSON array of strings.

Answer: %s`, s.numQ, answer)

	resp, err := s.llm.GenerateJSONFromPrompt(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var questions []string
	if err := json.Unmarshal([]byte(resp), &questions); err != nil {
		start := strings.Index(resp, "[")
		end := strings.LastIndex(resp, "]")
		if start >= 0 && end > start {
			if err2 := json.Unmarshal([]byte(resp[start:end+1]), &questions); err2 != nil {
				return nil, fmt.Errorf("parse questions JSON: %w", err)
			}
		} else {
			return nil, fmt.Errorf("parse questions JSON: %w", err)
		}
	}
	return questions, nil
}

// ContextRelevancyScorer evaluates what fraction of retrieved chunks is
// relevant to the question by asking an LLM about each chunk.
type ContextRelevancyScorer struct {
	llm ports.AnswerGenerator
}

// NewContextRelevancyScorer creates a new context relevancy scorer.
func NewContextRelevancyScorer(llm ports.AnswerGenerator) *ContextRelevancyScorer {
	return &ContextRelevancyScorer{llm: llm}
}

func (s *ContextRelevancyScorer) Name() string { return "context_relevancy" }

func (s *ContextRelevancyScorer) Score(ctx context.Context, input ScorerInput) (float64, error) {
	if len(input.RetrievedChunks) == 0 || input.Question == "" {
		return 0, nil
	}

	relevant := 0
	for _, chunk := range input.RetrievedChunks {
		ok, err := s.isRelevant(ctx, input.Question, chunk)
		if err != nil {
			return 0, fmt.Errorf("check chunk relevance: %w", err)
		}
		if ok {
			relevant++
		}
	}

	return float64(relevant) / float64(len(input.RetrievedChunks)), nil
}

func (s *ContextRelevancyScorer) isRelevant(ctx context.Context, question string, chunk domain.RetrievedChunk) (bool, error) {
	prompt := fmt.Sprintf(`Given the following question and text chunk, determine if the chunk contains information relevant to answering the question.
Answer with ONLY "yes" or "no".

Question: %s

Chunk:
%s`, question, chunk.Text)

	resp, err := s.llm.GenerateFromPrompt(ctx, prompt)
	if err != nil {
		return false, err
	}

	return strings.Contains(strings.ToLower(strings.TrimSpace(resp)), "yes"), nil
}

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a []float32, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dotProduct / denom
}
