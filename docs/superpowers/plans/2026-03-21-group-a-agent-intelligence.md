# Group A: Agent Intelligence — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the agent aware of filesystem paths, improve intent classification with LLM fallback, summarize large tool outputs, and recover from tool errors automatically.

**Architecture:** Four focused improvements to the agent loop — each adds a small function called from `Complete()` in `agent_chat.go`. New logic lives in dedicated files (`intent.go`, `tool_helpers.go`) to keep `agent_chat.go` from growing further.

**Tech Stack:** Go, Ollama API, MCP client

**Spec:** `docs/superpowers/specs/2026-03-21-group-a-agent-intelligence.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/core/usecase/tool_helpers.go` | Create | FS probe, summarization, error recovery helpers |
| `internal/core/usecase/tool_helpers_test.go` | Create | Tests for helpers |
| `internal/core/usecase/intent.go` | Modify | Add LLM fallback classification |
| `internal/core/usecase/intent_test.go` | Modify | Add LLM fallback tests |
| `internal/core/usecase/agent_chat.go` | Modify | Wire helpers into agent loop |

---

### Task A1: Filesystem Context Probe + Cache

**Files:**
- Create: `internal/core/usecase/tool_helpers.go`
- Create: `internal/core/usecase/tool_helpers_test.go`
- Modify: `internal/core/usecase/agent_chat.go`

- [ ] **Step 1: Write test for `probeFilesystemContext`**

Create `tool_helpers_test.go`:
```go
package usecase

import (
	"context"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type mockToolRegistry struct {
	callResults map[string]string
}

func (m *mockToolRegistry) ListTools() []ports.ToolDefinition {
	return []ports.ToolDefinition{
		{Name: "list_allowed_directories", Source: "filesystem"},
	}
}
func (m *mockToolRegistry) IsBuiltIn(name string) bool { return false }
func (m *mockToolRegistry) CallMCPTool(_ context.Context, name string, _ map[string]any) (string, error) {
	return m.callResults[name], nil
}

func TestProbeFilesystemContext(t *testing.T) {
	reg := &mockToolRegistry{callResults: map[string]string{
		"list_allowed_directories": `Allowed directories:\n/allowed/vaults`,
		"list_directory":           `[DIR] ML\n[DIR] Rust\n[FILE] notes.md`,
	}}
	ctx := context.Background()
	result := probeFilesystemContext(ctx, reg)
	if result == "" {
		t.Fatal("expected non-empty FS context")
	}
	if !contains(result, "/allowed/vaults") {
		t.Errorf("expected path in result, got: %s", result)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}
func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test — verify it fails**

Run: `go test ./internal/core/usecase/ -run TestProbeFilesystem -v`
Expected: FAIL (function not defined)

- [ ] **Step 3: Implement `probeFilesystemContext` in `tool_helpers.go`**

```go
package usecase

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type fsContextCache struct {
	mu      sync.RWMutex
	context string
	expires time.Time
}

var fsCache = &fsContextCache{}

// probeFilesystemContext calls MCP filesystem tools to discover available paths.
// Results are cached for 60 seconds.
func probeFilesystemContext(ctx context.Context, registry ports.MCPToolRegistry) string {
	if registry == nil {
		return ""
	}

	fsCache.mu.RLock()
	if time.Now().Before(fsCache.expires) && fsCache.context != "" {
		defer fsCache.mu.RUnlock()
		return fsCache.context
	}
	fsCache.mu.RUnlock()

	// Check if filesystem tools are available
	hasFS := false
	for _, t := range registry.ListTools() {
		if t.Name == "list_allowed_directories" {
			hasFS = true
			break
		}
	}
	if !hasFS {
		return ""
	}

	// Probe allowed directories
	dirs, err := registry.CallMCPTool(ctx, "list_allowed_directories", nil)
	if err != nil || dirs == "" {
		return ""
	}

	// List top-level contents of each allowed directory
	var sb strings.Builder
	sb.WriteString("Available filesystem paths:\n")
	sb.WriteString(dirs)

	// Try to list top-level of each path mentioned
	for _, line := range strings.Split(dirs, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/") {
			contents, err := registry.CallMCPTool(ctx, "list_directory", map[string]any{"path": line})
			if err == nil && contents != "" {
				sb.WriteString(fmt.Sprintf("\nContents of %s:\n%s", line, contents))
			}
		}
	}

	result := sb.String()

	fsCache.mu.Lock()
	fsCache.context = result
	fsCache.expires = time.Now().Add(60 * time.Second)
	fsCache.mu.Unlock()

	return result
}
```

- [ ] **Step 4: Wire into `buildSystemPrompt` in `agent_chat.go`**

In `buildSystemPrompt()`, add after memory hits section:
```go
if uc.toolRegistry != nil {
    if fsCtx := probeFilesystemContext(ctx, uc.toolRegistry); fsCtx != "" {
        sb.WriteString("\n")
        sb.WriteString(fsCtx)
        sb.WriteString("\n")
    }
}
```

Note: `buildSystemPrompt` needs `ctx` and `registry` params added.

- [ ] **Step 5: Run tests — verify they pass**

Run: `go test ./internal/core/usecase/ -run TestProbeFilesystem -v`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `go build ./... && go test ./...`
Expected: all pass

- [ ] **Step 7: Commit**

```bash
git add internal/core/usecase/tool_helpers.go internal/core/usecase/tool_helpers_test.go internal/core/usecase/agent_chat.go
git commit -m "feat(agent): add filesystem context probe for path awareness"
```

---

### Task A2: LLM Intent Router Fallback

**Files:**
- Modify: `internal/core/usecase/intent.go`
- Modify: `internal/core/usecase/intent_test.go`
- Modify: `internal/core/usecase/agent_chat.go`

- [ ] **Step 1: Write test for LLM intent classification**

Add to `intent_test.go`:
```go
func TestClassifyIntentLLMPrompt(t *testing.T) {
	prompt := classifyIntentLLMPrompt("сравни мой прогресс по модулям")
	if !strings.Contains(prompt, "knowledge") || !strings.Contains(prompt, "code") {
		t.Error("prompt should list all categories")
	}
}

func TestParseIntent(t *testing.T) {
	tests := []struct{ input string; expected Intent }{
		{"knowledge", IntentKnowledge},
		{"  CODE  ", IntentCode},
		{"file\n", IntentFile},
		{"garbage", IntentGeneral},
	}
	for _, tt := range tests {
		if got := parseIntent(tt.input); got != tt.expected {
			t.Errorf("parseIntent(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

- [ ] **Step 3: Implement LLM intent functions in `intent.go`**

Add:
```go
func classifyIntentLLMPrompt(message string) string {
	return fmt.Sprintf(`Classify this user request into exactly ONE word from: knowledge, code, file, task, web, general

Request: %s
Category:`, message)
}

func parseIntent(raw string) Intent {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(s, "knowledge"):
		return IntentKnowledge
	case strings.HasPrefix(s, "code"):
		return IntentCode
	case strings.HasPrefix(s, "file"):
		return IntentFile
	case strings.HasPrefix(s, "task"):
		return IntentTask
	case strings.HasPrefix(s, "web"):
		return IntentWeb
	default:
		return IntentGeneral
	}
}
```

- [ ] **Step 4: Wire LLM fallback into agent_chat.go**

In `Complete()`, replace `classifyIntentByKeywords(lastUserMessage)` with:
```go
intent := classifyIntentByKeywords(lastUserMessage)
if intent == IntentGeneral && uc.limits.IntentRouterEnabled {
    if classified, err := uc.querySvc.GenerateFromPrompt(ctx, classifyIntentLLMPrompt(lastUserMessage)); err == nil {
        intent = parseIntent(classified)
    }
}
```

- [ ] **Step 5: Run all tests**

Run: `go build ./... && go test ./...`
Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/core/usecase/intent.go internal/core/usecase/intent_test.go internal/core/usecase/agent_chat.go
git commit -m "feat(agent): add LLM fallback for intent classification"
```

---

### Task A3: Tool Result Summarization

**Files:**
- Modify: `internal/core/usecase/tool_helpers.go`
- Modify: `internal/core/usecase/tool_helpers_test.go`
- Modify: `internal/core/usecase/agent_chat.go`

- [ ] **Step 1: Write test for `maybeSummarize`**

Add to `tool_helpers_test.go`:
```go
func TestMaybeSummarize_Short(t *testing.T) {
	result := maybeSummarize("short text", 2048)
	if result != "short text" {
		t.Errorf("short text should pass through unchanged")
	}
}

func TestMaybeSummarize_Long(t *testing.T) {
	long := strings.Repeat("word ", 1000) // ~5000 bytes
	result := maybeSummarize(long, 2048)
	if len(result) >= len(long) {
		t.Errorf("long text should be truncated, got len=%d", len(result))
	}
	if !strings.Contains(result, "[...truncated]") {
		t.Errorf("should contain truncation marker")
	}
}
```

- [ ] **Step 2: Implement `maybeSummarize` (simple truncation, no LLM needed for MVP)**

```go
// maybeSummarize truncates tool output if it exceeds maxLen bytes.
// A future improvement can use LLM summarization instead of truncation.
func maybeSummarize(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "\n\n[...truncated at " + fmt.Sprintf("%d", maxLen) + " bytes]"
}
```

- [ ] **Step 3: Wire into agent loop**

In `agent_chat.go`, after `executeToolCall()` returns `event`, add:
```go
event.Output = maybeSummarize(event.Output, 2048)
```

- [ ] **Step 4: Run all tests**

- [ ] **Step 5: Commit**

---

### Task A4: Retry with Error Context (FS Hint)

**Files:**
- Modify: `internal/core/usecase/tool_helpers.go`
- Modify: `internal/core/usecase/tool_helpers_test.go`

- [ ] **Step 1: Write test for `isRecoverableToolError`**

```go
func TestIsRecoverableToolError(t *testing.T) {
	tests := []struct{ input string; expected bool }{
		{`{"error":"Access denied - path outside allowed directories"}`, true},
		{`{"error":"file not found: /foo/bar"}`, true},
		{`{"error":"connection refused"}`, false},
		{`{"answer":"some result"}`, false},
	}
	for _, tt := range tests {
		if got := isRecoverableToolError(tt.input); got != tt.expected {
			t.Errorf("isRecoverableToolError(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
```

- [ ] **Step 2: Implement `isRecoverableToolError` and hint injection**

```go
func isRecoverableToolError(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "path outside") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "access denied") ||
		strings.Contains(lower, "no such file")
}

func addFSHintToError(output string, fsContext string) string {
	if fsContext == "" {
		return output
	}
	return output + "\n\nHint — available paths:\n" + fsContext
}
```

- [ ] **Step 3: Wire into agent loop**

In `agent_chat.go`, in the tool error handling section:
```go
if event.Status == "error" && isRecoverableToolError(event.Output) {
    fsCtx := probeFilesystemContext(loopCtx, uc.toolRegistry)
    event.Output = addFSHintToError(event.Output, fsCtx)
}
```

- [ ] **Step 4: Run all tests**

- [ ] **Step 5: Commit**

```bash
git add internal/core/usecase/tool_helpers.go internal/core/usecase/tool_helpers_test.go internal/core/usecase/agent_chat.go
git commit -m "feat(agent): add error recovery with filesystem hints"
```
