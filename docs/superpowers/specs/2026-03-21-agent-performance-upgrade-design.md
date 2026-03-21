# Agent Performance Upgrade — Design Spec

## Problem

PAA agent (agent_chat.go) scores ~6/10 on real tasks. Root causes:
1. **Free-form JSON planner** — LLM manually generates JSON, frequent parse failures
2. **Rigid cascade** — always starts with knowledge_search, ignores user intent
3. **1 tool per iteration** — complex tasks exhaust 6-iteration budget
4. **No plan caching** — replans from scratch each iteration

## Solution: Three Improvements

### 1. Native Function Calling (highest impact)

**Replace** `GenerateJSONFromPrompt` planner with Ollama `/api/chat` + `tools` parameter.

**Current flow:**
```
buildPlannerPrompt() → GenerateJSONFromPrompt() → parseAgentStep(raw JSON)
```

**New flow:**
```
buildChatMessages() → ChatWithTools(messages, tools) → structured ToolCall response
```

**Changes:**

New port method in `AnswerGenerator`:
```go
type ToolCallResult struct {
    ToolName  string
    Arguments map[string]any
}

type ChatToolsResult struct {
    Content   string           // text response (for "final" answers)
    ToolCalls []ToolCallResult // tool calls (empty if final answer)
}

// ChatWithTools sends messages to LLM with tool definitions, returns either
// a text response or structured tool calls.
ChatWithTools(ctx context.Context, messages []ChatMessage, tools []ToolSchema) (*ChatToolsResult, error)
```

New Ollama implementation using `/api/chat` endpoint:
```go
func (c *Client) chatWithTools(ctx context.Context, messages []ChatMessage, tools []ToolSchema) (*ChatToolsResult, error) {
    model := c.plannerModel
    if model == "" { model = c.genModel }

    reqBody := map[string]any{
        "model":    model,
        "messages": messages,  // system + conversation history + scratchpad
        "tools":    tools,     // JSON Schema tool definitions
        "stream":   false,
        "think":    false,
    }
    // POST /api/chat → response.message.tool_calls OR response.message.content
}
```

Tool definitions generated from `MCPToolRegistry.ListTools()`:
```go
func toolDefsFromRegistry(registry ports.MCPToolRegistry, webSearchAvailable bool) []ToolSchema {
    var schemas []ToolSchema
    for _, t := range registry.ListTools() {
        schemas = append(schemas, ToolSchema{
            Type: "function",
            Function: FunctionSchema{
                Name:        t.Name,
                Description: t.Description,
                Parameters:  t.InputSchema,
            },
        })
    }
    return schemas
}
```

Agent loop changes (`Complete` method):
- Replace `GenerateJSONFromPrompt(buildPlannerPrompt(...))` with `ChatWithTools(messages, tools)`
- Remove `parseAgentStep()` and `buildPlannerRepairPrompt()` — no longer needed
- If response has `ToolCalls` → execute them, add to scratchpad
- If response has `Content` → that's the final answer
- System prompt simplified: no JSON schemas, no cascade instructions embedded in prompt

**Eliminates:** JSON parse failures, repair loop, format confusion.

### 2. Intent Router (LLM-based via API)

Lightweight LLM classification call BEFORE the main planner to determine intent category.

**Implementation:**
```go
type Intent string

const (
    IntentKnowledge Intent = "knowledge"  // search vault/documents
    IntentCode      Intent = "code"       // execute python/bash
    IntentFile      Intent = "file"       // read/write/list files
    IntentTask      Intent = "task"       // manage tasks
    IntentWeb       Intent = "web"        // search the internet
    IntentGeneral   Intent = "general"    // answer from LLM knowledge
)
```

New function `classifyIntent`:
```go
func (uc *AgentChatUseCase) classifyIntent(ctx context.Context, message string) Intent {
    // 1. Fast keyword pre-check (0ms)
    if containsAny(message, "python", "код", "скрипт", "выполни", "execute", "bash") {
        return IntentCode
    }
    if containsAny(message, "файл", "прочитай", "каталог", "директори", "file", "read_file") {
        return IntentFile
    }
    // ... more patterns

    // 2. If ambiguous → LLM classification (1-2s)
    prompt := fmt.Sprintf(`Classify this user request into ONE category:
- knowledge: user asks about their notes/documents
- code: user wants to run code or do calculations
- file: user wants to read/write/browse files
- task: user wants to manage tasks/todos
- web: user needs internet search
- general: general question

Request: %s
Category:`, message)

    result, _ := uc.querySvc.GenerateFromPrompt(ctx, prompt)
    return parseIntent(result)
}
```

Intent modifies the system prompt for ChatWithTools:
- `IntentCode` → system prompt says "User wants code execution. Use execute_python or execute_bash."
- `IntentFile` → "User wants file operations. Use filesystem tools."
- `IntentKnowledge` → default cascade behavior

**Cost:** 1 extra LLM call (~1-2s), but saves 2-3 wasted iterations on wrong tools.

### 3. Multi-Tool Execution + Higher Limits

**a) Ollama tool calling can return multiple tool_calls per response.** Handle them:
```go
if len(result.ToolCalls) > 0 {
    for _, call := range result.ToolCalls {
        event := uc.executeTool(ctx, call)
        scratchpad = append(scratchpad, event)
    }
    continue // next iteration with all results
}
```

**b) Increase `AGENT_MAX_ITERATIONS` default from 6 to 10.**

**c) Add `AGENT_PLANNER_MODEL` recommendation to `.env.example`:**
```
OLLAMA_PLANNER_MODEL=qwen3:14b
```

## Files to Modify

| File | Changes |
|------|---------|
| `internal/core/ports/outbound.go` | Add `ChatWithTools` to `AnswerGenerator` interface |
| `internal/core/ports/inbound.go` | Add `ChatWithTools` to `DocumentQueryService` |
| `internal/core/domain/agent_memory.go` | Add `ChatMessage`, `ToolSchema`, `ChatToolsResult` types |
| `internal/infrastructure/llm/ollama/client.go` | Implement `ChatWithTools` using `/api/chat` |
| `internal/infrastructure/llm/openaicompat/generator.go` | Implement `ChatWithTools` using OpenAI-compat chat API |
| `internal/infrastructure/llm/fallback/fallback.go` | Forward `ChatWithTools` |
| `internal/infrastructure/llm/routing/routing.go` | Forward `ChatWithTools` |
| `internal/core/usecase/agent_chat.go` | Rewrite agent loop to use function calling |
| `internal/core/usecase/query.go` | Forward `ChatWithTools` |
| `internal/config/config.go` | Add `AgentIntentRouterEnabled` |
| `.env.example` | Document `OLLAMA_PLANNER_MODEL`, intent router config |

## Migration Strategy

1. Add `ChatWithTools` to interfaces (backward compatible — existing methods stay)
2. Implement in ollama + openaicompat providers
3. Rewrite agent loop to use new API
4. Add intent router
5. Remove old `parseAgentStep`, `buildPlannerPrompt`, `buildPlannerRepairPrompt`

## Verification

1. `go test ./...` — all existing tests pass
2. `go vet ./...` — clean
3. Test: "Какие темы я изучаю в ML vault?" → uses knowledge_search → detailed answer
4. Test: "Вычисли факториал 20 через Python" → intent=code → execute_python → correct result
5. Test: "Прочитай файл Progress.md из ML vault" → intent=file → read_file → content
6. Test: "Сколько будет 2^64?" → intent=code → execute_python → 18446744073709551616
7. Test: "Что такое transformer?" → intent=knowledge → search → answer (or LLM if not in vault)

## Planner Model Recommendation

For RX 7900 GRE (16GB VRAM):
- Gen model: qwen3.5:9b (~6GB)
- Planner model: qwen3:14b (~9GB) — best tool calling support
- Total: ~15GB — fits in 16GB VRAM
