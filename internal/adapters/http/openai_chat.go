package httpadapter

import (
	"context"
	"log/slog"
	"net/http"
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
			if mapErrorToHTTPStatus(err) == http.StatusServiceUnavailable {
				return apigen.ChatCompletions503JSONResponse{Error: err.Error()}, nil
			}
			return apigen.ChatCompletions500JSONResponse{Error: err.Error()}, nil
		}
		debugMode := "tool_postprocess"
		response := buildTextChatCompletionResponse(completionID, created, modelID, lastUser, answer, &apigen.DebugInfo{Mode: &debugMode})
		if response.Usage != nil {
			rt.httpMetrics.RecordTokenUsage(
				"api",
				"chat_tool_postprocess",
				modelID,
				response.Usage.PromptTokens,
				response.Usage.CompletionTokens,
			)
		}
		if stream {
			return chatCompletionsSSEResponse{Chunks: buildTextStreamChunks(completionID, created, modelID, answer, rt.openAICompatStreamChunkChars)}, nil
		}
		return apigen.ChatCompletions200JSONResponse(response), nil
	}

	lastUser, ok := latestUserMessageContent(request.Body.Messages)
	if !ok {
		return apigen.ChatCompletions400JSONResponse{Error: "at least one user message with text content is required"}, nil
	}
	if response, handled, err := rt.tryAgentCompletion(ctx, request, completionID, created, modelID, lastUser, stream); handled {
		if err != nil {
			if mapErrorToHTTPStatus(err) == http.StatusServiceUnavailable {
				return apigen.ChatCompletions503JSONResponse{Error: err.Error()}, nil
			}
			return apigen.ChatCompletions500JSONResponse{Error: err.Error()}, nil
		}
		return response, nil
	}

	if toolCall, ok := rt.buildToolCallIfTriggered(lastUser, request.Body.Tools); ok {
		response := buildToolCallChatCompletionResponse(completionID, created, modelID, lastUser, toolCall)
		if response.Usage != nil {
			rt.httpMetrics.RecordTokenUsage(
				"api",
				"chat_tool_call",
				modelID,
				response.Usage.PromptTokens,
				response.Usage.CompletionTokens,
			)
		}
		if stream {
			return chatCompletionsSSEResponse{Chunks: buildToolCallStreamChunks(completionID, created, modelID, toolCall)}, nil
		}
		return apigen.ChatCompletions200JSONResponse(response), nil
	}

	ragQuestion := buildRAGQuestion(request.Body.Messages, lastUser, rt.openAICompatContextMessages)
	start := time.Now()
	answer, err := rt.querySvc.Answer(ctx, ragQuestion, rt.ragTopK, domain.SearchFilter{})
	if err != nil {
		if mapErrorToHTTPStatus(err) == http.StatusServiceUnavailable {
			return apigen.ChatCompletions503JSONResponse{Error: err.Error()}, nil
		}
		return apigen.ChatCompletions500JSONResponse{Error: err.Error()}, nil
	}

	debugMode := "rag"
	debug := &apigen.DebugInfo{Mode: &debugMode}
	if sources := toAPIDebugSources(answer.Sources); len(sources) > 0 {
		debug.Sources = &sources
	}

	response := buildTextChatCompletionResponse(completionID, created, modelID, ragQuestion, answer.Text, debug)
	rt.httpMetrics.RecordRAGObservation("api", "chat_completions", len(answer.Sources), time.Since(start))
	mode := string(answer.Retrieval.Mode)
	if mode == "" {
		mode = string(domain.RetrievalModeSemantic)
	}
	rt.httpMetrics.RecordRAGModeRequest("api", "chat_completions", mode)
	if response.Usage != nil {
		rt.httpMetrics.RecordTokenUsage(
			"api",
			"chat_completions",
			modelID,
			response.Usage.PromptTokens,
			response.Usage.CompletionTokens,
		)
	}
	slog.Info("rag_retrieval",
		"request_id", requestIDFromContext(ctx),
		"endpoint", "chat_completions",
		"retrieval_mode", mode,
		"semantic_candidates", answer.Retrieval.SemanticCandidates,
		"lexical_candidates", answer.Retrieval.LexicalCandidates,
		"rerank_applied", answer.Retrieval.RerankApplied,
		"retrieved_chunks", len(answer.Sources),
		"duration_ms", float64(time.Since(start).Microseconds())/1000.0,
	)
	if stream {
		return chatCompletionsSSEResponse{Chunks: buildTextStreamChunks(completionID, created, modelID, answer.Text, rt.openAICompatStreamChunkChars)}, nil
	}
	return apigen.ChatCompletions200JSONResponse(response), nil
}
