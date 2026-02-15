# ADR-0002: Server-side Agent Loop + Long-term Memory

## Status
Accepted (2026-02-14)

## Context
OpenAI-compatible chat already supported:
- direct RAG answers;
- keyword-triggered `tool_calls` handshake with OpenWebUI;
- post-processing of `role=tool` results.

Missing capabilities for Stage 3:
- iterative multi-step orchestration on backend side;
- task-oriented personal memory;
- long-term semantic retrieval across past conversations.

## Decision
1. Introduce server-side `AgentChatService` as primary orchestration path for `/v1/chat/completions` when feature flag is enabled and `metadata.user_id` is present.
2. Keep legacy `tool_calls`/post-tool path as fallback for backward compatibility.
3. Add two backend tools inside agent loop:
   - `knowledge_search` -> existing `DocumentQueryService.Answer`.
   - `task_tool` -> full CRUD + delete with soft-delete semantics.
4. Add short-term memory from recent conversation messages and long-term summaries:
   - source-of-truth in Postgres (`conversations`, `conversation_messages`, `memory_summaries`);
   - semantic retrieval/indexing in dedicated Qdrant collection (`conversation_memory`).
5. Add guardrails:
   - max iterations;
   - global timeout;
   - invalid plan repair attempt;
   - safe handling of tool errors.

## Consequences
### Positive
- Multi-step workflows now execute in one server request.
- Task memory and conversation memory become first-class backend capabilities.
- Existing OpenAI/OpenWebUI integrations remain compatible through fallback mode.
- New observability signals for agent runs, tool usage, and memory hits.

### Trade-offs
- More moving parts in chat path (planner, tool executor, memory layer).
- Additional Postgres schema and Qdrant collection maintenance.
- Planner reliability depends on strict JSON contract and fallback strategy.

## Defaults and constraints
- `AGENT_MODE_ENABLED=false` by default.
- Agent mode requires `metadata.user_id`.
- No new public REST endpoints for tasks in Stage 3.
- Single-agent orchestration only (no multi-agent graph).
