.PHONY: generate-openapi generate test vet test-core-cover eval eval-ragas eval-cases eval-compare monitoring-validate \
       up down restart logs ps build pull \
       up-gpu down-gpu restart-gpu logs-gpu ps-gpu build-gpu \
       status models lint

# ── Docker Compose ──────────────────────────────────────────────

COMPOSE_CPU  := docker compose
COMPOSE_GPU  := docker compose -f docker-compose.yml -f docker-compose.host-gpu.yml

# CPU (Ollama in container — no GPU)
up:
	$(COMPOSE_CPU) up -d --build

down:
	$(COMPOSE_CPU) down

restart:
	$(COMPOSE_CPU) down && $(COMPOSE_CPU) up -d --build

logs:
	$(COMPOSE_CPU) logs -f --tail=100

ps:
	$(COMPOSE_CPU) ps

build:
	$(COMPOSE_CPU) build

pull:
	$(COMPOSE_CPU) pull

# GPU (host Ollama via nginx proxy)
up-gpu:
	$(COMPOSE_GPU) up -d --build

down-gpu:
	$(COMPOSE_GPU) down

restart-gpu:
	$(COMPOSE_GPU) down && $(COMPOSE_GPU) up -d --build

logs-gpu:
	$(COMPOSE_GPU) logs -f --tail=100

ps-gpu:
	$(COMPOSE_GPU) ps

build-gpu:
	$(COMPOSE_GPU) build

# ── Utilities ───────────────────────────────────────────────────

# Health check: show service statuses + Ollama model list
status:
	@echo "=== Services ==="
	@$(COMPOSE_CPU) ps 2>/dev/null || $(COMPOSE_GPU) ps 2>/dev/null || echo "No services running"
	@echo ""
	@echo "=== Ollama Models ==="
	@$(COMPOSE_CPU) exec ollama ollama list 2>/dev/null || curl -s http://localhost:11434/api/tags 2>/dev/null | python3 -m json.tool 2>/dev/null || echo "Ollama not available"

# Pull common models into Ollama
models:
	@echo "Pulling models..."
	$(COMPOSE_CPU) exec ollama ollama pull llama3.1:8b 2>/dev/null || ollama pull llama3.1:8b
	$(COMPOSE_CPU) exec ollama ollama pull nomic-embed-text 2>/dev/null || ollama pull nomic-embed-text

# golangci-lint
lint:
	golangci-lint run ./...

# ── Eval & Test ─────────────────────────────────────────────────

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
PROMTOOL_IMAGE ?= prom/prometheus:v2.54.1

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

eval-ragas:
	python3 scripts/eval/ragas_eval.py \
		--api-url "$(EVAL_API_URL)" \
		--cases "$(EVAL_CASES)" \
		--k $(EVAL_K) \
		--out ./tmp/eval/ragas_report.json

eval-cases:
	scripts/eval/generate_cases_from_manifest.sh --manifest "$(EVAL_MANIFEST)" --out "$(EVAL_GENERATED_CASES)"

eval-compare:
	scripts/eval/compare_modes.sh \
		--semantic "$(EVAL_REPORT_SEMANTIC)" \
		--hybrid "$(EVAL_REPORT_HYBRID)" \
		--hybrid-rerank "$(EVAL_REPORT_HYBRID_RERANK)" \
		--out "$(EVAL_COMPARE_OUT)"

monitoring-validate:
	docker run --rm \
		--entrypoint=promtool \
		-v "$(CURDIR)/deploy/monitoring/prometheus:/etc/prometheus:ro" \
		$(PROMTOOL_IMAGE) \
		check config /etc/prometheus/prometheus.yml
	docker run --rm \
		--entrypoint=promtool \
		-v "$(CURDIR)/deploy/monitoring/prometheus:/etc/prometheus:ro" \
		$(PROMTOOL_IMAGE) \
		check rules /etc/prometheus/rules/paa-alerts.yml
