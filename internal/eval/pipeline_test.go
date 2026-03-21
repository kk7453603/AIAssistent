package eval

import (
	"context"
	"math"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// --- Retrieval metric tests ---

func TestPrecisionAtK(t *testing.T) {
	tests := []struct {
		name      string
		retrieved []domain.RetrievedChunk
		expected  []string
		want      float64
	}{
		{
			name:      "all relevant",
			retrieved: chunks("a.txt", "b.txt"),
			expected:  []string{"a.txt", "b.txt"},
			want:      1.0,
		},
		{
			name:      "half relevant",
			retrieved: chunks("a.txt", "c.txt"),
			expected:  []string{"a.txt", "b.txt"},
			want:      0.5,
		},
		{
			name:      "none relevant",
			retrieved: chunks("c.txt", "d.txt"),
			expected:  []string{"a.txt", "b.txt"},
			want:      0.0,
		},
		{
			name:      "empty retrieved",
			retrieved: nil,
			expected:  []string{"a.txt"},
			want:      0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PrecisionAtK(tc.retrieved, tc.expected)
			assertFloat(t, tc.want, got)
		})
	}
}

func TestRecallAtK(t *testing.T) {
	tests := []struct {
		name      string
		retrieved []domain.RetrievedChunk
		expected  []string
		want      float64
	}{
		{
			name:      "full recall",
			retrieved: chunks("a.txt", "b.txt", "c.txt"),
			expected:  []string{"a.txt", "b.txt"},
			want:      1.0,
		},
		{
			name:      "partial recall",
			retrieved: chunks("a.txt", "c.txt"),
			expected:  []string{"a.txt", "b.txt"},
			want:      0.5,
		},
		{
			name:      "no recall",
			retrieved: chunks("c.txt"),
			expected:  []string{"a.txt", "b.txt"},
			want:      0.0,
		},
		{
			name:      "empty expected",
			retrieved: chunks("a.txt"),
			expected:  nil,
			want:      0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RecallAtK(tc.retrieved, tc.expected)
			assertFloat(t, tc.want, got)
		})
	}
}

func TestMRR(t *testing.T) {
	tests := []struct {
		name      string
		retrieved []domain.RetrievedChunk
		expected  []string
		want      float64
	}{
		{
			name:      "first is relevant",
			retrieved: chunks("a.txt", "b.txt", "c.txt"),
			expected:  []string{"a.txt"},
			want:      1.0,
		},
		{
			name:      "second is relevant",
			retrieved: chunks("c.txt", "a.txt", "b.txt"),
			expected:  []string{"a.txt"},
			want:      0.5,
		},
		{
			name:      "third is relevant",
			retrieved: chunks("c.txt", "d.txt", "a.txt"),
			expected:  []string{"a.txt"},
			want:      1.0 / 3.0,
		},
		{
			name:      "none relevant",
			retrieved: chunks("c.txt", "d.txt"),
			expected:  []string{"a.txt"},
			want:      0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MRR(tc.retrieved, tc.expected)
			assertFloat(t, tc.want, got)
		})
	}
}

func TestNDCG(t *testing.T) {
	tests := []struct {
		name      string
		retrieved []domain.RetrievedChunk
		expected  []string
		want      float64
	}{
		{
			name:      "perfect ranking",
			retrieved: chunks("a.txt", "b.txt", "c.txt"),
			expected:  []string{"a.txt", "b.txt"},
			want:      1.0,
		},
		{
			name:      "reversed ranking",
			retrieved: chunks("c.txt", "a.txt", "b.txt"),
			expected:  []string{"a.txt", "b.txt"},
			// DCG = 0 + 1/log2(3) + 1/log2(4)
			// IDCG = 1/log2(2) + 1/log2(3)
			want: (1/math.Log2(3) + 1/math.Log2(4)) / (1/math.Log2(2) + 1/math.Log2(3)),
		},
		{
			name:      "empty",
			retrieved: nil,
			expected:  []string{"a.txt"},
			want:      0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NDCG(tc.retrieved, tc.expected)
			assertFloat(t, tc.want, got)
		})
	}
}

// --- Pipeline test with mock query function ---

func TestPipelineRetrievalOnly(t *testing.T) {
	mockQuery := func(_ context.Context, question string, topK int) ([]domain.RetrievedChunk, string, error) {
		return chunks("a.txt", "b.txt", "c.txt"), "mock answer", nil
	}

	p := &Pipeline{
		queryFn: mockQuery,
		config: PipelineConfig{
			TopK:        5,
			Concurrency: 1,
			Metrics:     []string{"retrieval"},
		},
	}

	cases := []EvalCase{
		{
			ID:                "1",
			Question:          "test question",
			ExpectedFilenames: []string{"a.txt", "b.txt"},
		},
	}

	report, err := p.Run(context.Background(), cases)
	if err != nil {
		t.Fatalf("pipeline run: %v", err)
	}

	if report.TotalCases != 1 {
		t.Fatalf("expected 1 case, got %d", report.TotalCases)
	}

	r := report.Cases[0]
	assertFloat(t, 2.0/3.0, r.PrecisionAtK)
	assertFloat(t, 1.0, r.RecallAtK)
	assertFloat(t, 1.0, r.MRR)
	// Faithfulness and relevancy should be 0 since we only ran retrieval.
	assertFloat(t, 0.0, r.Faithfulness)
	assertFloat(t, 0.0, r.AnswerRelevancy)
	assertFloat(t, 0.0, r.ContextRelevancy)
}

func TestComputeSummary(t *testing.T) {
	results := []EvalResult{
		{PrecisionAtK: 1.0, RecallAtK: 0.5, MRR: 1.0, NDCG: 0.8, Faithfulness: 0.9, AnswerRelevancy: 0.7, ContextRelevancy: 0.6},
		{PrecisionAtK: 0.5, RecallAtK: 1.0, MRR: 0.5, NDCG: 0.6, Faithfulness: 0.7, AnswerRelevancy: 0.9, ContextRelevancy: 0.8},
	}

	s := computeSummary(results)

	assertFloat(t, 0.75, s.MeanPrecision)
	assertFloat(t, 0.75, s.MeanRecall)
	assertFloat(t, 0.75, s.MeanMRR)
	assertFloat(t, 0.70, s.MeanNDCG)
	assertFloat(t, 0.80, s.MeanFaithfulness)
	assertFloat(t, 0.80, s.MeanAnswerRelevancy)
	assertFloat(t, 0.70, s.MeanContextRelevancy)
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	assertFloat(t, 1.0, cosineSimilarity(a, b))

	c := []float32{0, 1, 0}
	assertFloat(t, 0.0, cosineSimilarity(a, c))

	assertFloat(t, 0.0, cosineSimilarity(nil, nil))
}

// --- Helpers ---

func chunks(filenames ...string) []domain.RetrievedChunk {
	var result []domain.RetrievedChunk
	for i, f := range filenames {
		result = append(result, domain.RetrievedChunk{
			DocumentID: f,
			Filename:   f,
			ChunkIndex: i,
			Text:       "chunk text for " + f,
			Score:      float64(len(filenames)-i) / float64(len(filenames)),
		})
	}
	return result
}

func assertFloat(t *testing.T, want, got float64) {
	t.Helper()
	if math.Abs(want-got) > 1e-6 {
		t.Errorf("want %.6f, got %.6f", want, got)
	}
}
