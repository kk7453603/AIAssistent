.PHONY: generate-openapi generate test vet test-core-cover eval eval-cases

EVAL_API_URL ?= http://localhost:8080
EVAL_CASES ?= scripts/eval/cases.example.jsonl
EVAL_K ?= 5
EVAL_REPORT ?= ./tmp/eval/report.json
EVAL_MANIFEST ?= ./tmp/rag-fixtures/manifest.csv
EVAL_GENERATED_CASES ?= ./tmp/eval/retrieval_cases.jsonl

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
