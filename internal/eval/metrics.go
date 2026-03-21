package eval

import (
	"context"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// EvalCase represents a single evaluation test case.
type EvalCase struct {
	ID                string   `json:"id"`
	Question          string   `json:"question"`
	ExpectedFilenames []string `json:"expected_filenames"`
	GroundTruth       string   `json:"ground_truth,omitempty"`
}

// EvalResult holds all metric scores for a single evaluation case.
type EvalResult struct {
	CaseID           string  `json:"case_id"`
	Question         string  `json:"question"`
	GeneratedAnswer  string  `json:"generated_answer"`
	PrecisionAtK     float64 `json:"precision_at_k"`
	RecallAtK        float64 `json:"recall_at_k"`
	MRR              float64 `json:"mrr"`
	NDCG             float64 `json:"ndcg"`
	Faithfulness     float64 `json:"faithfulness"`
	AnswerRelevancy  float64 `json:"answer_relevancy"`
	ContextRelevancy float64 `json:"context_relevancy"`
}

// EvalReport contains the full evaluation report including all cases and summary.
type EvalReport struct {
	GeneratedAt   time.Time            `json:"generated_at"`
	RetrievalMode domain.RetrievalMode `json:"retrieval_mode"`
	TopK          int                  `json:"top_k"`
	TotalCases    int                  `json:"total_cases"`
	Summary       EvalSummary          `json:"summary"`
	Cases         []EvalResult         `json:"cases"`
}

// EvalSummary holds aggregated mean scores across all cases.
type EvalSummary struct {
	MeanPrecision        float64 `json:"mean_precision"`
	MeanRecall           float64 `json:"mean_recall"`
	MeanMRR              float64 `json:"mean_mrr"`
	MeanNDCG             float64 `json:"mean_ndcg"`
	MeanFaithfulness     float64 `json:"mean_faithfulness"`
	MeanAnswerRelevancy  float64 `json:"mean_answer_relevancy"`
	MeanContextRelevancy float64 `json:"mean_context_relevancy"`
}

// Scorer computes a single evaluation metric for a given case.
type Scorer interface {
	// Name returns the human-readable name of this scorer.
	Name() string
	// Score computes the metric value for the given evaluation context.
	Score(ctx context.Context, input ScorerInput) (float64, error)
}

// ScorerInput provides all data a scorer might need.
type ScorerInput struct {
	Question        string
	GeneratedAnswer string
	GroundTruth     string
	RetrievedChunks []domain.RetrievedChunk
}
