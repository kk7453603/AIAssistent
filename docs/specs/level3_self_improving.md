# SPEC: Self-Improving Agent

## Goal
Analyze agent errors, fallbacks, and low-quality responses to automatically suggest and apply improvements (prompt tuning, tool selection, routing rules).

## Current State
- Agent tracks fallback reasons in metrics (`agentMetrics`)
- Tool execution errors are logged
- No systematic error analysis or improvement loop

## Architecture

### New Package: `internal/core/usecase/improvement/`

```
internal/core/usecase/improvement/
  analyzer.go          — error pattern analyzer
  suggestions.go       — improvement suggestion generator
  feedback.go          — user feedback collection
  analyzer_test.go
```

### Error Pattern Analyzer

```go
type ErrorPattern struct {
    Type        string    `json:"type"`       // "tool_failure", "timeout", "low_quality", "fallback"
    Tool        string    `json:"tool,omitempty"`
    Provider    string    `json:"provider,omitempty"`
    Count       int       `json:"count"`
    LastSeen    time.Time `json:"last_seen"`
    Examples    []string  `json:"examples"`   // sample error messages
}

type Analyzer struct {
    patterns   map[string]*ErrorPattern   // key = type+tool+provider
    mu         sync.RWMutex
}

func (a *Analyzer) Record(event ErrorEvent)
func (a *Analyzer) Analyze() []Suggestion
```

### Suggestions

```go
type Suggestion struct {
    ID          string   `json:"id"`
    Type        string   `json:"type"`       // "prompt_change", "routing_rule", "tool_config", "timeout_adjust"
    Description string   `json:"description"`
    Confidence  float64  `json:"confidence"` // 0-1
    AutoApply   bool     `json:"auto_apply"` // safe to apply without confirmation
    Payload     string   `json:"payload"`    // JSON with specific changes
}
```

Example suggestions:
- "Tool X fails 40% of the time → increase timeout from 30s to 60s"
- "Provider Y has 95th percentile latency of 15s for code tasks → route code to Provider Z"
- "Knowledge search returns empty for 30% of queries → enable query expansion"

### Feedback Loop
- Agent appends quality signals to conversation metadata
- User can rate responses (thumbs up/down via API)
- Low-rated responses are analyzed for patterns

### Integration
- `agent_chat.go`: after each response, call `analyzer.Record()` with execution stats
- Periodic analysis job (via scheduler) generates suggestions
- API endpoint to view/apply suggestions

### API
- `GET /v1/agent/suggestions` — list current suggestions
- `POST /v1/agent/suggestions/{id}/apply` — apply a suggestion
- `POST /v1/agent/feedback` — submit user feedback

### Config
```
SELF_IMPROVE_ENABLED=false
SELF_IMPROVE_MIN_SAMPLES=20          # minimum events before generating suggestions
SELF_IMPROVE_AUTO_APPLY=false        # auto-apply safe suggestions
```

## Tests
- Unit: pattern detection from mock error events
- Unit: suggestion generation logic
