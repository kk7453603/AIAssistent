# Baseline W01 (Observability + Quality)

Date: 2026-02-14  
Environment: docker compose with host GPU Ollama proxy (`docker-compose.host-gpu.yml`)  
Models: `qwen3-coder:30b`, `qwen3-embedding:8b`

## Dataset and eval setup
- Generated and uploaded documents: `20` (`ready=20`, `failed=0`)
- Source artifacts:
  - `tmp/baseline-w01/manifest.csv`
  - `tmp/baseline-w01/upload_results.csv`
  - `tmp/baseline-w01/retrieval_cases.jsonl`
  - `tmp/baseline-w01/eval_report.json`
- Retrieval eval cases: `40` (2 кейса на документ)
- Eval command:
```bash
make eval \
  EVAL_API_URL=http://localhost:8080 \
  EVAL_CASES=./tmp/baseline-w01/retrieval_cases.jsonl \
  EVAL_K=5 \
  EVAL_REPORT=./tmp/baseline-w01/eval_report.json
```

## Retrieval quality (top-k = 5)
- `precision@5`: `0.17000000`
- `recall@5`: `0.85000000`
- `MRR@5`: `0.62791667`
- `evaluated_cases`: `40`
- `failed_cases`: `0`

## Latency snapshot (`/v1/rag/query`)
Computed from the last `40` successful `/v1/rag/query` requests in API logs:
- `mean`: `6506.19 ms`
- `p50`: `6404.93 ms`
- `p95`: `8681.95 ms`
- `min`: `5772.25 ms`
- `max`: `9647.46 ms`

## Observability checklist
- API metrics endpoint: `GET /metrics` on `:8080`
- Worker metrics endpoint: `GET /metrics` on `:9090`
- Worker health endpoint: `GET /healthz` on `:9090`
- JSON structured logs enabled for API and worker (`request_id`, `document_id`, `duration_ms`)
- LLM/RAG metrics are exported (`paa_llm_tokens_total`, `paa_rag_requests_total`, `paa_rag_retrieval_hit_total`, `paa_rag_no_context_total`)
- Worker queue lag is exported (`paa_worker_queue_lag_seconds`)

## Notes
- Prometheus counters are cumulative across process lifetime; for scenario-specific quality use `tmp/baseline-w01/eval_report.json`.
- During this run `no_context` metric was not emitted as non-zero for the measured requests.
