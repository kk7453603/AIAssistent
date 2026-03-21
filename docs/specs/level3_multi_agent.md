# SPEC: Multi-Agent Orchestration

## Goal
Enable specialized agent personas (researcher, coder, writer) that can be invoked independently or orchestrated by a coordinator agent. Each agent has its own system prompt, tool set, and behavior.

## Current State
- Single `AgentChatUseCase` with one agent loop
- Intent classification selects system prompt but same agent handles everything
- Tool registry (MCP) provides tools to one agent

## Architecture

### New Package: `internal/core/usecase/agents/`

```
internal/core/usecase/agents/
  registry.go           — agent persona registry
  coordinator.go        — coordinator that delegates to sub-agents
  persona.go            — persona definition (prompt, tools, config)
  researcher.go         — researcher agent config
  coder.go              — coder agent config
  writer.go             — writer agent config
  coordinator_test.go
```

### Persona Definition

```go
type AgentPersona struct {
    Name          string              `json:"name"`
    Description   string              `json:"description"`
    SystemPrompt  string              `json:"system_prompt"`
    AllowedTools  []string            `json:"allowed_tools"`    // tool name whitelist (empty = all)
    MaxIterations int                 `json:"max_iterations"`
    Provider      string              `json:"provider,omitempty"` // optional provider override
    Temperature   float64             `json:"temperature"`
}
```

Built-in personas:
- **researcher**: knowledge_search, web_search, obsidian_write. Focus on finding and synthesizing information.
- **coder**: execute_python, execute_bash, read_file, list_directory. Focus on code execution and analysis.
- **writer**: obsidian_write, knowledge_search. Focus on content creation, summarization, formatting.

### Coordinator Agent

```go
type Coordinator struct {
    personas   map[string]*AgentPersona
    agentChat  *AgentChatUseCase
    generator  ports.AnswerGenerator
}

func (c *Coordinator) Handle(ctx context.Context, req AgentRequest) (*AgentResponse, error)
```

Workflow:
1. Coordinator receives user message
2. Uses LLM to determine which agent(s) to invoke and in what order
3. Delegates subtasks to specialized agents
4. Aggregates results and returns final response

### Agent Communication
Agents communicate via structured messages in the conversation:
```go
type AgentDelegation struct {
    FromAgent string `json:"from_agent"`
    ToAgent   string `json:"to_agent"`
    Task      string `json:"task"`
    Result    string `json:"result,omitempty"`
}
```

### Config
```
MULTI_AGENT_ENABLED=false
MULTI_AGENT_COORDINATOR_MODEL=       # model for coordinator decisions
MULTI_AGENT_PERSONAS=                # JSON override for persona configs
```

### API Changes
Extend chat request metadata:
```json
{"metadata": {"agent": "researcher"}}  // direct invocation
{"metadata": {"agent": "auto"}}        // coordinator decides
```

## Tests
- Unit: coordinator delegation logic
- Unit: persona tool filtering
- Integration: multi-step task with agent handoffs
