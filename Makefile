.PHONY: generate-openapi generate test vet test-core-cover eval eval-cases eval-compare

EVAL_API_URL ?= http://localhost:8080
EVAL_CASES ?= scripts/eval/cases.example.jsonl
EVAL_K ?= 5
EVAL_REPORT ?= ./tmp/eval/report.json
EVAL_MANIFEST ?= ./tmp/rag-fixtures/manifest.csv
EVAL_GENERATED_CASES ?= ./tmp/eval/retrieval_cases.jsonl
EVAL_REPORT_SEMANTIC ?= ./tmp/eval/report_semantic.json
EVAL_REPORT_HYBRID ?= ./tmp/eval/report_hybrid.json
EVAL_REPORT_HYBRID_RERANK ?= ./tmp/eval/report_hybrid_rerank.json
EVAL_COMPARE_OUT ?= ./tmp/eval/modes_compare.json

generate-openapi:
	go generate ./internal/adapters/http/openapi

generate: generate-openapi

test:
	go test ./...

vet:
	go vet ./...

test-core-cover:
	go test ./internal/core/... ./internal/adapters/http -coverprofile=coverage.out

eval:
	scripts/eval/run.sh --api-url "$(EVAL_API_URL)" --cases "$(EVAL_CASES)" --k "$(EVAL_K)" --out "$(EVAL_REPORT)"

eval-cases:
	scripts/eval/generate_cases_from_manifest.sh --manifest "$(EVAL_MANIFEST)" --out "$(EVAL_GENERATED_CASES)"

eval-compare:
	scripts/eval/compare_modes.sh \
		--semantic "$(EVAL_REPORT_SEMANTIC)" \
		--hybrid "$(EVAL_REPORT_HYBRID)" \
		--hybrid-rerank "$(EVAL_REPORT_HYBRID_RERANK)" \
		--out "$(EVAL_COMPARE_OUT)"
