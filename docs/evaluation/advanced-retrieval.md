# Advanced Retrieval Benchmark

Date: 2026-02-14  
Environment: docker compose  
Collection: `QDRANT_COLLECTION=documents_qwen3_embedding_8b_hybrid_v1`

## Eval setup
- Cases file: `tmp/advanced-retrieval-fast/retrieval_cases_quick.jsonl` (4 cases)
- Top-k: `1`
- Modes:
  - `semantic`
  - `hybrid`
  - `hybrid+rerank`
- Reports:
  - `tmp/advanced-retrieval-fast/report_semantic.json`
  - `tmp/advanced-retrieval-fast/report_hybrid.json`
  - `tmp/advanced-retrieval-fast/report_hybrid_rerank.json`
  - `tmp/advanced-retrieval-fast/modes_compare.json`

## Metrics comparison

| Mode | precision@k | recall@k | MRR@k |
|---|---:|---:|---:|
| semantic | 0.50000000 | 0.50000000 | 0.50000000 |
| hybrid | 1.00000000 | 1.00000000 | 1.00000000 |
| hybrid+rerank | 1.00000000 | 1.00000000 | 1.00000000 |

## Deltas vs semantic
- `hybrid`:
  - `precision@k`: `+0.50000000`
  - `recall@k`: `+0.50000000`
  - `MRR@k`: `+0.50000000`
- `hybrid+rerank`:
  - `precision@k`: `+0.50000000`
  - `recall@k`: `+0.50000000`
  - `MRR@k`: `+0.50000000`

## Notes
- Это быстрый smoke-benchmark (4 кейса, `k=1`) для проверки регрессий и подтверждения направления качества.
- Для финального weekly-checkin стоит повторить тот же процесс на полном наборе `30-50` кейсов.

## Commands
```bash
# 1) Run per-mode evals on the same cases (set RAG_RETRIEVAL_MODE + restart api before each run)
make eval EVAL_CASES=./tmp/advanced-retrieval-fast/retrieval_cases_quick.jsonl EVAL_K=1 EVAL_REPORT=./tmp/advanced-retrieval-fast/report_semantic.json
make eval EVAL_CASES=./tmp/advanced-retrieval-fast/retrieval_cases_quick.jsonl EVAL_K=1 EVAL_REPORT=./tmp/advanced-retrieval-fast/report_hybrid.json
make eval EVAL_CASES=./tmp/advanced-retrieval-fast/retrieval_cases_quick.jsonl EVAL_K=1 EVAL_REPORT=./tmp/advanced-retrieval-fast/report_hybrid_rerank.json

# 2) Compare reports
make eval-compare \
  EVAL_REPORT_SEMANTIC=./tmp/advanced-retrieval-fast/report_semantic.json \
  EVAL_REPORT_HYBRID=./tmp/advanced-retrieval-fast/report_hybrid.json \
  EVAL_REPORT_HYBRID_RERANK=./tmp/advanced-retrieval-fast/report_hybrid_rerank.json \
  EVAL_COMPARE_OUT=./tmp/advanced-retrieval-fast/modes_compare.json
```
