package httpadapter

import (
	"context"
	"strings"
	"time"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func (rt *Router) ListModels(_ context.Context, _ apigen.ListModelsRequestObject) (apigen.ListModelsResponseObject, error) {
	created := time.Now().Unix()
	modelID := rt.openAICompatModelID
	if modelID == "" {
		modelID = "paa-rag-v1"
	}

	return apigen.ListModels200JSONResponse{
		Object: "list",
		Data: []apigen.ModelObject{
			{
				Id:      modelID,
				Object:  "model",
				OwnedBy: "personal-ai-assistant",
				Created: &created,
			},
		},
	}, nil
}

func (rt *Router) ChatCompletions(ctx context.Context, request apigen.ChatCompletionsRequestObject) (apigen.ChatCompletionsResponseObject, error) {
	if request.Body == nil {
		return apigen.ChatCompletions400JSONResponse{Error: "request body is required"}, nil
	}
	if len(request.Body.Messages) == 0 {
		return apigen.ChatCompletions400JSONResponse{Error: "messages are required"}, nil
	}

	modelID := strings.TrimSpace(request.Body.Model)
	if modelID == "" {
		modelID = rt.openAICompatModelID
	}
	if modelID == "" {
		modelID = "paa-rag-v1"
	}

	completionID := newCompletionID()
	created := time.Now().Unix()
	stream := request.Body.Stream != nil && *request.Body.Stream

	lastMessage := request.Body.Messages[len(request.Body.Messages)-1]
	if lastMessage.Role == apigen.Tool {
		toolContent := extractMessageText(lastMessage)
		if toolContent == "" {
			return apigen.ChatCompletions400JSONResponse{Error: "tool message content is required"}, nil
		}
		lastUser, _ := latestUserMessageContent(request.Body.Messages)
		answer, err := rt.postProcessToolOutput(ctx, request.Body.Messages, toolContent)
		if err != nil {
			return apigen.ChatCompletions500JSONResponse{Error: err.Error()}, nil
		}
		debugMode := "tool_postprocess"
		response := buildTextChatCompletionResponse(completionID, created, modelID, lastUser, answer, &apigen.DebugInfo{Mode: &debugMode})
		if stream {
			return chatCompletionsSSEResponse{Chunks: buildTextStreamChunks(completionID, created, modelID, answer, rt.openAICompatStreamChunkChars)}, nil
		}
		return apigen.ChatCompletions200JSONResponse(response), nil
	}

	lastUser, ok := latestUserMessageContent(request.Body.Messages)
	if !ok {
		return apigen.ChatCompletions400JSONResponse{Error: "at least one user message with text content is required"}, nil
	}

	if toolCall, ok := rt.buildToolCallIfTriggered(lastUser, request.Body.Tools); ok {
		response := buildToolCallChatCompletionResponse(completionID, created, modelID, lastUser, toolCall)
		if stream {
			return chatCompletionsSSEResponse{Chunks: buildToolCallStreamChunks(completionID, created, modelID, toolCall)}, nil
		}
		return apigen.ChatCompletions200JSONResponse(response), nil
	}

	ragQuestion := buildRAGQuestion(request.Body.Messages, lastUser, rt.openAICompatContextMessages)
	answer, err := rt.querySvc.Answer(ctx, ragQuestion, rt.ragTopK, domain.SearchFilter{})
	if err != nil {
		return apigen.ChatCompletions500JSONResponse{Error: err.Error()}, nil
	}

	debugMode := "rag"
	debug := &apigen.DebugInfo{Mode: &debugMode}
	if sources := toAPIDebugSources(answer.Sources); len(sources) > 0 {
		debug.Sources = &sources
	}

	response := buildTextChatCompletionResponse(completionID, created, modelID, ragQuestion, answer.Text, debug)
	if stream {
		return chatCompletionsSSEResponse{Chunks: buildTextStreamChunks(completionID, created, modelID, answer.Text, rt.openAICompatStreamChunkChars)}, nil
	}
	return apigen.ChatCompletions200JSONResponse(response), nil
}
