# Agent Performance Upgrade — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade agent from free-form JSON planner to native function calling, add intent router, support multi-tool execution — improving task completion from 6/10 to 9/10.

**Architecture:** Replace `GenerateJSONFromPrompt` → `parseAgentStep` pipeline with Ollama `/api/chat` native tool calling. Add keyword+LLM intent classifier before planner. Handle multiple tool_calls per iteration.

**Tech Stack:** Go, Ollama `/api/chat` API with `tools` param, mcp-go

**Spec:** `docs/superpowers/specs/2026-03-21-agent-performance-upgrade-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/core/domain/agent_memory.go` | Modify | Add `ChatMessage`, `ToolSchema`, `ChatToolsResult` types |
| `internal/core/ports/outbound.go` | Modify | Add `ChatWithTools` to `AnswerGenerator` |
| `internal/core/ports/inbound.go` | Modify | Add `ChatWithTools` to `DocumentQueryService` |
| `internal/infrastructure/llm/ollama/chat_tools.go` | Create | Ollama `/api/chat` with tools implementation |
| `internal/infrastructure/llm/ollama/chat_tools_test.go` | Create | Tests for Ollama chat with tools |
| `internal/infrastructure/llm/openaicompat/chat_tools.go` | Create | OpenAI-compat chat with tools |
| `internal/infrastructure/llm/fallback/fallback.go` | Modify | Forward `ChatWithTools` |
| `internal/infrastructure/llm/routing/routing.go` | Modify | Forward `ChatWithTools` |
| `internal/core/usecase/query.go` | Modify | Forward `ChatWithTools` |
| `internal/core/usecase/intent.go` | Create | Intent classifier (keyword + LLM) |
| `internal/core/usecase/intent_test.go` | Create | Tests for intent classifier |
| `internal/core/usecase/agent_chat.go` | Modify | Rewrite loop to use function calling |
| `internal/core/usecase/agent_chat_test.go` | Modify | Update tests for new loop |
| `internal/config/config.go` | Modify | Add intent router config |
| `.env.example` | Modify | Document planner model + intent router |

---

### Task 1: Domain types for function calling

**Files:**
- Modify: `internal/core/domain/agent_memory.go`

- [ ] **Step 1: Add chat and tool schema types**

Add to the end of `agent_memory.go`:

```go
// ChatMessage represents a message in the LLM chat format.
type ChatMessage struct {
	Role       string         `json:"role"`    // "system", "user", "assistant", "tool"
	Content    string         `json:"content"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// ToolCall represents a structured tool invocation from the LLM.
type ToolCall struct {
	ID       string         `json:"id,omitempty"`
	Function ToolCallFunc   `json:"function"`
}

type ToolCallFunc struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolSchema defines a tool for the LLM function calling API.
type ToolSchema struct {
	Type     string         `json:"type"` // "function"
	Function FunctionSchema `json:"function"`
}

type FunctionSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ChatToolsResult is the parsed response from ChatWithTools.
type ChatToolsResult struct {
	Content   string     // text response (final answer)
	ToolCalls []ToolCall // tool invocations (empty if final)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/core/domain/...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/core/domain/agent_memory.go
git commit -m "feat: add domain types for native function calling"
```

---

### Task 2: Port interfaces — add ChatWithTools

**Files:**
- Modify: `internal/core/ports/outbound.go`
- Modify: `internal/core/ports/inbound.go`

- [ ] **Step 1: Add ChatWithTools to AnswerGenerator**

In `outbound.go`, add to the `AnswerGenerator` interface:

```go
ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error)
```

- [ ] **Step 2: Add ChatWithTools to DocumentQueryService**

In `inbound.go`, add to the `DocumentQueryService` interface:

```go
ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error)
```

- [ ] **Step 3: Verify build compiles (expect failures in implementations)**

Run: `go build ./internal/core/ports/...`
Expected: success (interfaces only)

- [ ] **Step 4: Commit**

```bash
git add internal/core/ports/outbound.go internal/core/ports/inbound.go
git commit -m "feat: add ChatWithTools to port interfaces"
```

---

### Task 3: Ollama ChatWithTools implementation

**Files:**
- Create: `internal/infrastructure/llm/ollama/chat_tools.go`
- Create: `internal/infrastructure/llm/ollama/chat_tools_test.go`

- [ ] **Step 1: Write test for ChatWithTools**

Create `chat_tools_test.go` with a test that uses `httptest.NewServer` to mock the Ollama `/api/chat` response. Test two scenarios:
1. Model returns a text response (final answer) — `response.message.content` is set, `tool_calls` empty
2. Model returns tool calls — `response.message.tool_calls` is set

```go
func TestChatWithTools_TextResponse(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request has "tools" field
        var req map[string]any
        json.NewDecoder(r.Body).Decode(&req)
        if req["tools"] == nil {
            t.Error("expected tools in request")
        }
        json.NewEncoder(w).Encode(map[string]any{
            "message": map[string]any{
                "role":    "assistant",
                "content": "The answer is 42",
            },
        })
    }))
    defer srv.Close()

    client := NewWithOptions(srv.URL, "test-model", "test-embed", Options{})
    gen := NewGenerator(client)

    result, err := gen.ChatWithTools(context.Background(),
        []domain.ChatMessage{{Role: "user", Content: "What is 6*7?"}},
        []domain.ToolSchema{},
    )
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.Content != "The answer is 42" {
        t.Errorf("expected 'The answer is 42', got %q", result.Content)
    }
    if len(result.ToolCalls) != 0 {
        t.Errorf("expected no tool calls, got %d", len(result.ToolCalls))
    }
}

func TestChatWithTools_ToolCall(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{
            "message": map[string]any{
                "role":    "assistant",
                "content": "",
                "tool_calls": []map[string]any{
                    {
                        "function": map[string]any{
                            "name":      "knowledge_search",
                            "arguments": map[string]any{"question": "ML topics"},
                        },
                    },
                },
            },
        })
    }))
    defer srv.Close()

    client := NewWithOptions(srv.URL, "test-model", "test-embed", Options{})
    gen := NewGenerator(client)

    result, err := gen.ChatWithTools(context.Background(),
        []domain.ChatMessage{{Role: "user", Content: "What ML topics do I study?"}},
        []domain.ToolSchema{{Type: "function", Function: domain.FunctionSchema{Name: "knowledge_search"}}},
    )
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(result.ToolCalls) != 1 {
        t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
    }
    if result.ToolCalls[0].Function.Name != "knowledge_search" {
        t.Errorf("expected knowledge_search, got %q", result.ToolCalls[0].Function.Name)
    }
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `go test ./internal/infrastructure/llm/ollama/ -run TestChatWithTools -v`
Expected: FAIL (method not defined)

- [ ] **Step 3: Implement ChatWithTools**

Create `chat_tools.go`:

```go
package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// ChatWithTools sends a chat request to Ollama /api/chat with tool definitions.
// Returns either a text response (final answer) or structured tool calls.
func (g *Generator) ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	return g.client.chatWithTools(ctx, messages, tools)
}

func (c *Client) chatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	model := c.plannerModel
	if model == "" {
		model = c.genModel
	}

	// Build Ollama /api/chat request body.
	ollamaMessages := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		msg := map[string]any{"role": m.Role, "content": m.Content}
		if len(m.ToolCalls) > 0 {
			msg["tool_calls"] = m.ToolCalls
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		ollamaMessages = append(ollamaMessages, msg)
	}

	ollamaTools := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		ollamaTools = append(ollamaTools, map[string]any{
			"type": t.Type,
			"function": map[string]any{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  t.Function.Parameters,
			},
		})
	}

	reqBody := map[string]any{
		"model":    model,
		"messages": ollamaMessages,
		"stream":   false,
		"think":    false,
	}
	if len(ollamaTools) > 0 {
		reqBody["tools"] = ollamaTools
	}

	// Call Ollama /api/chat.
	var response struct {
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	respBody, err := c.doPost(ctx, "/api/chat", body)
	if err != nil {
		return nil, fmt.Errorf("ollama chat: %w", err)
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("unmarshal chat response: %w", err)
	}

	result := &domain.ChatToolsResult{
		Content: strings.TrimSpace(response.Message.Content),
	}

	for _, tc := range response.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, domain.ToolCall{
			Function: domain.ToolCallFunc{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return result, nil
}
```

Note: `doPost` is an existing helper in `client.go`. If it doesn't exist, extract the HTTP POST logic from the existing `generate()` method into a reusable `doPost(ctx, path, body) ([]byte, error)`.

Check `client.go` for existing HTTP helper methods. The `generate()` method at line 157 does the HTTP call. Extract the POST logic into `doPost` if needed, or inline the HTTP call.

- [ ] **Step 4: Run tests — verify they pass**

Run: `go test ./internal/infrastructure/llm/ollama/ -run TestChatWithTools -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/llm/ollama/chat_tools.go internal/infrastructure/llm/ollama/chat_tools_test.go
git commit -m "feat: implement Ollama ChatWithTools via /api/chat"
```

---

### Task 4: Forward ChatWithTools through providers

**Files:**
- Modify: `internal/infrastructure/llm/openaicompat/generator.go`
- Modify: `internal/infrastructure/llm/fallback/fallback.go`
- Modify: `internal/infrastructure/llm/routing/routing.go`
- Modify: `internal/core/usecase/query.go`

All providers need `ChatWithTools` to satisfy the `AnswerGenerator` interface. OpenAI-compat already uses `/v1/chat/completions` which supports `tools` natively.

- [ ] **Step 1: Add ChatWithTools to openaicompat**

Create `internal/infrastructure/llm/openaicompat/chat_tools.go`. Use the existing `/v1/chat/completions` endpoint with `tools` parameter. The OpenAI format is identical to what we need:
- Request: `{"model":"...","messages":[...],"tools":[...]}`
- Response: `choices[0].message.tool_calls` or `choices[0].message.content`

- [ ] **Step 2: Add ChatWithTools to fallback generator**

In `fallback.go`, add:
```go
func (g *Generator) ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	result, err := g.primary.ChatWithTools(ctx, messages, tools)
	if err != nil && g.fallback != nil {
		g.logger.Warn("primary_chat_tools_failed", "err", err)
		return g.fallback.ChatWithTools(ctx, messages, tools)
	}
	return result, err
}
```

- [ ] **Step 3: Add ChatWithTools to routing generator**

In `routing.go`, add:
```go
func (g *Generator) ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	provider := g.providerFromContext(ctx)
	gen := g.generators[provider]
	if gen == nil {
		gen = g.generators[g.defaultProvider]
	}
	return gen.ChatWithTools(ctx, messages, tools)
}
```

- [ ] **Step 4: Add ChatWithTools to QueryUseCase**

In `query.go`, add:
```go
func (uc *QueryUseCase) ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	return uc.generator.ChatWithTools(ctx, messages, tools)
}
```

- [ ] **Step 5: Verify full build**

Run: `go build ./...`
Expected: success

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/llm/openaicompat/chat_tools.go internal/infrastructure/llm/fallback/fallback.go internal/infrastructure/llm/routing/routing.go internal/core/usecase/query.go
git commit -m "feat: forward ChatWithTools through all LLM providers"
```

---

### Task 5: Intent classifier

**Files:**
- Create: `internal/core/usecase/intent.go`
- Create: `internal/core/usecase/intent_test.go`
- Modify: `internal/config/config.go`

- [ ] **Step 1: Write intent classifier tests**

Create `intent_test.go`:
```go
func TestClassifyIntent_Keywords(t *testing.T) {
	tests := []struct {
		input    string
		expected Intent
	}{
		{"выполни Python скрипт", IntentCode},
		{"запусти код", IntentCode},
		{"execute bash command", IntentCode},
		{"прочитай файл Progress.md", IntentFile},
		{"покажи содержимое каталога", IntentFile},
		{"создай задачу купить молоко", IntentTask},
		{"найди в интернете рецепт", IntentWeb},
		{"что такое transformer?", IntentGeneral}, // no keyword match → general
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := classifyIntentByKeywords(tt.input)
			if got != tt.expected {
				t.Errorf("classifyIntentByKeywords(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `go test ./internal/core/usecase/ -run TestClassifyIntent -v`
Expected: FAIL

- [ ] **Step 3: Implement intent classifier**

Create `intent.go`:
```go
package usecase

import "strings"

type Intent string

const (
	IntentKnowledge Intent = "knowledge"
	IntentCode      Intent = "code"
	IntentFile      Intent = "file"
	IntentTask      Intent = "task"
	IntentWeb       Intent = "web"
	IntentGeneral   Intent = "general"
)

var intentKeywords = map[Intent][]string{
	IntentCode: {"python", "код", "скрипт", "выполни", "execute", "bash",
		"запусти", "вычисли", "calculate", "compute", "execute_python", "execute_bash"},
	IntentFile: {"файл", "прочитай", "каталог", "директори", "file", "read_file",
		"list_directory", "папк", "содержимое файла", "открой"},
	IntentTask: {"задач", "task", "напомни", "todo", "дело"},
	IntentWeb:  {"интернет", "web", "найди в сети", "загугли", "поищи онлайн", "search online"},
}

func classifyIntentByKeywords(message string) Intent {
	lower := strings.ToLower(message)
	for intent, keywords := range intentKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return intent
			}
		}
	}
	return IntentGeneral
}

// systemPromptForIntent returns an additional instruction for the LLM
// based on the detected intent.
func systemPromptForIntent(intent Intent) string {
	switch intent {
	case IntentCode:
		return "The user wants to execute code. Prefer execute_python or execute_bash tools. Call them directly."
	case IntentFile:
		return "The user wants to work with files. Prefer filesystem tools (read_file, read_text_file, list_directory, search_files). Call them directly."
	case IntentTask:
		return "The user wants to manage tasks. Use task_create, task_list, task_update, task_complete, or task_delete."
	case IntentWeb:
		return "The user needs internet search. Use web_search tool."
	case IntentKnowledge:
		return "Search the knowledge base first using knowledge_search."
	default:
		return "Answer from your knowledge. If unsure, search the knowledge base first using knowledge_search."
	}
}
```

- [ ] **Step 4: Add config**

In `config.go`, add field `AgentIntentRouterEnabled bool` and in `Load()`:
```go
AgentIntentRouterEnabled: mustEnvBool("AGENT_INTENT_ROUTER_ENABLED", true),
```

- [ ] **Step 5: Run tests — verify they pass**

Run: `go test ./internal/core/usecase/ -run TestClassifyIntent -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/core/usecase/intent.go internal/core/usecase/intent_test.go internal/config/config.go
git commit -m "feat: add keyword-based intent classifier for agent routing"
```

---

### Task 6: Rewrite agent loop to use native function calling

**Files:**
- Modify: `internal/core/usecase/agent_chat.go`
- Modify: `internal/core/usecase/agent_chat_test.go`

This is the core task. Replace the `GenerateJSONFromPrompt` → `parseAgentStep` pipeline with `ChatWithTools`.

- [ ] **Step 1: Add tool schema builder**

Add function `toolSchemasFromRegistry` that converts `MCPToolRegistry.ListTools()` into `[]domain.ToolSchema`:

```go
func toolSchemasFromRegistry(registry ports.MCPToolRegistry, webSearchAvailable bool) []domain.ToolSchema {
	var schemas []domain.ToolSchema
	for _, t := range registry.ListTools() {
		// Skip web_search if not available
		if t.Name == "web_search" && !webSearchAvailable {
			continue
		}
		schemas = append(schemas, domain.ToolSchema{
			Type: "function",
			Function: domain.FunctionSchema{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	return schemas
}
```

- [ ] **Step 2: Build chat messages helper**

Add function `buildChatMessages` that constructs `[]domain.ChatMessage` from system prompt + conversation history + scratchpad:

```go
func buildChatMessages(systemPrompt string, shortMemory []domain.ConversationMessage, memoryHits []domain.MemoryHit, scratchpad []domain.AgentToolEvent, userMessage string) []domain.ChatMessage {
	var messages []domain.ChatMessage

	// System message with intent-specific instructions
	messages = append(messages, domain.ChatMessage{Role: "system", Content: systemPrompt})

	// Short-term memory (recent conversation)
	for _, msg := range shortMemory {
		if content := strings.TrimSpace(msg.Content); content != "" {
			messages = append(messages, domain.ChatMessage{Role: msg.Role, Content: content})
		}
	}

	// Current user message
	messages = append(messages, domain.ChatMessage{Role: "user", Content: userMessage})

	// Tool results from previous iterations (scratchpad)
	for _, event := range scratchpad {
		messages = append(messages, domain.ChatMessage{
			Role:    "tool",
			Content: event.Output,
		})
	}

	return messages
}
```

- [ ] **Step 3: Rewrite the agent loop in `Complete`**

Replace the inner loop body. The new loop:
1. Classifies intent (first iteration only)
2. Builds chat messages from conversation + scratchpad
3. Calls `ChatWithTools(messages, toolSchemas)`
4. If `result.Content` is non-empty → final answer, break
5. If `result.ToolCalls` is non-empty → execute ALL tool calls, add to scratchpad, continue
6. If neither → fallback

Key change: remove `buildPlannerPrompt`, `parseAgentStep`, `buildPlannerRepairPrompt` calls. Replace with `buildChatMessages` + `ChatWithTools`.

The system prompt should be:
```
You are a personal AI assistant. You have access to tools for searching knowledge, executing code, managing files, and more.

RULES:
- Always respond in Russian.
- Use tools when they help answer the question.
- After using a tool, analyze its output and provide a complete answer.
- If you don't need tools, answer directly.

{intent-specific instruction from systemPromptForIntent()}

{memory hints from long-term memory, if any}
```

- [ ] **Step 4: Handle multi-tool calls**

When `ChatWithTools` returns multiple tool calls, execute all of them in sequence:
```go
for _, tc := range chatResult.ToolCalls {
    event, err := uc.executeToolCall(ctx, userID, tc)
    // ... handle error, add to scratchpad/toolEvents
}
```

Rename `executeTool(ctx, userID, step domain.AgentPlanStep)` to `executeToolCall(ctx, userID, tc domain.ToolCall)` — adapt it to work with `domain.ToolCall` instead of `domain.AgentPlanStep`.

- [ ] **Step 5: Update agent_chat_test.go**

Update the mock `fakeAgentQueryService` to implement `ChatWithTools`. The mock should:
- Return a `ChatToolsResult` with `Content` set (simulating a direct answer)
- Or return tool calls for tool-testing scenarios

Replace all `GenerateJSONFromPrompt` mock behaviors with `ChatWithTools` equivalents.

- [ ] **Step 6: Run all tests**

Run: `go test ./internal/core/usecase/ -v`
Expected: PASS

- [ ] **Step 7: Run full build + vet**

Run: `go build ./... && go vet ./...`
Expected: success

- [ ] **Step 8: Commit**

```bash
git add internal/core/usecase/agent_chat.go internal/core/usecase/agent_chat_test.go
git commit -m "feat: rewrite agent loop to use native function calling"
```

---

### Task 7: Clean up legacy code + update config

**Files:**
- Modify: `internal/core/usecase/agent_chat.go`
- Modify: `.env.example`
- Modify: `internal/config/config.go`

- [ ] **Step 1: Remove dead code**

Remove from `agent_chat.go`:
- `buildPlannerPrompt()` function
- `buildPlannerRepairPrompt()` function
- `parseAgentStep()` function
- `knownToolsFromRegistry()` function (replaced by `toolSchemasFromRegistry`)
- Old `AgentPlanStep` usage (keep the domain type for backward compat)

- [ ] **Step 2: Increase default max iterations**

In `config.go`, change `AgentMaxIterations` default from 6 to 10:
```go
AgentMaxIterations: mustEnvInt("AGENT_MAX_ITERATIONS", 10),
```

- [ ] **Step 3: Update .env.example**

Add planner model and intent router docs:
```
# Planner model for agent tool selection (separate from generation model).
# Recommended: qwen3:14b for RX 7900 GRE (16GB VRAM).
# If empty, uses OLLAMA_GEN_MODEL.
OLLAMA_PLANNER_MODEL=qwen3:14b

# Intent router classifies user requests before the planner.
AGENT_INTENT_ROUTER_ENABLED=true
AGENT_MAX_ITERATIONS=10
```

- [ ] **Step 4: Run full test suite**

Run: `go test ./... && go vet ./...`
Expected: all pass

- [ ] **Step 5: Commit**

```bash
git add internal/core/usecase/agent_chat.go internal/config/config.go .env.example
git commit -m "feat: clean up legacy planner, increase iterations, document planner model"
```

---

### Task 8: End-to-end verification

**Files:** none (testing only)

- [ ] **Step 1: Pull planner model**

```bash
ollama pull qwen3:14b
```

- [ ] **Step 2: Build and start stack**

```bash
docker compose -f docker-compose.yml -f docker-compose.host-gpu.yml build api
docker compose -f docker-compose.yml -f docker-compose.host-gpu.yml up -d
```

- [ ] **Step 3: Test knowledge search**

```bash
curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"paa-rag-v1","stream":false,"messages":[{"role":"user","content":"Какие темы я изучаю в ML vault?"}],"metadata":{"user_id":"test","conversation_id":"e2e-1"}}'
```

Expected: Agent uses `knowledge_search` → returns detailed ML topics.

- [ ] **Step 4: Test code execution**

```bash
curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"paa-rag-v1","stream":false,"messages":[{"role":"user","content":"Вычисли сумму простых чисел до 1000 через Python"}],"metadata":{"user_id":"test","conversation_id":"e2e-2"}}'
```

Expected: Agent detects intent=code → uses `execute_python` → returns 76127.

- [ ] **Step 5: Test file operations**

```bash
curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"paa-rag-v1","stream":false,"messages":[{"role":"user","content":"Покажи список файлов в ML vault"}],"metadata":{"user_id":"test","conversation_id":"e2e-3"}}'
```

Expected: Agent detects intent=file → uses `list_directory` → returns file list.

- [ ] **Step 6: Test multi-tool complex query**

```bash
curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"paa-rag-v1","stream":false,"messages":[{"role":"user","content":"Прочитай файл Progress.md из ML vault и посчитай процент выполнения модулей через Python"}],"metadata":{"user_id":"test","conversation_id":"e2e-4"}}'
```

Expected: Agent uses filesystem tools to find & read file, then execute_python to calculate stats.
