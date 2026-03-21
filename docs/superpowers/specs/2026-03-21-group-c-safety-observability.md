# Group C: Safety & Observability — Design Spec

## Improvements

### C1: Observability Metrics

**Goal:** Prometheus metrics for agent performance monitoring.

**New metrics** (in `internal/observability/metrics/`):
- `paa_agent_intent_classification_total{intent}` — counter per intent type
- `paa_agent_tool_call_duration_seconds{tool,status}` — histogram
- `paa_agent_tool_call_total{tool,status}` — counter
- `paa_agent_iterations_total{conversation_id}` — histogram of iterations per request
- `paa_agent_request_duration_seconds` — histogram of total agent Complete() time
- `paa_mcp_client_connected{server}` — gauge (1=connected, 0=disconnected)

**Files:**
- Create: `internal/observability/metrics/agent_metrics.go` — metric definitions
- Modify: `internal/core/usecase/agent_chat.go` — instrument with metrics
- Modify: `internal/infrastructure/mcp/client.go` — connection gauge
- Modify: `deploy/monitoring/grafana/dashboards/` — add agent dashboard (optional)

### C2: Code Execution Guardrails

**Goal:** Prevent dangerous code execution patterns.

**Mechanism:** Before passing code to `execute_python`/`execute_bash`, check against deny patterns:

```go
var denyPatterns = []string{
    `rm\s+-rf\s+/`,           // rm -rf /
    `curl.*\|\s*bash`,        // curl | bash
    `wget.*\|\s*sh`,          // wget | sh
    `shutdown`, `reboot`,
    `mkfs`, `dd\s+if=`,
    `:(){ :|:& };:`,          // fork bomb
    `/etc/passwd`, `/etc/shadow`,
}
```

Return error if matched:
```
"Code execution blocked: potentially dangerous pattern detected. Please rephrase."
```

Also: add `--network none` to mcp-code-runner in docker-compose.yml to prevent network access from executed code.

**Files:**
- Create: `internal/core/usecase/guardrails.go` — pattern checker
- Create: `internal/core/usecase/guardrails_test.go` — tests
- Modify: `internal/core/usecase/agent_chat.go` — check before execute_python/execute_bash
- Modify: `docker-compose.yml` — add network isolation for code-runner

### C3: Cross-Session Tool Memory

**Goal:** Agent remembers tool results across sessions for continuity.

**Mechanism:** After a tool call produces useful output (status=ok, len>100), store a summary in the existing memory system (MemoryStore + MemoryVectorStore). On subsequent requests, relevant tool memories surface via the existing `memoryHits` mechanism.

```go
// After successful tool call with useful output:
if event.Status == "ok" && len(event.Output) > 100 {
    summary := fmt.Sprintf("Tool %s was used: %s", event.Tool, maybeSummarize(ctx, event.Output))
    // Store as MemorySummary → embed → index in Qdrant
}
```

This reuses the existing `maybePersistSummary` infrastructure. No new DB tables needed.

**Files:**
- Modify: `internal/core/usecase/agent_chat.go` — persist tool memories after execution
- Potentially modify: `internal/core/domain/agent_memory.go` — add ToolMemory type (or reuse MemorySummary)

## Verification

1. `go test ./...` — pass
2. C1: `curl localhost:8080/metrics | grep paa_agent` shows metrics
3. C2: "запусти rm -rf /" → blocked, "вычисли 2+2" → allowed
4. C3: After reading Progress.md, next session mentions "you previously read Progress.md"
