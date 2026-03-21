# Group B: Agent Infrastructure — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SSE streaming of tool events, parallel tool execution, and result caching to improve agent responsiveness and efficiency.

**Architecture:** Three independent features. SSE streaming adds a callback to the agent loop. Parallel execution uses errgroup for concurrent tool calls. Caching wraps tool execution with TTL-based in-memory cache.

**Tech Stack:** Go, SSE, errgroup, sync.Map

**Spec:** `docs/superpowers/specs/2026-03-21-group-b-agent-infrastructure.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/core/usecase/tool_cache.go` | Create | TTL-based in-memory tool result cache |
| `internal/core/usecase/tool_cache_test.go` | Create | Cache tests |
| `internal/core/usecase/agent_chat.go` | Modify | Add callback, parallel execution, cache integration |
| `internal/core/domain/agent_memory.go` | Modify | Add ToolStatusCallback type |
| `internal/core/ports/inbound.go` | Modify | Update AgentChatService interface |
| `internal/adapters/http/router_openai.go` | Modify | Pass SSE callback when streaming |

---

### Task B1: Tool Result Cache

**Files:**
- Create: `internal/core/usecase/tool_cache.go`
- Create: `internal/core/usecase/tool_cache_test.go`
- Modify: `internal/core/usecase/agent_chat.go`

- [ ] **Step 1: Write cache tests**

```go
package usecase

import (
	"testing"
	"time"
)

func TestToolCache_SetGet(t *testing.T) {
	c := newToolCache()
	c.set("list_directory", `{"path":"/vaults"}`, "result", 60*time.Second)
	got, ok := c.get("list_directory", `{"path":"/vaults"}`)
	if !ok || got != "result" {
		t.Errorf("expected cache hit, got ok=%v result=%q", ok, got)
	}
}

func TestToolCache_Miss(t *testing.T) {
	c := newToolCache()
	_, ok := c.get("list_directory", `{"path":"/vaults"}`)
	if ok {
		t.Error("expected cache miss")
	}
}

func TestToolCache_Expired(t *testing.T) {
	c := newToolCache()
	c.set("list_directory", `{"path":"/vaults"}`, "result", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	_, ok := c.get("list_directory", `{"path":"/vaults"}`)
	if ok {
		t.Error("expected cache miss after expiry")
	}
}

func TestToolCache_NoCacheForExecute(t *testing.T) {
	ttl := cacheTTLForTool("execute_python")
	if ttl > 0 {
		t.Error("execute_python should not be cached")
	}
}
```

- [ ] **Step 2: Implement cache**

```go
package usecase

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type toolCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	output  string
	expires time.Time
}

func newToolCache() *toolCache {
	return &toolCache{entries: make(map[string]cacheEntry)}
}

func (c *toolCache) get(tool, argsKey string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := cacheKey(tool, argsKey)
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expires) {
		return "", false
	}
	return entry.output, true
}

func (c *toolCache) set(tool, argsKey, output string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[cacheKey(tool, argsKey)] = cacheEntry{
		output:  output,
		expires: time.Now().Add(ttl),
	}
}

func cacheKey(tool, argsKey string) string {
	return tool + ":" + argsKey
}

func argsToKey(args map[string]any) string {
	data, _ := json.Marshal(args)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8])
}

// cacheTTLForTool returns the cache TTL for a tool, or 0 if not cacheable.
func cacheTTLForTool(tool string) time.Duration {
	switch tool {
	case "list_allowed_directories":
		return 5 * time.Minute
	case "list_directory", "list_directory_with_sizes", "directory_tree":
		return 60 * time.Second
	case "read_file", "read_text_file", "get_file_info":
		return 30 * time.Second
	case "search_files":
		return 30 * time.Second
	default:
		return 0 // no cache for execute_*, knowledge_search, web_search, write operations
	}
}
```

- [ ] **Step 3: Add cache field to AgentChatUseCase and wire in**

In `agent_chat.go`, add field `toolResultCache *toolCache` to struct, init in constructor `newToolCache()`. Before `executeToolCall()`:
```go
if ttl := cacheTTLForTool(tc.Function.Name); ttl > 0 {
    argsKey := argsToKey(tc.Function.Arguments)
    if cached, ok := uc.toolResultCache.get(tc.Function.Name, argsKey); ok {
        event = domain.AgentToolEvent{Tool: tc.Function.Name, Status: "ok", Output: cached}
        // skip actual execution
        continue
    }
}
```
After successful execution, cache the result.

- [ ] **Step 4: Run all tests**

- [ ] **Step 5: Commit**

---

### Task B2: Parallel Tool Execution

**Files:**
- Modify: `internal/core/usecase/agent_chat.go`

- [ ] **Step 1: Import errgroup**

Add `"golang.org/x/sync/errgroup"` to imports.

- [ ] **Step 2: Replace sequential tool execution with parallel**

In the tool call loop, when `len(chatResult.ToolCalls) > 1`:
```go
g, gCtx := errgroup.WithContext(loopCtx)
events := make([]domain.AgentToolEvent, len(chatResult.ToolCalls))
for i, tc := range chatResult.ToolCalls {
    g.Go(func() error {
        toolCtx, toolCancel := context.WithTimeout(gCtx, uc.limits.ToolTimeout)
        defer toolCancel()
        ev, err := uc.executeToolCall(toolCtx, userID, tc, lastUserMessage)
        if err != nil {
            errorPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
            ev = domain.AgentToolEvent{Tool: tc.Function.Name, Status: "error", Output: string(errorPayload)}
        }
        events[i] = ev
        return nil
    })
}
_ = g.Wait()
// Process all events
for _, event := range events { ... }
```

Keep single tool call path unchanged (no goroutine overhead for 1 call).

- [ ] **Step 3: Run all tests**

- [ ] **Step 4: Commit**

---

### Task B3: SSE Streaming Tool Events

**Files:**
- Modify: `internal/core/domain/agent_memory.go`
- Modify: `internal/core/usecase/agent_chat.go`
- Modify: `internal/adapters/http/router_openai.go` (or wherever SSE streaming lives)

- [ ] **Step 1: Add callback type to domain**

```go
// ToolStatusCallback is called during the agent loop to report tool execution progress.
type ToolStatusCallback func(toolName string, status string) // status: "running", "done", "error"
```

- [ ] **Step 2: Add callback param to Complete()**

Change signature:
```go
func (uc *AgentChatUseCase) Complete(ctx context.Context, req domain.AgentChatRequest, onToolStatus domain.ToolStatusCallback) (*domain.AgentRunResult, error)
```

Before each `executeToolCall`, call `onToolStatus(tc.Function.Name, "running")`. After, call `onToolStatus(tc.Function.Name, "done")` or `"error"`.

Guard with nil check: `if onToolStatus != nil { onToolStatus(...) }`.

- [ ] **Step 3: Update interface and callers**

Update `AgentChatService` interface in `ports/inbound.go`. Update all callers to pass `nil` (non-streaming) or the SSE callback (streaming).

In the HTTP streaming handler, create callback that writes SSE:
```go
onToolStatus := func(tool, status string) {
    chunk := fmt.Sprintf(`{"choices":[{"delta":{"tool_status":{"tool":"%s","status":"%s"}}}]}`, tool, status)
    fmt.Fprintf(w, "data: %s\n\n", chunk)
    flusher.Flush()
}
```

- [ ] **Step 4: Run all tests** (update test mocks for new signature)

- [ ] **Step 5: Commit**
