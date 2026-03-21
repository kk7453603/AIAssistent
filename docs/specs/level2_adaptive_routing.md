# SPEC: Adaptive Model Routing

## Goal
Automatically select the optimal LLM provider/model based on task type (intent), complexity, and historical performance. Extends the existing `routing.Generator` with intent-aware and performance-aware routing.

## Current State
- `internal/infrastructure/llm/routing/` — context-based provider routing (manual selection via `metadata.model`)
- `internal/core/usecase/intent.go` — intent classification (knowledge, code, file, task, web, general)
- `internal/infrastructure/llm/fallback/` — simple primary→fallback chain
- Extra providers configured via `LLM_EXTRA_PROVIDERS` env var

## Architecture

### New/Modified Files

```
internal/infrastructure/llm/routing/
  adaptive.go          — AdaptiveGenerator with routing rules
  adaptive_test.go     — unit tests
  performance.go       — performance tracker (latency, error rate, quality)
  performance_test.go  — unit tests
  rules.go             — routing rule definitions & matching
  rules_test.go        — unit tests
```

### Routing Rules

```go
// rules.go

type RoutingRule struct {
    Intent          usecase.Intent   `json:"intent"`
    ComplexityMin   int              `json:"complexity_min"`   // 0-10 scale
    ComplexityMax   int              `json:"complexity_max"`
    Provider        string           `json:"provider"`         // provider name
    Priority        int              `json:"priority"`         // lower = higher priority
}

// Default rules (configurable via env/JSON):
// intent=code,       complexity>=7 → large model (e.g., openrouter/claude)
// intent=code,       complexity<7  → fast model (e.g., groq/llama)
// intent=knowledge,  any           → default local model (ollama)
// intent=general,    complexity<4  → fast model
// intent=general,    complexity>=4 → default model
// intent=web,        any           → fast model (latency-sensitive)
// intent=task,       any           → fast model (simple CRUD)
```

### Complexity Estimator

```go
// adaptive.go

func estimateComplexity(message string, history []domain.ChatMessage) int
```

Heuristics:
- Message length (short=1, medium=3, long=5)
- Keyword markers: "explain in detail", "compare", "analyze" → +2
- Multi-part questions (contains "and", "also", numbered list) → +2
- Code block presence → +1
- Conversation depth (many turns) → +1

### Performance Tracker

```go
// performance.go

type ProviderStats struct {
    Provider       string
    TotalCalls     int64
    TotalErrors    int64
    MeanLatencyMs  float64
    P95LatencyMs   float64
    ErrorRate      float64       // errors / total
    LastUpdated    time.Time
}

type PerformanceTracker struct {
    stats map[string]*ProviderStats  // guarded by mutex
}

func (pt *PerformanceTracker) Record(provider string, latency time.Duration, err error)
func (pt *PerformanceTracker) Stats(provider string) ProviderStats
func (pt *PerformanceTracker) BestFor(candidates []string) string  // lowest error rate, then latency
```

### AdaptiveGenerator

```go
// adaptive.go

type AdaptiveGenerator struct {
    providers   map[string]ports.AnswerGenerator
    rules       []RoutingRule
    tracker     *PerformanceTracker
    fallback    string   // default provider
    logger      *slog.Logger
}

func (ag *AdaptiveGenerator) resolve(ctx context.Context, intent usecase.Intent, complexity int) ports.AnswerGenerator
```

Resolution order:
1. If context has explicit provider → use it (backward compat)
2. Match intent + complexity against rules (sorted by priority)
3. Among matching rules, prefer provider with best performance stats
4. Fallback to default provider

### Integration with AgentChatUseCase

`agent_chat.go` already classifies intent. Pass intent via context to AdaptiveGenerator:

```go
// routing/context.go — extend with intent key
type contextKey string
const intentKey contextKey = "routing.intent"
const complexityKey contextKey = "routing.complexity"

func WithIntent(ctx context.Context, intent string) context.Context
func IntentFrom(ctx context.Context) string
func WithComplexity(ctx context.Context, complexity int) context.Context
func ComplexityFrom(ctx context.Context) int
```

### Config

```
ROUTING_ADAPTIVE_ENABLED=true
ROUTING_RULES=                      # optional JSON override for rules
ROUTING_PERFORMANCE_WINDOW=100      # sliding window for stats (last N calls)
```

### Bootstrap Changes

In `internal/bootstrap/bootstrap.go`:
- When `ROUTING_ADAPTIVE_ENABLED=true`, wrap the routing.Generator with AdaptiveGenerator
- Initialize PerformanceTracker
- Parse rules from `ROUTING_RULES` or use defaults

## Metrics (Prometheus)
- `paa_routing_decision_total{provider,intent,complexity_bucket}` — counter
- `paa_provider_latency_seconds{provider}` — histogram
- `paa_provider_errors_total{provider}` — counter

## Tests
- Unit: rule matching, complexity estimation, performance tracking
- Unit: AdaptiveGenerator.resolve() with various intent/complexity combos
- Integration: verify routing decisions propagate through agent chat flow
