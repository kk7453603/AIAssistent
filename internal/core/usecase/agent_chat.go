package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
	"github.com/kirillkom/personal-ai-assistant/internal/infrastructure/llm/routing"
	"github.com/kirillkom/personal-ai-assistant/internal/observability/metrics"
)

const (
	agentToolKnowledgeSearch = "knowledge_search"
	agentToolWebSearch       = "web_search"
	agentToolObsidianWrite   = "obsidian_write"
	agentToolTask            = "task_tool"
)

type AgentChatUseCase struct {
	querySvc        ports.DocumentQueryService
	embedder        ports.Embedder
	conversations   ports.ConversationStore
	tasks           ports.TaskStore
	memories        ports.MemoryStore
	memoryVector    ports.MemoryVectorStore
	webSearcher     ports.WebSearcher
	obsidianWriter  ports.ObsidianNoteWriter
	toolRegistry    ports.MCPToolRegistry
	limits          domain.AgentLimits
	toolResultCache *toolCache
	agentMetrics    *metrics.AgentMetrics
	obsidianVaults  []ports.AgentVaultInfo
	modelRouting    *domain.ModelRouting
	graphStore      ports.GraphStore
	orchestrator    *OrchestratorUseCase
}

func NewAgentChatUseCase(
	querySvc ports.DocumentQueryService,
	embedder ports.Embedder,
	conversations ports.ConversationStore,
	tasks ports.TaskStore,
	memories ports.MemoryStore,
	memoryVector ports.MemoryVectorStore,
	webSearcher ports.WebSearcher,
	obsidianWriter ports.ObsidianNoteWriter,
	toolRegistry ports.MCPToolRegistry,
	limits domain.AgentLimits,
	agentMetrics *metrics.AgentMetrics,
) *AgentChatUseCase {
	if limits.MaxIterations <= 0 {
		limits.MaxIterations = 6
	}
	if limits.Timeout <= 0 {
		limits.Timeout = 90 * time.Second
	}
	if limits.PlannerTimeout <= 0 {
		limits.PlannerTimeout = 20 * time.Second
	}
	if limits.ToolTimeout <= 0 {
		limits.ToolTimeout = 30 * time.Second
	}
	if limits.ShortMemoryMessages <= 0 {
		limits.ShortMemoryMessages = 20
	}
	if limits.SummaryEveryTurns <= 0 {
		limits.SummaryEveryTurns = 6
	}
	if limits.MemoryTopK <= 0 {
		limits.MemoryTopK = 4
	}
	if limits.KnowledgeTopK <= 0 {
		limits.KnowledgeTopK = 5
	}

	return &AgentChatUseCase{
		querySvc:        querySvc,
		embedder:        embedder,
		conversations:   conversations,
		tasks:           tasks,
		memories:        memories,
		memoryVector:    memoryVector,
		webSearcher:     webSearcher,
		obsidianWriter:  obsidianWriter,
		toolRegistry:    toolRegistry,
		limits:          limits,
		toolResultCache: newToolCache(),
		agentMetrics:    agentMetrics,
	}
}

// SetObsidianWriter sets the ObsidianNoteWriter after construction (to break circular dependency with Router).
func (uc *AgentChatUseCase) SetObsidianWriter(w ports.ObsidianNoteWriter) {
	uc.obsidianWriter = w
}

// SetObsidianVaults sets the list of available Obsidian vaults for the system prompt.
func (uc *AgentChatUseCase) SetObsidianVaults(vaults []ports.AgentVaultInfo) {
	uc.obsidianVaults = vaults
}

func (uc *AgentChatUseCase) SetModelRouting(r *domain.ModelRouting) {
	uc.modelRouting = r
}

func (uc *AgentChatUseCase) SetGraphStore(g ports.GraphStore) {
	uc.graphStore = g
}

func (uc *AgentChatUseCase) SetOrchestrator(o *OrchestratorUseCase) {
	uc.orchestrator = o
}

func (uc *AgentChatUseCase) Complete(ctx context.Context, req domain.AgentChatRequest, onToolStatus domain.ToolStatusCallback) (*domain.AgentRunResult, error) {
	requestStart := time.Now()
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return nil, domain.WrapError(domain.ErrInvalidInput, "agent complete", fmt.Errorf("user_id is required"))
	}

	lastUserMessage, ok := latestUserInput(req.Messages)
	if !ok {
		return nil, domain.WrapError(domain.ErrInvalidInput, "agent complete", fmt.Errorf("at least one user message is required"))
	}

	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		conversationID = uuid.NewString()
	}

	if _, err := uc.conversations.EnsureConversation(ctx, userID, conversationID); err != nil {
		return nil, fmt.Errorf("ensure conversation: %w", err)
	}

	shortMemory, err := uc.conversations.ListRecentMessages(ctx, userID, conversationID, uc.limits.ShortMemoryMessages)
	if err != nil {
		return nil, fmt.Errorf("load short memory: %w", err)
	}

	memoryHits := make([]domain.MemoryHit, 0)
	queryVector, err := uc.embedder.EmbedQuery(ctx, lastUserMessage)
	if err == nil && len(queryVector) > 0 {
		memoryHits, err = uc.memoryVector.SearchSummaries(ctx, userID, "", queryVector, uc.limits.MemoryTopK)
		if err != nil {
			memoryHits = nil
		}
	}

	turn, err := uc.conversations.NextUserTurn(ctx, userID, conversationID)
	if err != nil {
		return nil, fmt.Errorf("next user turn: %w", err)
	}

	if err := uc.conversations.AppendMessage(ctx, domain.ConversationMessage{
		ID:             uuid.NewString(),
		UserID:         userID,
		ConversationID: conversationID,
		Role:           "user",
		Content:        lastUserMessage,
		UserTurn:       turn,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("append user message: %w", err)
	}

	loopCtx, cancel := context.WithTimeout(ctx, uc.limits.Timeout)
	defer cancel()

	thinkingLines := make([]string, 0, uc.limits.MaxIterations)
	toolEvents := make([]domain.AgentToolEvent, 0, uc.limits.MaxIterations)
	toolsInvoked := make([]string, 0, uc.limits.MaxIterations)
	toolSet := make(map[string]struct{})
	finalAnswer := ""
	fallbackReason := ""
	iterations := 0

	webSearchAvailable := uc.webSearcher != nil

	// Build intent-aware system prompt
	intent := IntentGeneral
	if uc.limits.IntentRouterEnabled {
		intent = classifyIntentByKeywords(lastUserMessage)
		if intent == IntentGeneral {
			classifyCtx, classifyCancel := context.WithTimeout(ctx, 5*time.Second)
			if classified, err := uc.querySvc.GenerateFromPrompt(classifyCtx, classifyIntentLLMPrompt(lastUserMessage)); err == nil {
				intent = parseIntent(classified)
			}
			classifyCancel()
		}
	}
	if uc.agentMetrics != nil {
		uc.agentMetrics.IntentClassifications.WithLabelValues(string(intent)).Inc()
	}
	// Adaptive model routing based on complexity.
	var tier domain.ComplexityTier = domain.TierSimple
	if uc.modelRouting != nil {
		tier = classifyComplexityRules(lastUserMessage, intent)
		if tier == TierUncertain {
			tier = domain.TierComplex // safer default
		}
		model := uc.modelRouting.ModelFor(tier)
		ctx = routing.WithProvider(ctx, model)
		slog.Info("adaptive_routing", "tier", tier, "model", model, "intent", intent)
	}

	// Multi-agent orchestration for complex tasks.
	if uc.orchestrator != nil && shouldOrchestrate(intent, tier, lastUserMessage) {
		slog.Info("orchestrating_multi_agent", "intent", intent, "tier", tier)
		return uc.orchestrator.Execute(ctx, req, onToolStatus, nil)
	}

	systemPrompt := buildSystemPrompt(ctx, intent, memoryHits, uc.toolRegistry, uc.obsidianVaults)
	toolSchemas := toolSchemasFromRegistry(uc.toolRegistry, webSearchAvailable)

	// Build initial messages
	chatMessages := []domain.ChatMessage{
		{Role: "system", Content: systemPrompt},
	}
	// Add short memory as conversation history
	for _, msg := range shortMemory {
		if content := strings.TrimSpace(msg.Content); content != "" {
			chatMessages = append(chatMessages, domain.ChatMessage{Role: msg.Role, Content: content})
		}
	}
	// Add current user message
	chatMessages = append(chatMessages, domain.ChatMessage{Role: "user", Content: lastUserMessage})

	// Main loop — uses native function calling via ChatWithTools
	for i := 1; i <= uc.limits.MaxIterations; i++ {
		if loopCtx.Err() != nil {
			fallbackReason = "timeout"
			break
		}
		iterations = i

		plannerCtx, plannerCancel := context.WithTimeout(loopCtx, uc.limits.PlannerTimeout)
		chatResult, err := uc.querySvc.ChatWithTools(plannerCtx, chatMessages, toolSchemas)
		plannerCancel()
		if err != nil {
			if isAgentTimeoutError(err) {
				fallbackReason = "timeout"
			} else {
				fallbackReason = "planner_error"
				slog.Error("agent_planner_error",
					"error", err.Error(),
					"iteration", i,
				)
			}
			break
		}

		// If LLM returned a text response — final answer
		if len(chatResult.ToolCalls) == 0 && chatResult.Content != "" {
			finalAnswer = chatResult.Content
			break
		}

		// If LLM returned tool calls — execute all of them
		if len(chatResult.ToolCalls) > 0 {
			// Add assistant message with tool calls to conversation
			chatMessages = append(chatMessages, domain.ChatMessage{
				Role:      "assistant",
				ToolCalls: chatResult.ToolCalls,
			})

			var iterEvents []domain.AgentToolEvent

			if len(chatResult.ToolCalls) > 1 {
				// Parallel execution for multiple tool calls
				iterEvents = make([]domain.AgentToolEvent, len(chatResult.ToolCalls))
				var wg sync.WaitGroup
				for i, tc := range chatResult.ToolCalls {
					wg.Add(1)
					go func(idx int, call domain.ToolCall) {
						defer wg.Done()
						var ev domain.AgentToolEvent
						ttl := cacheTTLForTool(call.Function.Name)
						if ttl > 0 {
							argsKey := argsToKey(call.Function.Arguments)
							if cached, ok := uc.toolResultCache.get(call.Function.Name, argsKey); ok {
								iterEvents[idx] = domain.AgentToolEvent{Tool: call.Function.Name, Status: "ok", Output: cached}
								if onToolStatus != nil {
									onToolStatus(call.Function.Name, "ok")
								}
								return
							}
						}
						if onToolStatus != nil {
							onToolStatus(call.Function.Name, "running")
						}
						toolCtx, toolCancel := context.WithTimeout(loopCtx, uc.limits.ToolTimeout)
						ev, execErr := uc.executeToolCall(toolCtx, userID, call, lastUserMessage)
						toolCancel()
						if execErr != nil {
							errorPayload, _ := json.Marshal(map[string]string{"error": execErr.Error()})
							ev = domain.AgentToolEvent{Tool: call.Function.Name, Status: "error", Output: string(errorPayload)}
						} else if ttl > 0 {
							uc.toolResultCache.set(call.Function.Name, argsToKey(call.Function.Arguments), ev.Output, ttl)
						}
						iterEvents[idx] = ev
						if onToolStatus != nil {
							onToolStatus(ev.Tool, ev.Status)
						}
					}(i, tc)
				}
				wg.Wait()
			} else {
				// Single tool call — sequential path
				tc := chatResult.ToolCalls[0]
				var event domain.AgentToolEvent
				cached := false
				ttl := cacheTTLForTool(tc.Function.Name)
				if ttl > 0 {
					argsKey := argsToKey(tc.Function.Arguments)
					if cachedOutput, ok := uc.toolResultCache.get(tc.Function.Name, argsKey); ok {
						event = domain.AgentToolEvent{Tool: tc.Function.Name, Status: "ok", Output: cachedOutput}
						cached = true
						if onToolStatus != nil {
							onToolStatus(tc.Function.Name, "ok")
						}
					}
				}
				if !cached {
					if onToolStatus != nil {
						onToolStatus(tc.Function.Name, "running")
					}
					toolCtx, toolCancel := context.WithTimeout(loopCtx, uc.limits.ToolTimeout)
					var execErr error
					event, execErr = uc.executeToolCall(toolCtx, userID, tc, lastUserMessage)
					toolCancel()
					if execErr != nil {
						if isAgentTimeoutError(execErr) {
							fallbackReason = "timeout"
						}
						errorPayload, _ := json.Marshal(map[string]string{"error": execErr.Error()})
						event = domain.AgentToolEvent{Tool: tc.Function.Name, Status: "error", Output: string(errorPayload)}
					} else if ttl > 0 {
						uc.toolResultCache.set(tc.Function.Name, argsToKey(tc.Function.Arguments), event.Output, ttl)
					}
					if onToolStatus != nil {
						onToolStatus(event.Tool, event.Status)
					}
				}
				iterEvents = []domain.AgentToolEvent{event}
			}

			// Process collected events (thinking lines, FS hints, summarize, track tools, append messages)
			for idx, event := range iterEvents {
				tc := chatResult.ToolCalls[idx]
				if event.Status == "error" {
					thinkingLines = append(thinkingLines, fmt.Sprintf("✗ Tool %s error: %s", tc.Function.Name, event.Output))
				} else {
					thinkingLines = append(thinkingLines, fmt.Sprintf("✓ Tool %s: ok", event.Tool))
				}

				if uc.agentMetrics != nil {
					uc.agentMetrics.ToolCallTotal.WithLabelValues(event.Tool, event.Status).Inc()
				}

				if event.Status == "ok" && len(event.Output) >= 200 {
					uc.maybePersistToolMemory(loopCtx, userID, conversationID, event)
				}

				if isRecoverableToolError(event.Output) {
					fsCtx := probeFilesystemContext(loopCtx, uc.toolRegistry)
					event.Output = addFSHintToError(event.Output, fsCtx)
				}

				event.Output = maybeSummarize(event.Output, 2048)

				toolEvents = append(toolEvents, event)
				if event.Tool != "" {
					if _, seen := toolSet[event.Tool]; !seen {
						toolSet[event.Tool] = struct{}{}
						toolsInvoked = append(toolsInvoked, event.Tool)
					}
				}

				chatMessages = append(chatMessages, domain.ChatMessage{
					Role:       "tool",
					Content:    event.Output,
					ToolCallID: tc.ID,
				})
			}

			if fallbackReason == "timeout" {
				break
			}
			continue
		}

		// Neither content nor tool calls — unexpected
		fallbackReason = "empty_response"
		break
	}

	if fallbackReason == "" && finalAnswer == "" {
		fallbackReason = "max_iterations"
	}
	if finalAnswer == "" && shouldFallbackToRAG(fallbackReason) {
		thinkingLines = append(thinkingLines, "Fallback: searching knowledge base directly")
		fallbackAnswer, fallbackErr := uc.answerFromKnowledgeFallback(ctx, lastUserMessage)
		if fallbackErr == nil && strings.TrimSpace(fallbackAnswer) != "" {
			finalAnswer = fallbackAnswer
		}
	}
	if finalAnswer == "" {
		finalAnswer = "I reached the current execution limits. Please refine the request and try again."
	}

	thinkingContent := strings.Join(thinkingLines, "\n")
	if thinkingContent != "" {
		finalAnswer = fmt.Sprintf("<think>\n%s\n</think>\n\n%s", thinkingContent, finalAnswer)
	}

	for _, event := range toolEvents {
		if err := uc.conversations.AppendMessage(ctx, domain.ConversationMessage{
			ID:             uuid.NewString(),
			UserID:         userID,
			ConversationID: conversationID,
			Role:           "tool",
			Content:        sanitizeUTF8(event.Output),
			ToolName:       event.Tool,
			UserTurn:       turn,
			CreatedAt:      time.Now().UTC(),
		}); err != nil {
			return nil, fmt.Errorf("append tool message: %w", err)
		}
	}

	if err := uc.conversations.AppendMessage(ctx, domain.ConversationMessage{
		ID:             uuid.NewString(),
		UserID:         userID,
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        sanitizeUTF8(finalAnswer),
		UserTurn:       turn,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("append assistant message: %w", err)
	}

	summaryCreated, err := uc.maybePersistSummary(ctx, userID, conversationID, turn, req.SessionEnd)
	if err != nil {
		return nil, err
	}

	if uc.agentMetrics != nil {
		uc.agentMetrics.IterationsPerRequest.Observe(float64(iterations))
		uc.agentMetrics.RequestDuration.Observe(time.Since(requestStart).Seconds())
	}

	return &domain.AgentRunResult{
		ConversationID: conversationID,
		Answer:         finalAnswer,
		Thinking:       thinkingContent,
		Iterations:     iterations,
		MemoryHits:     len(memoryHits),
		SummaryCreated: summaryCreated,
		ToolsInvoked:   toolsInvoked,
		FallbackReason: fallbackReason,
		ToolEvents:     toolEvents,
	}, nil
}

func shouldFallbackToRAG(reason string) bool {
	switch reason {
	case "planner_invalid_json", "planner_error", "timeout":
		return true
	default:
		return false
	}
}

func isAgentTimeoutError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

func (uc *AgentChatUseCase) answerFromKnowledgeFallback(ctx context.Context, question string) (string, error) {
	fallbackCtx, cancel := context.WithTimeout(ctx, uc.limits.ToolTimeout)
	defer cancel()

	answer, err := uc.querySvc.Answer(fallbackCtx, question, uc.limits.KnowledgeTopK, domain.SearchFilter{})
	if err != nil {
		return "", fmt.Errorf("rag fallback answer: %w", err)
	}
	text := strings.TrimSpace(answer.Text)
	if text == "" {
		return "", fmt.Errorf("rag fallback answer is empty")
	}
	return text, nil
}

func latestUserInput(messages []domain.AgentInputMessage) (string, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(messages[i].Role), "user") {
			content := strings.TrimSpace(messages[i].Content)
			if content != "" {
				return content, true
			}
		}
	}
	return "", false
}

func (uc *AgentChatUseCase) executeWebSearch(ctx context.Context, step domain.AgentPlanStep, fallbackQuestion string) (domain.AgentToolEvent, error) {
	if uc.webSearcher == nil {
		return domain.AgentToolEvent{}, fmt.Errorf("web search is not configured")
	}
	query := stringInput(step.Input, "query", fallbackQuestion)
	limit := intInput(step.Input, "limit", 5)
	results, err := uc.webSearcher.Search(ctx, query, limit)
	if err != nil {
		return domain.AgentToolEvent{}, fmt.Errorf("web search: %w", err)
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	})
	return domain.AgentToolEvent{
		Tool:   agentToolWebSearch,
		Status: "ok",
		Output: string(payload),
	}, nil
}

func (uc *AgentChatUseCase) executeObsidianWrite(ctx context.Context, step domain.AgentPlanStep) (domain.AgentToolEvent, error) {
	if uc.obsidianWriter == nil {
		return domain.AgentToolEvent{}, fmt.Errorf("obsidian write is not configured")
	}
	vaultID := stringInput(step.Input, "vault", "")
	title := stringInput(step.Input, "title", "")
	content := stringInput(step.Input, "content", "")
	folder := stringInput(step.Input, "folder", "")
	if title == "" {
		return domain.AgentToolEvent{}, fmt.Errorf("obsidian_write requires title")
	}
	if content == "" {
		return domain.AgentToolEvent{}, fmt.Errorf("obsidian_write requires content")
	}
	path, err := uc.obsidianWriter.CreateNote(ctx, vaultID, title, content, folder)
	if err != nil {
		return domain.AgentToolEvent{}, fmt.Errorf("obsidian write: %w", err)
	}
	payload, _ := json.Marshal(map[string]string{
		"status": "created",
		"vault":  vaultID,
		"title":  title,
		"path":   path,
	})
	return domain.AgentToolEvent{
		Tool:   agentToolObsidianWrite,
		Status: "ok",
		Output: string(payload),
	}, nil
}

func (uc *AgentChatUseCase) executeTaskTool(ctx context.Context, userID string, step domain.AgentPlanStep) (domain.AgentToolEvent, error) {
	action := step.Action
	if action == "" {
		action = strings.ToLower(strings.TrimSpace(stringInput(step.Input, "action", "")))
	}

	switch action {
	case "create":
		title := strings.TrimSpace(stringInput(step.Input, "title", ""))
		if title == "" {
			return domain.AgentToolEvent{}, fmt.Errorf("task create requires title")
		}
		now := time.Now().UTC()
		task := &domain.Task{
			ID:        uuid.NewString(),
			UserID:    userID,
			Title:     title,
			Details:   strings.TrimSpace(stringInput(step.Input, "details", "")),
			Status:    domain.TaskStatusOpen,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if dueRaw := strings.TrimSpace(stringInput(step.Input, "due_at", "")); dueRaw != "" {
			dueAt, err := parseTimeRFC3339(dueRaw)
			if err != nil {
				return domain.AgentToolEvent{}, fmt.Errorf("task create due_at: %w", err)
			}
			task.DueAt = &dueAt
		}
		if err := uc.tasks.CreateTask(ctx, task); err != nil {
			return domain.AgentToolEvent{}, fmt.Errorf("task create: %w", err)
		}
		payload, _ := json.Marshal(task)
		return domain.AgentToolEvent{Tool: agentToolTask, Status: "ok", Output: string(payload)}, nil
	case "list":
		includeDeleted := boolInput(step.Input, "include_deleted", false)
		tasks, err := uc.tasks.ListTasks(ctx, userID, includeDeleted)
		if err != nil {
			return domain.AgentToolEvent{}, fmt.Errorf("task list: %w", err)
		}
		payload, _ := json.Marshal(tasks)
		return domain.AgentToolEvent{Tool: agentToolTask, Status: "ok", Output: string(payload)}, nil
	case "get":
		id := strings.TrimSpace(stringInput(step.Input, "id", ""))
		if id == "" {
			return domain.AgentToolEvent{}, fmt.Errorf("task get requires id")
		}
		task, err := uc.tasks.GetTaskByID(ctx, userID, id)
		if err != nil {
			return domain.AgentToolEvent{}, fmt.Errorf("task get: %w", err)
		}
		payload, _ := json.Marshal(task)
		return domain.AgentToolEvent{Tool: agentToolTask, Status: "ok", Output: string(payload)}, nil
	case "update":
		id := strings.TrimSpace(stringInput(step.Input, "id", ""))
		if id == "" {
			return domain.AgentToolEvent{}, fmt.Errorf("task update requires id")
		}
		task, err := uc.tasks.GetTaskByID(ctx, userID, id)
		if err != nil {
			return domain.AgentToolEvent{}, fmt.Errorf("task update load: %w", err)
		}
		if title := strings.TrimSpace(stringInput(step.Input, "title", "")); title != "" {
			task.Title = title
		}
		if details, ok := step.Input["details"]; ok {
			task.Details = strings.TrimSpace(fmt.Sprint(details))
		}
		if status := strings.TrimSpace(stringInput(step.Input, "status", "")); status != "" {
			switch domain.TaskStatus(strings.ToLower(status)) {
			case domain.TaskStatusOpen, domain.TaskStatusCompleted:
				task.Status = domain.TaskStatus(strings.ToLower(status))
			default:
				return domain.AgentToolEvent{}, fmt.Errorf("unsupported task status: %s", status)
			}
		}
		if dueRaw, ok := step.Input["due_at"]; ok {
			dueStr := strings.TrimSpace(fmt.Sprint(dueRaw))
			if dueStr == "" || strings.EqualFold(dueStr, "null") {
				task.DueAt = nil
			} else {
				dueAt, err := parseTimeRFC3339(dueStr)
				if err != nil {
					return domain.AgentToolEvent{}, fmt.Errorf("task update due_at: %w", err)
				}
				task.DueAt = &dueAt
			}
		}
		task.UpdatedAt = time.Now().UTC()
		if err := uc.tasks.UpdateTask(ctx, task); err != nil {
			return domain.AgentToolEvent{}, fmt.Errorf("task update: %w", err)
		}
		payload, _ := json.Marshal(task)
		return domain.AgentToolEvent{Tool: agentToolTask, Status: "ok", Output: string(payload)}, nil
	case "delete":
		id := strings.TrimSpace(stringInput(step.Input, "id", ""))
		if id == "" {
			return domain.AgentToolEvent{}, fmt.Errorf("task delete requires id")
		}
		if err := uc.tasks.SoftDeleteTask(ctx, userID, id); err != nil {
			return domain.AgentToolEvent{}, fmt.Errorf("task delete: %w", err)
		}
		payload, _ := json.Marshal(map[string]string{"id": id, "status": "deleted"})
		return domain.AgentToolEvent{Tool: agentToolTask, Status: "ok", Output: string(payload)}, nil
	case "complete":
		id := strings.TrimSpace(stringInput(step.Input, "id", ""))
		if id == "" {
			return domain.AgentToolEvent{}, fmt.Errorf("task complete requires id")
		}
		task, err := uc.tasks.GetTaskByID(ctx, userID, id)
		if err != nil {
			return domain.AgentToolEvent{}, fmt.Errorf("task complete load: %w", err)
		}
		task.Status = domain.TaskStatusCompleted
		task.UpdatedAt = time.Now().UTC()
		if err := uc.tasks.UpdateTask(ctx, task); err != nil {
			return domain.AgentToolEvent{}, fmt.Errorf("task complete: %w", err)
		}
		payload, _ := json.Marshal(task)
		return domain.AgentToolEvent{Tool: agentToolTask, Status: "ok", Output: string(payload)}, nil
	default:
		return domain.AgentToolEvent{}, fmt.Errorf("unsupported task action: %s", action)
	}
}

func (uc *AgentChatUseCase) maybePersistSummary(ctx context.Context, userID, conversationID string, currentTurn int, force bool) (bool, error) {
	lastTurn, err := uc.memories.GetLastSummaryEndTurn(ctx, userID, conversationID)
	if err != nil {
		return false, fmt.Errorf("get last summary turn: %w", err)
	}
	if currentTurn <= lastTurn {
		return false, nil
	}

	turnCount := currentTurn - lastTurn
	if !force && turnCount < uc.limits.SummaryEveryTurns {
		return false, nil
	}

	messages, err := uc.conversations.ListMessagesByTurnRange(ctx, userID, conversationID, lastTurn+1, currentTurn)
	if err != nil {
		return false, fmt.Errorf("load messages for summary: %w", err)
	}
	if len(messages) == 0 {
		return false, nil
	}

	lines := make([]string, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", msg.Role, content))
	}
	if len(lines) == 0 {
		return false, nil
	}

	prompt := fmt.Sprintf(`Summarize the following conversation turns in concise factual form.
Include user goals, key facts, decisions, and explicit todo items.
Return plain text.

%s`, strings.Join(lines, "\n"))

	summaryText, err := uc.querySvc.GenerateFromPrompt(ctx, prompt)
	if err != nil {
		return false, fmt.Errorf("generate summary: %w", err)
	}
	summaryText = strings.TrimSpace(summaryText)
	if summaryText == "" {
		return false, nil
	}

	summary := &domain.MemorySummary{
		ID:             uuid.NewString(),
		UserID:         userID,
		ConversationID: conversationID,
		TurnFrom:       lastTurn + 1,
		TurnTo:         currentTurn,
		Summary:        summaryText,
		CreatedAt:      time.Now().UTC(),
	}
	if err := uc.memories.CreateSummary(ctx, summary); err != nil {
		return false, fmt.Errorf("create summary: %w", err)
	}

	vector, err := uc.embedder.EmbedQuery(ctx, summaryText)
	if err == nil && len(vector) > 0 {
		if err := uc.memoryVector.IndexSummary(ctx, *summary, vector); err != nil {
			return false, fmt.Errorf("index summary: %w", err)
		}
	}

	if err := uc.conversations.UpdateLastSummaryEndTurn(ctx, userID, conversationID, currentTurn); err != nil {
		return false, fmt.Errorf("update last summary end turn: %w", err)
	}
	return true, nil
}

func stringInput(input map[string]interface{}, key, fallback string) string {
	if input == nil {
		return fallback
	}
	value, ok := input[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func intInput(input map[string]interface{}, key string, fallback int) int {
	if input == nil {
		return fallback
	}
	value, ok := input[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return fallback
		}
		return n
	default:
		return fallback
	}
}

func boolInput(input map[string]interface{}, key string, fallback bool) bool {
	if input == nil {
		return fallback
	}
	value, ok := input[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}

func parseTimeRFC3339(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

// toolSchemasFromRegistry converts MCPToolRegistry tools to domain.ToolSchema
// for use with the ChatWithTools API.
func toolSchemasFromRegistry(registry ports.MCPToolRegistry, webSearchAvailable bool) []domain.ToolSchema {
	if registry == nil {
		return nil
	}
	var schemas []domain.ToolSchema
	for _, t := range registry.ListTools() {
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

// buildSystemPrompt builds the system prompt for the function-calling agent loop,
// incorporating intent-specific guidance and long-term memory.
func buildSystemPrompt(ctx context.Context, intent Intent, memoryHits []domain.MemoryHit, registry ports.MCPToolRegistry, vaults []ports.AgentVaultInfo) string {
	var sb strings.Builder
	sb.WriteString(`You are a personal AI assistant. You have access to tools for searching knowledge, executing code, managing files, and more.

RULES:
- Always respond in Russian.
- Use tools when they help answer the question. Call them directly.
- After using a tool, analyze its output and provide a complete answer to the user.
- If you don't need tools, answer directly from your knowledge.
- When answering from knowledge base results, cite the sources.
- IMPORTANT: Always consider the full conversation history when formulating tool queries. If the user asks a follow-up question (e.g. "а в чем суть последнего обновления?"), use context from previous messages to build a specific query — not just the latest message in isolation.
- When using web_search, always build specific, disambiguated queries. For example: if the conversation is about Rust programming language, search "Rust programming language news 2025", NOT just "Rust news". Add domain-specific keywords to avoid ambiguity (e.g. "Rust lang" vs "Rust game").
- When user asks to save/write/create a note in Obsidian, use the obsidian_write tool. Pass vault id from the list of available vaults below.
`)

	sb.WriteString("\n")
	sb.WriteString(systemPromptForIntent(intent))
	sb.WriteString("\n")

	if len(vaults) > 0 {
		sb.WriteString("\nAvailable Obsidian vaults for obsidian_write tool:\n")
		for _, v := range vaults {
			fmt.Fprintf(&sb, "- %s (%s)\n", v.ID, v.Name)
		}
	}

	if len(memoryHits) > 0 {
		sb.WriteString("\nRelevant long-term memory:\n")
		for _, hit := range memoryHits {
			fmt.Fprintf(&sb, "- [score=%.3f] %s\n", hit.Score, strings.TrimSpace(hit.Summary.Summary))
		}
	}

	if registry != nil {
		if fsCtx := probeFilesystemContext(ctx, registry); fsCtx != "" {
			sb.WriteString("\n")
			sb.WriteString(fsCtx)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// executeToolCall dispatches a domain.ToolCall to the appropriate tool handler.
// It replaces the old executeTool that worked with AgentPlanStep.
func (uc *AgentChatUseCase) executeToolCall(ctx context.Context, userID string, tc domain.ToolCall, fallbackQuestion string) (domain.AgentToolEvent, error) {
	toolName := tc.Function.Name
	args := tc.Function.Arguments

	// Guardrails for code execution
	if toolName == "execute_python" || toolName == "execute_bash" {
		code := stringFromArgs(args, "code", stringFromArgs(args, "command", ""))
		if err := checkCodeSafety(code); err != nil {
			return domain.AgentToolEvent{Tool: toolName, Status: "error", Output: err.Error()}, nil
		}
	}

	switch toolName {
	case agentToolKnowledgeSearch:
		question := stringFromArgs(args, "question", fallbackQuestion)
		limit := intFromArgs(args, "limit", uc.limits.KnowledgeTopK)
		if expanded := uc.expandQueryWithGraph(ctx, question); expanded != "" {
			question = question + " " + expanded
		}
		answer, err := uc.querySvc.Answer(ctx, question, limit, domain.SearchFilter{})
		if err != nil {
			return domain.AgentToolEvent{}, fmt.Errorf("knowledge search: %w", err)
		}
		payload, _ := json.Marshal(map[string]any{"question": question, "answer": answer.Text, "sources": answer.Sources})
		return domain.AgentToolEvent{Tool: toolName, Status: "ok", Output: string(payload)}, nil

	case agentToolWebSearch:
		return uc.executeWebSearch(ctx, domain.AgentPlanStep{Input: args, Tool: toolName}, fallbackQuestion)

	case agentToolObsidianWrite:
		return uc.executeObsidianWrite(ctx, domain.AgentPlanStep{Input: args, Tool: toolName})

	case agentToolTask:
		action := stringFromArgs(args, "action", "")
		return uc.executeTaskTool(ctx, userID, domain.AgentPlanStep{Input: args, Tool: toolName, Action: action})

	default:
		// MCP tool
		if uc.toolRegistry != nil && !uc.toolRegistry.IsBuiltIn(toolName) {
			output, err := uc.toolRegistry.CallMCPTool(ctx, toolName, args)
			if err != nil {
				return domain.AgentToolEvent{}, fmt.Errorf("mcp tool %s: %w", toolName, err)
			}
			return domain.AgentToolEvent{Tool: toolName, Status: "ok", Output: output}, nil
		}
		return domain.AgentToolEvent{}, fmt.Errorf("unsupported tool: %s", toolName)
	}
}

func (uc *AgentChatUseCase) maybePersistToolMemory(ctx context.Context, userID, conversationID string, event domain.AgentToolEvent) {
	if event.Status != "ok" || len(event.Output) < 200 {
		return
	}
	summary := fmt.Sprintf("Tool %s result: %s", event.Tool, maybeSummarize(event.Output, 500))
	memSummary := &domain.MemorySummary{
		ID:             uuid.NewString(),
		UserID:         userID,
		ConversationID: conversationID,
		Summary:        summary,
		CreatedAt:      time.Now().UTC(),
	}
	if err := uc.memories.CreateSummary(ctx, memSummary); err != nil {
		return
	}
	vector, err := uc.embedder.EmbedQuery(ctx, summary)
	if err == nil && len(vector) > 0 {
		_ = uc.memoryVector.IndexSummary(ctx, *memSummary, vector)
	}
}

func stringFromArgs(args map[string]any, key, fallback string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return fallback
}

func intFromArgs(args map[string]any, key string, fallback int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return fallback
}

func (uc *AgentChatUseCase) expandQueryWithGraph(ctx context.Context, query string) string {
	if uc.graphStore == nil {
		return ""
	}

	tokens := strings.Fields(query)
	var expansions []string
	seen := make(map[string]bool)

	for _, token := range tokens {
		if len(token) < 3 {
			continue
		}
		nodes, err := uc.graphStore.FindByTitle(ctx, token)
		if err != nil || len(nodes) == 0 {
			continue
		}

		limit := 2
		if len(nodes) < limit {
			limit = len(nodes)
		}
		for _, node := range nodes[:limit] {
			related, err := uc.graphStore.GetRelated(ctx, node.ID, 1, 3)
			if err != nil {
				continue
			}
			for _, rel := range related {
				targetID := rel.TargetID
				if targetID == node.ID {
					targetID = rel.SourceID
				}
				// Look up the target node's title.
				targetNodes, err := uc.graphStore.FindByTitle(ctx, targetID)
				if err != nil || len(targetNodes) == 0 {
					continue
				}
				title := targetNodes[0].Title
				if title != "" && !seen[title] {
					seen[title] = true
					expansions = append(expansions, title)
				}
			}
		}
	}

	if len(expansions) > 5 {
		expansions = expansions[:5]
	}
	if len(expansions) > 0 {
		slog.Info("graph_query_expansion", "expansions", expansions)
	}
	return strings.Join(expansions, " ")
}

// sanitizeUTF8 strips invalid UTF-8 byte sequences (e.g. truncated multi-byte
// Cyrillic characters from SearXNG) so the string can be safely stored in
// PostgreSQL without triggering "invalid byte sequence for encoding UTF8".
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			i++ // skip invalid byte
			continue
		}
		b.WriteRune(r)
		i += size
	}
	return b.String()
}
