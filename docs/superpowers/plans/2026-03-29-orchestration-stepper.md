# Orchestration Stepper in Chat — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show multi-agent orchestration steps inline in chat as a vertical stepper with agent badges, tasks, statuses, durations, and collapsible markdown results.

**Architecture:** Extend backend SSE events to include orchestration step details. Add `OrchestrationStepper` React component rendered inside `ThinkBlock`. Parse new SSE event format in `chatStore`.

**Tech Stack:** Go (SSE), React 19, Zustand 5, Tailwind 3, react-markdown

**Spec:** `docs/superpowers/specs/2026-03-29-orchestration-stepper-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/core/domain/orchestration.go` | Modify | Add OrchStepCallback type |
| `internal/core/usecase/orchestrator.go` | Modify | Call OrchStepCallback at step start/end |
| `internal/core/usecase/agent_chat.go` | Modify | Pass OrchStepCallback to orchestrator |
| `internal/adapters/http/openai_sse.go` | Modify | Add orchestration_step SSE event format |
| `internal/adapters/http/openai_agent.go` | Modify | Build OrchStepCallback from SSE writer |
| `ui/src/api/types.ts` | Modify | Add OrchestrationStepEvent type |
| `ui/src/stores/chatStore.ts` | Modify | Parse orchestration_step SSE events |
| `ui/src/components/chat/OrchestrationStepper.tsx` | Create | Vertical stepper component |
| `ui/src/components/chat/MessageBubble.tsx` | Modify | Render OrchestrationStepper in ThinkBlock |

---

### Task 1: Backend — OrchStepCallback type and orchestrator changes

**Files:**
- Modify: `internal/core/domain/orchestration.go`
- Modify: `internal/core/usecase/orchestrator.go`

- [ ] **Step 1: Add OrchStepCallback to domain**

In `internal/core/domain/orchestration.go`, append after the `OrchestrationStatus` struct:

```go
// OrchStepCallback is called when an orchestration step starts or completes.
type OrchStepCallback func(status OrchestrationStatus)
```

- [ ] **Step 2: Update OrchestratorUseCase.Execute to accept and call OrchStepCallback**

In `internal/core/usecase/orchestrator.go`, change the `Execute` method signature to:

```go
func (uc *OrchestratorUseCase) Execute(
	ctx context.Context,
	req domain.AgentChatRequest,
	onToolStatus domain.ToolStatusCallback,
	onOrchStep domain.OrchStepCallback,
) (*domain.AgentRunResult, error) {
```

Inside the step loop, add two callback calls. **Before** the agent execution (after `if onToolStatus != nil` block at line ~90):

```go
		if onOrchStep != nil {
			onOrchStep(domain.OrchestrationStatus{
				OrchestrationID: orchID,
				StepIndex:       stepIndex,
				AgentName:       step.Agent,
				Task:            step.Task,
				Status:          "started",
			})
		}
```

**After** the step completes (after `orchStore.AddStep` block, before the `onToolStatus` completed call at line ~141):

```go
		if onOrchStep != nil {
			onOrchStep(domain.OrchestrationStatus{
				OrchestrationID: orchID,
				StepIndex:       stepIndex,
				AgentName:       step.Agent,
				Task:            step.Task,
				Status:          orchStep.Status,
				Result:          orchStep.Result,
			})
		}
```

- [ ] **Step 3: Verify Go compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && go build ./...
```

This will fail because callers of `Execute` need updating — that's Task 2.

- [ ] **Step 4: Commit backend domain + orchestrator changes**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add internal/core/domain/orchestration.go internal/core/usecase/orchestrator.go && git commit -m "feat: add OrchStepCallback and emit orchestration step events"
```

---

### Task 2: Backend — Wire callback through agent_chat and HTTP layer

**Files:**
- Modify: `internal/core/usecase/agent_chat.go`
- Modify: `internal/adapters/http/openai_agent.go`
- Modify: `internal/adapters/http/openai_sse.go`

- [ ] **Step 1: Find where orchestrator.Execute is called in agent_chat.go**

Search for `uc.orchestrator.Execute` in `internal/core/usecase/agent_chat.go`. Update the call to pass the new `onOrchStep` parameter. The orchestrator is called from `AgentChatUseCase.Complete`. Pass a new callback parameter through the Complete method.

First, update the `AgentChatService` port if needed. Check `internal/core/ports/inbound.go` for the `Complete` signature:

```go
Complete(ctx context.Context, req domain.AgentChatRequest, onToolStatus domain.ToolStatusCallback) (*domain.AgentRunResult, error)
```

We need to extend this to also accept `onOrchStep`. **However**, to minimize port changes, we'll pass `onOrchStep` through the `AgentChatRequest` or use a combined callback approach.

**Simpler approach:** Add `OnOrchStep domain.OrchStepCallback` field to `AgentChatRequest`:

In `internal/core/domain/agent.go` (or wherever `AgentChatRequest` is defined), add:

```go
type AgentChatRequest struct {
    // ... existing fields ...
    OnOrchStep OrchStepCallback `json:"-"` // runtime callback, not serialized
}
```

Then in `agent_chat.go` where orchestrator is called, pass `req.OnOrchStep`:

```go
result, err := uc.orchestrator.Execute(ctx, req, onToolStatus, req.OnOrchStep)
```

- [ ] **Step 2: Add orchestration step events to SSE**

In `internal/adapters/http/openai_sse.go`, add a new entry type:

```go
type orchStepEntry struct {
	Type            string  `json:"type"`
	OrchestrationID string  `json:"orchestration_id"`
	StepIndex       int     `json:"step_index"`
	AgentName       string  `json:"agent_name"`
	Task            string  `json:"task"`
	Status          string  `json:"status"`
	Result          string  `json:"result,omitempty"`
	DurationMS      float64 `json:"duration_ms,omitempty"`
}
```

Update `agentSSEResponse` to include orchestration steps:

```go
type agentSSEResponse struct {
	ToolEvents []toolStatusEntry
	OrchSteps  []orchStepEntry
	Chunks     []apigen.ChatCompletionChunk
}
```

In `agentSSEResponse.VisitChatCompletionsResponse`, after emitting tool-status events and before content chunks, emit orchestration step events:

```go
	// Emit orchestration step events.
	for _, os := range response.OrchSteps {
		data, _ := json.Marshal(os)
		chunk := map[string]any{
			"id":     "orchestration-step",
			"object": "chat.completion.chunk",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"orchestration_step": string(data)},
			}},
		}
		chunkData, _ := json.Marshal(chunk)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", chunkData); err != nil {
			return err
		}
		flusher.Flush()
	}
```

- [ ] **Step 3: Build OrchStepCallback in openai_agent.go**

In `internal/adapters/http/openai_agent.go`, in `tryAgentCompletion`, add alongside the `onToolStatus` callback:

```go
	var orchStepEvents []orchStepEntry
	var orchStepMu sync.Mutex

	if stream {
		// ... existing onToolStatus setup ...

		req := domain.AgentChatRequest{
			// ... existing fields ...
			OnOrchStep: func(status domain.OrchestrationStatus) {
				orchStepMu.Lock()
				orchStepEvents = append(orchStepEvents, orchStepEntry{
					Type:            "orchestration_step",
					OrchestrationID: status.OrchestrationID,
					StepIndex:       status.StepIndex,
					AgentName:       status.AgentName,
					Task:            status.Task,
					Status:          status.Status,
					Result:          status.Result,
				})
				orchStepMu.Unlock()
			},
		}
	}
```

Update the SSE response construction to include orchStepEvents:

```go
	if stream {
		return agentSSEResponse{
			ToolEvents: toolStatusEvents,
			OrchSteps:  orchStepEvents,
			Chunks:     buildTextStreamChunks(completionID, created, modelID, result.Answer, rt.openAICompatStreamChunkChars),
		}, true, nil
	}
```

- [ ] **Step 4: Verify Go compiles and tests pass**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && go build ./... && go test ./internal/adapters/http/ -v -count=1
```

- [ ] **Step 5: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add internal/core/domain/ internal/core/usecase/agent_chat.go internal/adapters/http/openai_sse.go internal/adapters/http/openai_agent.go && git commit -m "feat: wire OrchStepCallback through HTTP SSE layer"
```

---

### Task 3: Frontend — Types and SSE parsing

**Files:**
- Modify: `ui/src/api/types.ts`
- Modify: `ui/src/stores/chatStore.ts`

- [ ] **Step 1: Add orchestration types**

Append to `ui/src/api/types.ts`:

```typescript
// --- Orchestration Stepper ---

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

- [ ] **Step 2: Add orchestration steps to chat store**

In `ui/src/stores/chatStore.ts`:

1. Add import:
```typescript
import type { ChatMessage, ToolStatusDelta, OrchestrationStepEvent } from "../api/types";
```

2. Add to `ChatState` interface:
```typescript
  orchSteps: OrchestrationStepEvent[];
```

3. Add initial state:
```typescript
  orchSteps: [],
```

4. Reset in `loadConversation`:
```typescript
  orchSteps: [],
```

5. Reset in `sendMessage` initial set:
```typescript
  orchSteps: [],
```

6. In the SSE parsing loop, after the `delta?.tool_status` block and before `delta?.content`, add:

```typescript
                if (delta?.orchestration_step) {
                  const step: OrchestrationStepEvent = JSON.parse(delta.orchestration_step);
                  set((s) => {
                    const existing = s.orchSteps.findIndex(
                      (os) =>
                        os.orchestration_id === step.orchestration_id &&
                        os.step_index === step.step_index,
                    );
                    if (existing >= 0) {
                      const updated = [...s.orchSteps];
                      updated[existing] = step;
                      return { orchSteps: updated };
                    }
                    return { orchSteps: [...s.orchSteps, step] };
                  });
                  continue;
                }
```

7. In `clearMessages`:
```typescript
  clearMessages: () => set({ messages: [], toolStatus: [], orchSteps: [] }),
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

- [ ] **Step 4: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/api/types.ts ui/src/stores/chatStore.ts && git commit -m "feat(ui): parse orchestration_step SSE events in chat store"
```

---

### Task 4: Frontend — OrchestrationStepper component

**Files:**
- Create: `ui/src/components/chat/OrchestrationStepper.tsx`

- [ ] **Step 1: Create OrchestrationStepper**

Create `ui/src/components/chat/OrchestrationStepper.tsx`:

```tsx
import { useState } from "react";
import { Check, ChevronDown, ChevronRight, Loader2, X } from "lucide-react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { OrchestrationStepEvent } from "../../api/types";

const AGENT_COLORS: Record<string, string> = {
  researcher: "#3b82f6",
  coder: "#10b981",
  writer: "#8b5cf6",
  critic: "#f97316",
};

function agentColor(name: string): string {
  return AGENT_COLORS[name] ?? "#6b7280";
}

function StatusIcon({ status }: { status: string }) {
  if (status === "started") {
    return <Loader2 className="h-4 w-4 animate-spin text-blue-400" />;
  }
  if (status === "completed") {
    return <Check className="h-4 w-4 text-green-400" />;
  }
  return <X className="h-4 w-4 text-red-400" />;
}

function formatDuration(ms: number): string {
  if (ms <= 0) return "";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

interface StepRowProps {
  step: OrchestrationStepEvent;
  isLast: boolean;
  isLastCompleted: boolean;
}

function StepRow({ step, isLast, isLastCompleted }: StepRowProps) {
  const [expanded, setExpanded] = useState(isLastCompleted);
  const hasResult = step.result && step.status !== "started";

  return (
    <div className="relative flex gap-3">
      {/* Timeline line */}
      <div className="flex flex-col items-center">
        <div
          className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-2"
          style={{ borderColor: agentColor(step.agent_name) }}
        >
          <StatusIcon status={step.status} />
        </div>
        {!isLast && (
          <div className="w-0.5 flex-1 bg-gray-300 dark:bg-gray-700" />
        )}
      </div>

      {/* Content */}
      <div className="flex-1 pb-4">
        <div className="flex items-center gap-2">
          <span
            className="rounded-full px-2 py-0.5 text-xs font-medium text-white"
            style={{ backgroundColor: agentColor(step.agent_name) }}
          >
            {step.agent_name}
          </span>
          <span className="text-sm text-gray-700 dark:text-gray-300 truncate">
            {step.task}
          </span>
          {step.duration_ms > 0 && (
            <span className="ml-auto shrink-0 text-xs text-gray-400">
              {formatDuration(step.duration_ms)}
            </span>
          )}
          {step.status === "started" && (
            <span className="ml-auto shrink-0 text-xs text-blue-400 animate-pulse">
              running...
            </span>
          )}
        </div>

        {/* Collapsible result */}
        {hasResult && (
          <div className="mt-1">
            <button
              onClick={() => setExpanded(!expanded)}
              className="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
            >
              {expanded ? (
                <ChevronDown className="h-3 w-3" />
              ) : (
                <ChevronRight className="h-3 w-3" />
              )}
              {expanded ? "Hide result" : "Show result"}
            </button>
            {expanded && (
              <div className="mt-1 rounded border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50 p-2 prose dark:prose-invert prose-xs max-w-none">
                <Markdown remarkPlugins={[remarkGfm]}>
                  {step.result}
                </Markdown>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

interface Props {
  steps: OrchestrationStepEvent[];
}

export function OrchestrationStepper({ steps }: Props) {
  if (steps.length === 0) return null;

  const sorted = [...steps].sort((a, b) => a.step_index - b.step_index);
  const lastCompletedIndex = sorted
    .filter((s) => s.status === "completed")
    .at(-1)?.step_index;

  return (
    <div className="space-y-0">
      {sorted.map((step, i) => (
        <StepRow
          key={`${step.orchestration_id}-${step.step_index}`}
          step={step}
          isLast={i === sorted.length - 1}
          isLastCompleted={step.step_index === lastCompletedIndex}
        />
      ))}
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/components/chat/OrchestrationStepper.tsx && git commit -m "feat(ui): add OrchestrationStepper component with timeline and collapsible results"
```

---

### Task 5: Frontend — Integrate stepper into MessageBubble

**Files:**
- Modify: `ui/src/components/chat/MessageBubble.tsx`

- [ ] **Step 1: Update MessageBubble props and ThinkBlock**

In `ui/src/components/chat/MessageBubble.tsx`:

1. Add import:
```typescript
import { OrchestrationStepper } from "./OrchestrationStepper";
import type { ChatMessage, ToolStatusDelta, OrchestrationStepEvent } from "../../api/types";
```

2. Update `Props` interface:
```typescript
interface Props {
  message: ChatMessage;
  toolStatus: ToolStatusDelta[];
  orchSteps: OrchestrationStepEvent[];
  isStreaming: boolean;
}
```

3. Update `ThinkBlock` props and rendering — add `orchSteps` parameter:

```typescript
function ThinkBlock({
  thinking,
  toolStatus,
  orchSteps,
  isStreaming,
}: {
  thinking: string;
  toolStatus: ToolStatusDelta[];
  orchSteps: OrchestrationStepEvent[];
  isStreaming: boolean;
}) {
  const [open, setOpen] = useState(false);
  const hasContent = thinking || toolStatus.length > 0 || orchSteps.length > 0;
  if (!hasContent) return null;

  // Auto-expand if orchestration is happening
  const effectiveOpen = open || (orchSteps.length > 0 && isStreaming);
```

Inside the `{open && (` block, add orchestration stepper before tool status:

```tsx
      {effectiveOpen && (
        <div className="border-t border-gray-300 dark:border-gray-700 px-3 py-2">
          {orchSteps.length > 0 && (
            <div className="mb-2">
              <OrchestrationStepper steps={orchSteps} />
            </div>
          )}
          {toolStatus.length > 0 && (
```

Replace `{open && (` with `{effectiveOpen && (` throughout.

4. Update `MessageBubble` to pass `orchSteps`:

```typescript
export function MessageBubble({ message, toolStatus, orchSteps, isStreaming }: Props) {
```

And in the return:
```tsx
        <ThinkBlock
          thinking={thinking}
          toolStatus={toolStatus}
          orchSteps={orchSteps}
          isStreaming={isStreaming}
        />
```

5. Update the label text when orchSteps exist:
```tsx
        <span>
          {orchSteps.length > 0
            ? isStreaming
              ? "Multi-Agent Orchestration..."
              : "Multi-Agent Orchestration"
            : isStreaming
              ? "Thinking..."
              : "Thought process"}
        </span>
```

- [ ] **Step 2: Update MessageList.tsx to pass orchSteps**

Read `ui/src/components/chat/MessageList.tsx` to find where `MessageBubble` is rendered. Add `orchSteps` prop from the chat store. The `orchSteps` should only be passed to the last assistant message (the one currently streaming or the last one).

In `MessageList.tsx`, import:
```typescript
import { useChatStore } from "../../stores/chatStore";
```

Get orchSteps from store:
```typescript
const orchSteps = useChatStore((s) => s.orchSteps);
```

Pass to the last assistant MessageBubble:
```tsx
<MessageBubble
  message={msg}
  toolStatus={isLastAssistant ? toolStatus : []}
  orchSteps={isLastAssistant ? orchSteps : []}
  isStreaming={isLastAssistant && isStreaming}
/>
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

- [ ] **Step 4: Verify Vite build**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx vite build
```

- [ ] **Step 5: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/components/chat/MessageBubble.tsx ui/src/components/chat/MessageList.tsx && git commit -m "feat(ui): integrate OrchestrationStepper into chat message flow"
```

---

### Task 6: Verify full build

- [ ] **Step 1: Go build**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && go build ./...
```

- [ ] **Step 2: Go tests**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && go test ./internal/adapters/http/ -v -count=1
```

- [ ] **Step 3: TypeScript check**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

- [ ] **Step 4: Vite build**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx vite build
```

- [ ] **Step 5: Fix any issues found, commit if needed**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add -A && git commit -m "fix: resolve build issues in orchestration stepper integration"
```

Only commit if there were actual fixes.
