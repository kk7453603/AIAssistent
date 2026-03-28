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

	result, err := rt.agentSvc.Complete(ctx, agentReq, onToolStatus)
	if err != nil {
		return nil, true, err
	}

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

	response := buildTextChatCompletionResponse(completionID, created, modelID, lastUser, result.Answer, debug)
	if response.Usage != nil {
		rt.httpMetrics.RecordTokenUsage(
			"api",
			"chat_completions_agent",
			modelID,
			response.Usage.PromptTokens,
			response.Usage.CompletionTokens,
		)
	}
	slog.Info("agent_chat",
		"request_id", requestIDFromContext(ctx),
		"endpoint", "chat_completions",
		"user_id", userID,
		"conversation_id", result.ConversationID,
		"agent_enabled", true,
		"iterations", result.Iterations,
		"memory_hits", result.MemoryHits,
		"tools", result.ToolsInvoked,
		"fallback_reason", result.FallbackReason,
	)
	if stream {
		return agentSSEResponse{
			ToolEvents: toolStatusEvents,
			OrchSteps:  orchStepEvents,
			Chunks:     buildTextStreamChunks(completionID, created, modelID, result.Answer, rt.openAICompatStreamChunkChars),
		}, true, nil
	}
	return apigen.ChatCompletions200JSONResponse(response), true, nil
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
