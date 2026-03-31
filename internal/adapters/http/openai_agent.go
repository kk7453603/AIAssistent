package httpadapter

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func (rt *Router) tryAgentCompletion(
	ctx context.Context,
	request apigen.ChatCompletionsRequestObject,
	completionID string,
	created int64,
	modelID string,
	lastUser string,
	stream bool,
) (apigen.ChatCompletionsResponseObject, bool, error) {
	userID, conversationID, sessionEnd, ok := rt.agentMetadata(request.Body)
	if !ok {
		return nil, false, nil
	}

	inputMessages := toAgentInputMessages(request.Body.Messages)

	var toolStatusEvents []toolStatusEntry
	var toolStatusMu sync.Mutex
	var onToolStatus domain.ToolStatusCallback
	if stream {
		onToolStatus = func(tool, status string) {
			toolStatusMu.Lock()
			toolStatusEvents = append(toolStatusEvents, toolStatusEntry{Tool: tool, Status: status})
			toolStatusMu.Unlock()
		}
	}

	var orchStepEvents []orchStepEntry
	var orchStepMu sync.Mutex

	agentReq := domain.AgentChatRequest{
		UserID:         userID,
		ConversationID: conversationID,
		SessionEnd:     sessionEnd,
		Messages:       inputMessages,
	}
	if stream {
		agentReq.OnOrchStep = func(status domain.OrchestrationStatus) {
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
		}
	}

	// For streaming: set up real-time thinking delta via channel.
	// The response visitor drains this channel and writes SSE events
	// WHILE the agent is still running.
	var thinkingCh chan string
	if stream {
		thinkingCh = make(chan string, 512)
		agentReq.OnThinkingDelta = func(text string) {
			select {
			case thinkingCh <- text:
			default:
			}
		}
	}

	// Launch agent in background goroutine
	resultCh := make(chan agentResult, 1)

	go func() {
		defer func() {
			if thinkingCh != nil {
				close(thinkingCh)
			}
		}()
		result, err := rt.agentSvc.Complete(ctx, agentReq, onToolStatus)
		resultCh <- agentResult{result, err}
	}()

	if stream {
		// Return a streaming response that writes thinking tokens in real-time
		return &realtimeAgentSSEResponse{
			rt:             rt,
			thinkingCh:     thinkingCh,
			resultCh:       resultCh,
			toolStatusMu:   &toolStatusMu,
			toolStatusPtr:  &toolStatusEvents,
			orchStepMu:     &orchStepMu,
			orchStepPtr:    &orchStepEvents,
			completionID:   completionID,
			created:        created,
			modelID:        modelID,
			lastUser:       lastUser,
			userID:         userID,
		}, true, nil
	}

	// Non-streaming: wait for result
	ar := <-resultCh
	if ar.err != nil {
		return nil, true, ar.err
	}
	result := ar.result

	rt.recordAgentMetrics(result, userID, modelID)

	debug := rt.buildAgentDebug(result)
	response := buildTextChatCompletionResponse(completionID, created, modelID, lastUser, result.Answer, debug)
	if response.Usage != nil {
		rt.httpMetrics.RecordTokenUsage("api", "chat_completions_agent", modelID, response.Usage.PromptTokens, response.Usage.CompletionTokens)
	}
	return apigen.ChatCompletions200JSONResponse(response), true, nil
}

func (rt *Router) recordAgentMetrics(result *domain.AgentRunResult, userID, modelID string) {
	status := "success"
	if result.FallbackReason != "" {
		status = "fallback"
	}
	rt.httpMetrics.RecordAgentRun("api", "chat_completions", status, result.Iterations)
	rt.httpMetrics.RecordMemoryHits("api", "chat_completions", result.MemoryHits)
	rt.httpMetrics.RecordAgentFallbackReason("api", "chat_completions", result.FallbackReason)
	if result.SummaryCreated {
		rt.httpMetrics.RecordMemorySummary("api")
	}
	for _, event := range result.ToolEvents {
		rt.httpMetrics.RecordAgentToolCall("api", event.Tool, event.Status)
	}

	slog.Info("agent_chat",
		"endpoint", "chat_completions",
		"user_id", userID,
		"conversation_id", result.ConversationID,
		"iterations", result.Iterations,
		"memory_hits", result.MemoryHits,
		"tools", result.ToolsInvoked,
		"fallback_reason", result.FallbackReason,
	)
}

func (rt *Router) buildAgentDebug(result *domain.AgentRunResult) *apigen.DebugInfo {
	mode := "agent"
	agentEnabled := true
	conversationIDValue := result.ConversationID
	iterations := result.Iterations
	memoryHits := result.MemoryHits
	debug := &apigen.DebugInfo{
		Mode:            &mode,
		AgentEnabled:    &agentEnabled,
		ConversationId:  &conversationIDValue,
		AgentIterations: &iterations,
		MemoryHits:      &memoryHits,
	}
	if len(result.ToolsInvoked) > 0 {
		tools := append([]string(nil), result.ToolsInvoked...)
		debug.ToolsInvoked = &tools
	}
	if result.FallbackReason != "" {
		reason := result.FallbackReason
		debug.FallbackReason = &reason
	}
	return debug
}

func (rt *Router) agentMetadata(body *apigen.ChatCompletionsJSONRequestBody) (userID, conversationID string, sessionEnd bool, ok bool) {
	if !rt.agentModeEnabled || rt.agentSvc == nil || body == nil || body.Metadata == nil {
		return "", "", false, false
	}
	userID = strings.TrimSpace(valueOrEmpty(body.Metadata.UserId))
	if userID == "" {
		return "", "", false, false
	}
	conversationID = strings.TrimSpace(valueOrEmpty(body.Metadata.ConversationId))
	if body.Metadata.SessionEnd != nil {
		sessionEnd = *body.Metadata.SessionEnd
	}
	return userID, conversationID, sessionEnd, true
}

func toAgentInputMessages(messages []apigen.ChatMessage) []domain.AgentInputMessage {
	out := make([]domain.AgentInputMessage, 0, len(messages))
	for _, msg := range messages {
		content := extractMessageText(msg)
		if strings.TrimSpace(content) == "" {
			continue
		}
		out = append(out, domain.AgentInputMessage{
			Role:    string(msg.Role),
			Content: content,
		})
	}
	return out
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
