# Orchestration Stepper in Chat — Design Spec

## Goal

Show multi-agent orchestration progress inline within chat messages as a vertical stepper, with real-time SSE updates for each agent step.

## Architecture

Minimal changes: extend backend SSE events with orchestration details, add one React component to render the stepper within the chat message flow.

## Backend Changes

### SSE Event Extension

File: `internal/adapters/http/router.go`

Currently, orchestration sends generic `tool_status` SSE events:
```json
{"tool":"orchestrator:researcher","status":"started"}
```

**Change:** Add a new SSE event type `orchestration_step` with full details:
```json
{
  "type": "orchestration_step",
  "orchestration_id": "uuid",
  "step_index": 0,
  "agent_name": "researcher",
  "task": "Find information about quantum computing",
  "status": "started",
  "result": "",
  "duration_ms": 0
}
```

Events are sent at two points per step:
1. **Step started** — `status: "started"`, empty result
2. **Step completed/failed** — `status: "completed"|"failed"`, result filled, duration_ms set

Implementation: modify the `onToolStatus` callback construction in the SSE chat handler to also emit `orchestration_step` events when it detects `orchestrator:*` tool names. The orchestrator usecase already sends these — we just need to enrich the SSE payload.

### Orchestrator Callback Extension

File: `internal/core/usecase/orchestrator.go`

Change `onToolStatus` calls to pass additional context. Options:
- **Option chosen:** Add an `onOrchestrationStep` callback to the orchestrator that carries full `OrchestrationStatus` data. The HTTP handler converts this to SSE events.

Add to `OrchestratorUseCase.Execute` signature or pass via context:
```go
type OrchStepCallback func(status domain.OrchestrationStatus)
```

Called at step start and step completion with full details.

## Frontend Changes

### New Types (types.ts)

```typescript
export interface OrchestrationStepEvent {
  type: "orchestration_step";
  orchestration_id: string;
  step_index: number;
  agent_name: string;
  task: string;
  status: "started" | "completed" | "failed";
  result: string;
  duration_ms: number;
}
```

### OrchestrationStepper Component

File: `ui/src/components/chat/OrchestrationStepper.tsx`

**Props:**
```typescript
interface OrchestrationStep {
  stepIndex: number;
  agentName: string;
  task: string;
  status: "started" | "completed" | "failed";
  result: string;
  durationMs: number;
}

interface Props {
  steps: OrchestrationStep[];
}
```

**Visual design:**
- Vertical timeline line on the left
- Circle indicators on the line:
  - Blue pulsing dot = running (started)
  - Green checkmark = completed
  - Red cross = failed
- Agent name badge with color by agent type:
  - researcher = blue
  - coder = green
  - writer = violet
  - critic = orange
- Task description text next to badge
- Duration (e.g., "2.3s") for completed steps
- Collapsible result block (markdown rendered):
  - Intermediate steps: collapsed by default
  - Last completed step: expanded by default
  - Click toggles expand/collapse

**Rendering:**
```
│ ● researcher — Find information about...          1.2s
│   └─ [collapsed result, click to expand]
│
│ ● coder — Generate code based on findings          3.4s
│   └─ [collapsed result]
│
│ ◉ writer — Compose final answer                    (running...)
│   └─ [no result yet]
```

### Chat Integration

File: `ui/src/components/chat/MessageBubble.tsx` (or MessageList.tsx)

During SSE streaming, `orchestration_step` events accumulate in an array. When orchestration events are present for the current assistant message, render `<OrchestrationStepper steps={...} />` above the final answer text.

The stepper appears as a collapsible section (similar to existing `<think>` blocks) with header "Multi-Agent Orchestration" and expand/collapse toggle.

### State Management

Orchestration step events are ephemeral — stored in the chat streaming state, not in the Zustand store. The `chatStore` or streaming handler accumulates steps into an array as SSE events arrive, and passes them to the message component.

After streaming completes, the steps array is stored alongside the message content so it persists during the session (but not across sessions — that would require the history API endpoint added later).

## Agent Colors

```typescript
const AGENT_COLORS: Record<string, string> = {
  researcher: "#3b82f6",  // blue
  coder:      "#10b981",  // emerald
  writer:     "#8b5cf6",  // violet
  critic:     "#f97316",  // orange
};
```

Fallback: gray (#6b7280) for unknown agents.

## Scope Boundaries

**In scope:**
- Backend: OrchStepCallback, SSE orchestration_step events
- Frontend: OrchestrationStepper component, chat integration
- Collapsible results with markdown rendering

**Out of scope (future):**
- Orchestration history page (Group B)
- GET /v1/orchestrations API endpoint
- Persistent orchestration history in UI
