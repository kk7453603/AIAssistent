# Group B: Agent Infrastructure — Design Spec

## Improvements

### B1: Streaming Tool Events (SSE)

**Goal:** User sees real-time progress during agent execution instead of waiting 20-60s.

**Mechanism:** The existing `/v1/chat/completions` already supports SSE streaming for the final answer. Extend it to emit **tool status events** during the agent loop.

Events emitted as SSE `data:` lines with a custom field:
```json
{"choices":[{"delta":{"tool_status":{"tool":"knowledge_search","status":"running"}}}]}
{"choices":[{"delta":{"tool_status":{"tool":"knowledge_search","status":"done"}}}]}
{"choices":[{"delta":{"content":"Вот результат..."}}]}
```

**Implementation:** Change `AgentChatService.Complete()` to accept an optional callback:
```go
type ToolStatusCallback func(tool string, status string)
```

The HTTP handler passes a callback that writes SSE events. Non-streaming callers pass nil.

**Files:**
- Modify: `internal/core/usecase/agent_chat.go` — add callback param to `Complete()`
- Modify: `internal/core/ports/inbound.go` — update `AgentChatService` interface
- Modify: `internal/adapters/http/router_openai.go` — pass SSE callback in streaming mode
- Modify: `internal/core/domain/agent_memory.go` — add callback type

### B2: Parallel Tool Execution

**Goal:** When LLM returns multiple independent tool_calls, execute them concurrently.

**Mechanism:** Use `errgroup` to run tool calls in parallel:
```go
if len(chatResult.ToolCalls) > 1 {
    g, gCtx := errgroup.WithContext(toolCtx)
    results := make([]domain.AgentToolEvent, len(chatResult.ToolCalls))
    for i, tc := range chatResult.ToolCalls {
        g.Go(func() error {
            results[i], _ = uc.executeToolCall(gCtx, userID, tc, lastUserMessage)
            return nil
        })
    }
    g.Wait()
}
```

**Files:**
- Modify: `internal/core/usecase/agent_chat.go` — parallel execution in tool call loop
- Modify: `go.mod` — add `golang.org/x/sync` (already present as indirect)

### B3: Tool Result Caching

**Goal:** Avoid redundant MCP calls for unchanged data.

**Mechanism:** In-memory cache with TTL for specific tools:
- `list_directory` → 60s TTL
- `list_allowed_directories` → 300s TTL
- `read_file` → 30s TTL (content may change)
- `knowledge_search` → no cache (always fresh)
- `execute_python` → no cache (side effects)

Cache key: `tool_name + hash(arguments)`. Implementation: simple `sync.Map` with expiry timestamps.

```go
type toolCache struct {
    mu      sync.RWMutex
    entries map[string]cacheEntry
}
type cacheEntry struct {
    output  string
    expires time.Time
}
```

**Files:**
- Create: `internal/core/usecase/tool_cache.go` — cache implementation
- Modify: `internal/core/usecase/agent_chat.go` — check cache before `executeToolCall()`

## Verification

1. `go test ./...` — pass
2. B1: SSE stream shows tool status events in real-time
3. B2: 2 independent tool calls complete faster than sequential
4. B3: repeated `list_directory` calls hit cache (visible in logs)
