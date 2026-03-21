# SPEC: RAG Evaluation Pipeline (RAGAS-style)

## Goal
Implement a Go-native RAG evaluation pipeline that measures **faithfulness**, **answer relevancy**, and **context relevancy** alongside existing retrieval metrics (precision, recall, MRR). This replaces the bash-only `scripts/eval/run.sh` with a programmatic, extensible evaluation framework.

## Current State
- `scripts/eval/run.sh` — bash script measuring precision@K, recall@K, MRR via curl + jq
- `QueryUseCase.Answer()` returns `domain.Answer{Text, Sources, Retrieval}`
- No LLM-based quality metrics (faithfulness, relevancy)

## Architecture

### New Package: `internal/eval/`

```
internal/eval/
  metrics.go        — metric interfaces & types
  faithfulness.go   — faithfulness scorer (LLM-based)
  relevancy.go      — answer & context relevancy scorers
  retrieval.go      — precision, recall, MRR, NDCG
  pipeline.go       — orchestrator: runs all metrics on a dataset
  report.go         — report generation (JSON + summary)
  pipeline_test.go  — unit tests
```

### Domain Types

```go
// internal/eval/metrics.go

type EvalCase struct {
    ID                string   `json:"id"`
    Question          string   `json:"question"`
    ExpectedFilenames []string `json:"expected_filenames"`
    GroundTruth       string   `json:"ground_truth,omitempty"` // reference answer for faithfulness
}

type EvalResult struct {
    CaseID             string  `json:"case_id"`
    Question           string  `json:"question"`
    GeneratedAnswer    string  `json:"generated_answer"`
    PrecisionAtK       float64 `json:"precision_at_k"`
    RecallAtK          float64 `json:"recall_at_k"`
    MRR                float64 `json:"mrr"`
    NDCG               float64 `json:"ndcg"`
    Faithfulness       float64 `json:"faithfulness"`       // 0-1: answer grounded in context
    AnswerRelevancy    float64 `json:"answer_relevancy"`    // 0-1: answer addresses question
    ContextRelevancy   float64 `json:"context_relevancy"`   // 0-1: retrieved chunks relevant to question
}

type EvalReport struct {
    GeneratedAt    time.Time           `json:"generated_at"`
    RetrievalMode  domain.RetrievalMode `json:"retrieval_mode"`
    TopK           int                 `json:"top_k"`
    TotalCases     int                 `json:"total_cases"`
    Summary        EvalSummary         `json:"summary"`
    Cases          []EvalResult        `json:"cases"`
}

type EvalSummary struct {
    MeanPrecision      float64 `json:"mean_precision"`
    MeanRecall         float64 `json:"mean_recall"`
    MeanMRR            float64 `json:"mean_mrr"`
    MeanNDCG           float64 `json:"mean_ndcg"`
    MeanFaithfulness   float64 `json:"mean_faithfulness"`
    MeanAnswerRelevancy float64 `json:"mean_answer_relevancy"`
    MeanContextRelevancy float64 `json:"mean_context_relevancy"`
}
```

### Faithfulness Scorer
Uses LLM to extract claims from generated answer, then checks each claim against context chunks:
1. Extract atomic statements from answer via LLM
2. For each statement, ask LLM: "Is this statement supported by the context? yes/no"
3. Score = supported_count / total_count

### Answer Relevancy Scorer
Uses LLM to generate N questions from the answer, then computes cosine similarity between original question embedding and generated question embeddings. Score = mean similarity.

### Context Relevancy Scorer
For each retrieved chunk, ask LLM: "Is this chunk relevant to the question? yes/no". Score = relevant_count / total_count.

### CLI Entry Point

```go
// cmd/eval/main.go
// Flags: --cases, --api-url, --top-k, --mode, --out, --metrics (retrieval,faithfulness,relevancy,all)
```

## Config (env vars)
```
EVAL_METRICS=all                    # comma-separated: retrieval,faithfulness,answer_relevancy,context_relevancy
EVAL_FAITHFULNESS_MODEL=            # override model for eval LLM calls (defaults to LLM_MODEL)
EVAL_CONCURRENCY=4                  # parallel case evaluation
```

## Integration
- `Makefile` target `eval-ragas` runs the new Go evaluator
- Existing `eval` target kept for backward compatibility
- New cases format extends existing JSONL with optional `ground_truth` field

## Dependencies
- `ports.AnswerGenerator` — for LLM-based scoring
- `ports.Embedder` — for cosine similarity in answer relevancy
- `ports.DocumentQueryService` — to run queries against live system

## Tests
- Unit tests with mock LLM responses for each scorer
- Table-driven tests for retrieval metrics (precision, recall, MRR, NDCG)
- Integration test that runs full pipeline on 3-5 sample cases
