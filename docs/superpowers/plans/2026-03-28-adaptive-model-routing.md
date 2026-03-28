# Adaptive Model Routing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Auto-select the optimal LLM model per request based on task complexity and type, with auto-discovery of available Ollama models.

**Architecture:** `ComplexityClassifier` (rule-based + LLM fallback) determines tier (simple/complex/code). `ModelDiscovery` auto-detects Ollama models and assigns tiers. `AgentChatUseCase` sets the model via existing `routing.WithProvider` before LLM calls.

**Tech Stack:** Go, Ollama `/api/tags` API, existing routing infrastructure.

**Spec:** `docs/superpowers/specs/2026-03-28-adaptive-model-routing.md`

---

### Task 1: Add domain types for routing

**Files:**
- Create: `internal/core/domain/routing.go`

- [ ] **Step 1: Create routing domain types**

Create `internal/core/domain/routing.go`:

```go
package domain

// ComplexityTier represents the complexity level of a user request.
type ComplexityTier string

const (
	TierSimple  ComplexityTier = "simple"
	TierComplex ComplexityTier = "complex"
	TierCode    ComplexityTier = "code"
)

// ModelRouting maps complexity tiers to model names.
type ModelRouting struct {
	Simple  string `json:"simple"`
	Complex string `json:"complex"`
	Code    string `json:"code"`
}

// ModelFor returns the model name for a given tier, falling back to Simple.
func (r ModelRouting) ModelFor(tier ComplexityTier) string {
	switch tier {
	case TierCode:
		if r.Code != "" {
			return r.Code
		}
		return r.Complex
	case TierComplex:
		if r.Complex != "" {
			return r.Complex
		}
	}
	if r.Simple != "" {
		return r.Simple
	}
	return r.Complex
}

// ModelInfo describes an available LLM model.
type ModelInfo struct {
	Name      string
	SizeBytes int64
}
```

- [ ] **Step 2: Run vet**

Run: `go vet ./internal/core/domain/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/core/domain/routing.go
git commit -m "feat(domain): add ComplexityTier, ModelRouting, ModelInfo types"
```

---

### Task 2: Implement Ollama ModelDiscovery

**Files:**
- Create: `internal/infrastructure/llm/ollama/discovery.go`
- Create: `internal/infrastructure/llm/ollama/discovery_test.go`

- [ ] **Step 1: Write test**

Create `internal/infrastructure/llm/ollama/discovery_test.go`:

```go
package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscovery_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("expected /api/tags, got %s", r.URL.Path)
		}
		resp := map[string]any{
			"models": []map[string]any{
				{"name": "llama3.1:8b", "size": 4_000_000_000},
				{"name": "qwen3.5:9b", "size": 6_000_000_000},
				{"name": "qwen-coder:7b", "size": 4_500_000_000},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	d := NewDiscovery(server.URL, server.Client())
	models, err := d.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}
	if models[0].Name != "llama3.1:8b" {
		t.Errorf("models[0].Name = %q, want %q", models[0].Name, "llama3.1:8b")
	}
}

func TestDiscovery_ListModels_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	d := NewDiscovery(server.URL, server.Client())
	_, err := d.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
```

Run: `go test ./internal/infrastructure/llm/ollama/ -run TestDiscovery -v`
Expected: FAIL

- [ ] **Step 2: Implement Discovery**

Create `internal/infrastructure/llm/ollama/discovery.go`:

```go
package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// Discovery lists available models from Ollama.
type Discovery struct {
	baseURL    string
	httpClient *http.Client
}

func NewDiscovery(baseURL string, httpClient *http.Client) *Discovery {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Discovery{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (d *Discovery) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama list models: status %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}

	models := make([]domain.ModelInfo, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, domain.ModelInfo{
			Name:      m.Name,
			SizeBytes: m.Size,
		})
	}
	return models, nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/infrastructure/llm/ollama/ -run TestDiscovery -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/llm/ollama/discovery.go internal/infrastructure/llm/ollama/discovery_test.go
git commit -m "feat(ollama): model auto-discovery via /api/tags"
```

---

### Task 3: Implement ComplexityClassifier

**Files:**
- Create: `internal/core/usecase/complexity.go`
- Create: `internal/core/usecase/complexity_test.go`

- [ ] **Step 1: Write tests**

Create `internal/core/usecase/complexity_test.go`:

```go
package usecase

import (
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestClassifyComplexity(t *testing.T) {
	tests := []struct {
		name    string
		message string
		intent  Intent
		want    domain.ComplexityTier
	}{
		{name: "code intent", message: "напиши код на Python", intent: IntentCode, want: domain.TierCode},
		{name: "short simple", message: "привет", intent: IntentGeneral, want: domain.TierSimple},
		{name: "short question", message: "как дела?", intent: IntentGeneral, want: domain.TierSimple},
		{name: "complex keyword RU", message: "сравни подходы к архитектуре микросервисов", intent: IntentGeneral, want: domain.TierComplex},
		{name: "complex keyword EN", message: "analyze the performance of this algorithm", intent: IntentGeneral, want: domain.TierComplex},
		{name: "explain why", message: "explain why transformers work better than RNNs", intent: IntentGeneral, want: domain.TierComplex},
		{name: "medium length general", message: "расскажи что такое dependency injection в Go", intent: IntentKnowledge, want: domain.TierUncertain},
		{name: "write plan", message: "напиши план обучения машинному обучению", intent: IntentGeneral, want: domain.TierComplex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyComplexityRules(tt.message, tt.intent)
			if got != tt.want {
				t.Errorf("classifyComplexityRules(%q, %q) = %q, want %q", tt.message, tt.intent, got, tt.want)
			}
		})
	}
}

func TestAutoAssignTiers(t *testing.T) {
	models := []domain.ModelInfo{
		{Name: "llama3.1:8b", SizeBytes: 4_000_000_000},
		{Name: "qwen3.5:9b", SizeBytes: 9_000_000_000},
		{Name: "qwen-coder:7b", SizeBytes: 4_500_000_000},
	}

	routing := AutoAssignTiers(models, "llama3.1:8b")

	if routing.Code != "qwen-coder:7b" {
		t.Errorf("Code = %q, want %q", routing.Code, "qwen-coder:7b")
	}
	if routing.Complex != "qwen3.5:9b" {
		t.Errorf("Complex = %q, want %q", routing.Complex, "qwen3.5:9b")
	}
	if routing.Simple != "llama3.1:8b" {
		t.Errorf("Simple = %q, want %q", routing.Simple, "llama3.1:8b")
	}
}

func TestAutoAssignTiers_SingleModel(t *testing.T) {
	models := []domain.ModelInfo{
		{Name: "llama3.1:8b", SizeBytes: 4_000_000_000},
	}

	routing := AutoAssignTiers(models, "llama3.1:8b")

	if routing.Simple != "llama3.1:8b" || routing.Complex != "llama3.1:8b" || routing.Code != "llama3.1:8b" {
		t.Errorf("single model should fill all tiers, got %+v", routing)
	}
}

func TestAutoAssignTiers_Empty(t *testing.T) {
	routing := AutoAssignTiers(nil, "default-model")

	if routing.Simple != "default-model" || routing.Complex != "default-model" {
		t.Errorf("empty models should use default, got %+v", routing)
	}
}
```

Run: `go test ./internal/core/usecase/ -run TestClassifyComplexity -v`
Expected: FAIL

- [ ] **Step 2: Implement ComplexityClassifier**

Create `internal/core/usecase/complexity.go`:

```go
package usecase

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

const (
	// TierUncertain means rule-based classifier is not confident.
	// LLM fallback should be used.
	TierUncertain domain.ComplexityTier = "uncertain"
)

var complexityKeywords = []string{
	"сравни", "объясни разницу", "проанализируй", "напиши план",
	"compare", "analyze", "explain why", "explain the difference",
	"evaluate", "design", "разработай", "спроектируй", "оцени",
}

// classifyComplexityRules uses heuristics to determine request complexity.
// Returns TierUncertain if no rule matches with high confidence.
func classifyComplexityRules(message string, intent Intent) domain.ComplexityTier {
	// Intent-based fast path.
	if intent == IntentCode {
		return domain.TierCode
	}

	lower := strings.ToLower(message)
	runeCount := utf8.RuneCountInString(message)

	// Short messages without question complexity → simple.
	if runeCount < 30 {
		return domain.TierSimple
	}

	// Complexity keywords → complex.
	for _, kw := range complexityKeywords {
		if strings.Contains(lower, kw) {
			return domain.TierComplex
		}
	}

	return TierUncertain
}

const autoAssignLargeModelThreshold = 8_000_000_000 // ~8GB ≈ 13B+ params

// AutoAssignTiers automatically maps discovered models to complexity tiers.
func AutoAssignTiers(models []domain.ModelInfo, defaultModel string) domain.ModelRouting {
	routing := domain.ModelRouting{
		Simple:  defaultModel,
		Complex: defaultModel,
		Code:    defaultModel,
	}

	if len(models) == 0 {
		return routing
	}

	// Sort by size descending — largest first.
	sorted := make([]domain.ModelInfo, len(models))
	copy(sorted, models)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SizeBytes > sorted[j].SizeBytes
	})

	// Assign code model (name contains "code" or "coder").
	for _, m := range sorted {
		nameLower := strings.ToLower(m.Name)
		if strings.Contains(nameLower, "code") {
			routing.Code = m.Name
			break
		}
	}

	// Assign complex = largest model.
	routing.Complex = sorted[0].Name

	// Assign simple = smallest model.
	routing.Simple = sorted[len(sorted)-1].Name

	// If code wasn't found, use complex.
	if routing.Code == defaultModel {
		routing.Code = routing.Complex
	}

	return routing
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/core/usecase/ -run "TestClassifyComplexity|TestAutoAssignTiers" -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/core/usecase/complexity.go internal/core/usecase/complexity_test.go
git commit -m "feat(usecase): complexity classifier with rule-based heuristics and auto-assign tiers"
```

---

### Task 4: Add MODEL_ROUTING config + parser

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add config field and parser**

In `internal/config/config.go`, add to Config struct (after `AgentIntentRouterEnabled`):

```go
ModelRouting string // JSON: {"simple":"llama3.1:8b","complex":"qwen3.5:9b","code":"qwen-coder:7b"}
```

Add to `Load()`:

```go
ModelRouting: os.Getenv("MODEL_ROUTING"),
```

Add parser function (add `"github.com/kirillkom/personal-ai-assistant/internal/core/domain"` import if not present, and `"encoding/json"`):

```go
func ParseModelRouting(raw string) *domain.ModelRouting {
	if raw == "" {
		return nil
	}
	var result domain.ModelRouting
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return &result
}
```

- [ ] **Step 2: Run vet**

Run: `go vet ./internal/config/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add MODEL_ROUTING env var with JSON parser"
```

---

### Task 5: Integrate routing into AgentChatUseCase

**Files:**
- Modify: `internal/core/usecase/agent_chat.go`
- Modify: `internal/bootstrap/bootstrap.go`

- [ ] **Step 1: Add ModelRouting field to AgentChatUseCase**

In `agent_chat.go`, add field to the struct (find the struct definition and add):

```go
modelRouting *domain.ModelRouting
```

Add a setter method:

```go
func (uc *AgentChatUseCase) SetModelRouting(r *domain.ModelRouting) {
	uc.modelRouting = r
}
```

- [ ] **Step 2: Add complexity-based routing in Complete**

In the `Complete` method, after intent classification (around line 187 where `uc.agentMetrics.IntentClassifications` is called), add:

```go
// Adaptive model routing based on complexity.
if uc.modelRouting != nil {
	tier := classifyComplexityRules(lastUserMessage, intent)
	if tier == TierUncertain {
		// Default uncertain to complex (safer).
		tier = domain.TierComplex
	}
	model := uc.modelRouting.ModelFor(tier)
	ctx = routing.WithProvider(ctx, model)
	slog.Info("adaptive_routing", "tier", tier, "model", model, "intent", intent)
}
```

Add import for `routing` package:

```go
"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/routing"
```

- [ ] **Step 3: Wire in bootstrap**

In `internal/bootstrap/bootstrap.go`, after creating `agentUC`, add:

```go
// Adaptive model routing.
if routingCfg := config.ParseModelRouting(cfg.ModelRouting); routingCfg != nil {
	agentUC.SetModelRouting(routingCfg)
} else {
	// Auto-discover from Ollama.
	discovery := ollama.NewDiscovery(cfg.OllamaURL, nil)
	if models, err := discovery.ListModels(ctx); err == nil && len(models) > 1 {
		autoRouting := usecase.AutoAssignTiers(models, cfg.OllamaGenModel)
		agentUC.SetModelRouting(&autoRouting)
		slog.Info("auto_model_routing", "simple", autoRouting.Simple, "complex", autoRouting.Complex, "code", autoRouting.Code)
	}
}
```

- [ ] **Step 4: Run build and tests**

Run: `go build ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/usecase/agent_chat.go internal/bootstrap/bootstrap.go
git commit -m "feat(agent): adaptive model routing with auto-discovery integration"
```

---

### Task 6: Final verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1 -v 2>&1 | grep -E "FAIL|ok"`
Expected: All PASS

- [ ] **Step 2: Run vet**

Run: `go vet ./...`
Expected: Clean

- [ ] **Step 3: Push**

```bash
git push origin main
```
