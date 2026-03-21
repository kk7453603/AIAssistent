# Group A: Agent Intelligence — Design Spec

## Problem

Agent performance is 9/10 after native function calling upgrade, but:
1. Agent doesn't know filesystem paths → passes wrong paths to MCP tools
2. Keyword intent classifier falls back to "general" on ambiguous queries
3. Large tool outputs (5KB+) bloat context window, reducing reasoning quality
4. Agent treats recoverable errors (wrong path) as final failures

## Improvements

### A1: Filesystem Context in System Prompt

**Goal:** Agent knows available filesystem paths without guessing.

**Mechanism:** At first request (lazy init), call `list_allowed_directories` and `list_directory` (top level only) on the filesystem MCP server. Cache result in `AgentChatUseCase` with 60s TTL.

Inject into system prompt:
```
Available filesystem paths:
/allowed/vaults/ — contains: ML/, Rust/, architector/, neovim/, vpn/
```

**Files:**
- Modify: `internal/core/usecase/agent_chat.go` — add `probeFilesystem()`, inject into `buildSystemPrompt()`
- Add field `fsContextCache string` and `fsContextExpiry time.Time` to `AgentChatUseCase`

**Fallback:** If MCP filesystem not connected, skip probe silently.

### A2: LLM Intent Router Fallback

**Goal:** When keyword classifier returns `IntentGeneral`, use LLM to classify.

**Mechanism:**
```go
func (uc *AgentChatUseCase) classifyIntent(ctx context.Context, msg string) Intent {
    fast := classifyIntentByKeywords(msg)
    if fast != IntentGeneral || !uc.limits.IntentRouterEnabled {
        return fast
    }
    // LLM fallback (~1-2s, uses gen model not planner)
    result, err := uc.querySvc.GenerateFromPrompt(ctx, classifyIntentLLMPrompt(msg))
    if err != nil {
        return IntentGeneral
    }
    return parseIntent(result)
}
```

LLM prompt (~50 tokens input):
```
Classify into ONE word: knowledge, code, file, task, web, general
Request: {msg}
Category:
```

**Files:**
- Modify: `internal/core/usecase/intent.go` — add `classifyIntent()` method on usecase, `classifyIntentLLMPrompt()`
- Modify: `internal/core/usecase/agent_chat.go` — replace `classifyIntentByKeywords()` call with `uc.classifyIntent()`

### A3: Tool Result Summarization

**Goal:** Keep tool outputs in context window manageable.

**Mechanism:** After executing a tool, if output > 2048 bytes, summarize via LLM before adding to chat messages.

```go
func (uc *AgentChatUseCase) maybeSummarize(ctx context.Context, toolOutput string) string {
    if len(toolOutput) <= 2048 {
        return toolOutput
    }
    prompt := fmt.Sprintf("Summarize preserving key data, numbers, paths:\n\n%s", toolOutput[:4096])
    summary, err := uc.querySvc.GenerateFromPrompt(ctx, prompt)
    if err != nil {
        return toolOutput[:2048] + "\n[...truncated]"
    }
    return summary
}
```

**Files:**
- Modify: `internal/core/usecase/agent_chat.go` — add `maybeSummarize()`, call after `executeToolCall()`

### A4: Retry with Error Context

**Goal:** Agent recovers from wrong-path errors by using FS context.

**Mechanism:** Already partially working — tool errors go into chat messages. Enhancement:
- When tool returns error containing "path outside", "not found", "access denied" → append FS context hint to the error message
- Do NOT count recoverable errors toward `fallbackReason`

```go
if isRecoverableToolError(event.Output) && uc.fsContextCache != "" {
    event.Output += "\n\nHint: " + uc.fsContextCache
}
```

**Files:**
- Modify: `internal/core/usecase/agent_chat.go` — add `isRecoverableToolError()`, hint injection

## Verification

1. `go test ./...` — pass
2. Test: "Покажи файлы в ML vault" → agent uses `/allowed/vaults/ML` (not `/ML vault`)
3. Test: "сравни мой прогресс по модулям" → LLM intent classifies correctly
4. Test: read large file → output summarized before next iteration
5. Test: wrong path error → agent retries with correct path
