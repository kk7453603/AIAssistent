package httpadapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// agentResult carries the outcome of an agent execution.
type agentResult struct {
	result *domain.AgentRunResult
	err    error
}

// realtimeAgentSSEResponse writes SSE events in real-time as the agent executes.
// Thinking tokens are streamed as they arrive from Ollama, before the agent finishes.
type realtimeAgentSSEResponse struct {
	rt *Router

	thinkingCh chan string
	resultCh   chan agentResult

	toolStatusMu  *sync.Mutex
	toolStatusPtr *[]toolStatusEntry
	orchStepMu    *sync.Mutex
	orchStepPtr   *[]orchStepEntry

	completionID string
	created      int64
	modelID      string
	lastUser     string
	userID       string
}

func (r *realtimeAgentSSEResponse) VisitChatCompletionsResponse(w http.ResponseWriter) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming is not supported by response writer")
	}

	// Set SSE headers immediately — before agent finishes
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Phase 1: Stream thinking tokens in real-time as they arrive
	for token := range r.thinkingCh {
		chunk := map[string]any{
			"id":     "thinking",
			"object": "chat.completion.chunk",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"thinking_delta": token},
			}},
		}
		data, _ := json.Marshal(chunk)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return err
		}
		flusher.Flush()
	}

	// Phase 2: thinkingCh closed means agent goroutine finished. Get result.
	ar := <-r.resultCh
	if ar.err != nil {
		errChunk := map[string]any{
			"id":     "error",
			"object": "chat.completion.chunk",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"content": "Error: " + ar.err.Error()},
			}},
		}
		data, _ := json.Marshal(errChunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		flusher.Flush()
		return nil
	}

	result := ar.result
	r.rt.recordAgentMetrics(result, r.userID, r.modelID)

	// Phase 3: Emit tool-status events
	r.toolStatusMu.Lock()
	toolEvents := append([]toolStatusEntry(nil), *r.toolStatusPtr...)
	r.toolStatusMu.Unlock()

	for _, te := range toolEvents {
		toolStatusJSON := fmt.Sprintf(`{"tool":"%s","status":"%s"}`, te.Tool, te.Status)
		chunk := map[string]any{
			"id":     "tool-status",
			"object": "chat.completion.chunk",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"tool_status": toolStatusJSON},
			}},
		}
		data, _ := json.Marshal(chunk)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return err
		}
		flusher.Flush()
	}

	// Phase 4: Emit orchestration step events
	r.orchStepMu.Lock()
	orchSteps := append([]orchStepEntry(nil), *r.orchStepPtr...)
	r.orchStepMu.Unlock()

	for _, os := range orchSteps {
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

	// Phase 5: Emit content chunks
	contentChunks := buildTextStreamChunks(r.completionID, r.created, r.modelID, result.Answer, r.rt.openAICompatStreamChunkChars)
	for _, chunk := range contentChunks {
		payload, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
		flusher.Flush()
	}

	if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
