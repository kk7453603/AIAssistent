package eval

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// QueryFunc runs a query and returns retrieved chunks and the generated answer.
type QueryFunc func(ctx context.Context, question string, topK int) ([]domain.RetrievedChunk, string, error)

// PipelineConfig configures the evaluation pipeline.
type PipelineConfig struct {
	TopK          int
	RetrievalMode domain.RetrievalMode
	Concurrency   int
	Metrics       []string // which metric groups to run: "retrieval", "faithfulness", "answer_relevancy", "context_relevancy", "all"
}

// Pipeline orchestrates running all evaluation metrics on a dataset.
type Pipeline struct {
	queryFn    QueryFunc
	scorers    []Scorer
	config     PipelineConfig
}

// NewPipeline creates a new evaluation pipeline.
func NewPipeline(
	queryFn QueryFunc,
	llm ports.AnswerGenerator,
	embedder ports.Embedder,
	config PipelineConfig,
) *Pipeline {
	if config.TopK <= 0 {
		config.TopK = 5
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 4
	}

	metricSet := toSet(config.Metrics)
	runAll := metricSet["all"] || len(config.Metrics) == 0

	var scorers []Scorer
	if runAll || metricSet["faithfulness"] {
		scorers = append(scorers, NewFaithfulnessScorer(llm))
	}
	if runAll || metricSet["answer_relevancy"] {
		scorers = append(scorers, NewAnswerRelevancyScorer(llm, embedder, 3))
	}
	if runAll || metricSet["context_relevancy"] {
		scorers = append(scorers, NewContextRelevancyScorer(llm))
	}

	return &Pipeline{
		queryFn: queryFn,
		scorers: scorers,
		config:  config,
	}
}

// Run executes the evaluation pipeline on all cases concurrently.
func (p *Pipeline) Run(ctx context.Context, cases []EvalCase) (*EvalReport, error) {
	results := make([]EvalResult, len(cases))
	errs := make([]error, len(cases))

	sem := make(chan struct{}, p.config.Concurrency)
	var wg sync.WaitGroup

	for i, ec := range cases {
		wg.Add(1)
		go func(idx int, evalCase EvalCase) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := p.evaluateCase(ctx, evalCase)
			if err != nil {
				errs[idx] = err
				slog.Error("eval case failed", "case_id", evalCase.ID, "error", err)
				return
			}
			results[idx] = *result
		}(i, ec)
	}
	wg.Wait()

	// Check for any errors.
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("case %s: %w", cases[i].ID, err)
		}
	}

	report := &EvalReport{
		GeneratedAt:   time.Now().UTC(),
		RetrievalMode: p.config.RetrievalMode,
		TopK:          p.config.TopK,
		TotalCases:    len(cases),
		Cases:         results,
	}
	report.Summary = computeSummary(results)

	return report, nil
}

func (p *Pipeline) evaluateCase(ctx context.Context, ec EvalCase) (*EvalResult, error) {
	// Run the query.
	chunks, answer, err := p.queryFn(ctx, ec.Question, p.config.TopK)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	result := &EvalResult{
		CaseID:          ec.ID,
		Question:        ec.Question,
		GeneratedAnswer: answer,
	}

	// Retrieval metrics (always computed).
	metricSet := toSet(p.config.Metrics)
	runAll := metricSet["all"] || len(p.config.Metrics) == 0
	if runAll || metricSet["retrieval"] {
		result.PrecisionAtK = PrecisionAtK(chunks, ec.ExpectedFilenames)
		result.RecallAtK = RecallAtK(chunks, ec.ExpectedFilenames)
		result.MRR = MRR(chunks, ec.ExpectedFilenames)
		result.NDCG = NDCG(chunks, ec.ExpectedFilenames)
	}

	// LLM-based scorers.
	input := ScorerInput{
		Question:        ec.Question,
		GeneratedAnswer: answer,
		GroundTruth:     ec.GroundTruth,
		RetrievedChunks: chunks,
	}

	for _, scorer := range p.scorers {
		score, err := scorer.Score(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("scorer %s: %w", scorer.Name(), err)
		}
		switch scorer.Name() {
		case "faithfulness":
			result.Faithfulness = score
		case "answer_relevancy":
			result.AnswerRelevancy = score
		case "context_relevancy":
			result.ContextRelevancy = score
		}
	}

	return result, nil
}

func computeSummary(results []EvalResult) EvalSummary {
	if len(results) == 0 {
		return EvalSummary{}
	}

	var s EvalSummary
	n := float64(len(results))
	for _, r := range results {
		s.MeanPrecision += r.PrecisionAtK
		s.MeanRecall += r.RecallAtK
		s.MeanMRR += r.MRR
		s.MeanNDCG += r.NDCG
		s.MeanFaithfulness += r.Faithfulness
		s.MeanAnswerRelevancy += r.AnswerRelevancy
		s.MeanContextRelevancy += r.ContextRelevancy
	}
	s.MeanPrecision /= n
	s.MeanRecall /= n
	s.MeanMRR /= n
	s.MeanNDCG /= n
	s.MeanFaithfulness /= n
	s.MeanAnswerRelevancy /= n
	s.MeanContextRelevancy /= n

	return s
}
