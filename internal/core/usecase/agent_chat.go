package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

const (
	agentToolKnowledgeSearch = "knowledge_search"
	agentToolWebSearch       = "web_search"
	agentToolObsidianWrite   = "obsidian_write"
	agentToolTask            = "task_tool"
)

type AgentChatUseCase struct {
	querySvc       ports.DocumentQueryService
	embedder       ports.Embedder
	conversations  ports.ConversationStore
	tasks          ports.TaskStore
	memories       ports.MemoryStore
	memoryVector   ports.MemoryVectorStore
	webSearcher    ports.WebSearcher
	obsidianWriter ports.ObsidianNoteWriter
	limits         domain.AgentLimits
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
	limits domain.AgentLimits,
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
		limits.ShortMemoryMessages = 12
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
		querySvc:       querySvc,
		embedder:       embedder,
		conversations:  conversations,
		tasks:          tasks,
		memories:       memories,
		memoryVector:   memoryVector,
		webSearcher:    webSearcher,
		obsidianWriter: obsidianWriter,
		limits:         limits,
	}
}

// SetObsidianWriter sets the ObsidianNoteWriter after construction (to break circular dependency with Router).
func (uc *AgentChatUseCase) SetObsidianWriter(w ports.ObsidianNoteWriter) {
	uc.obsidianWriter = w
}

func (uc *AgentChatUseCase) Complete(ctx context.Context, req domain.AgentChatRequest) (*domain.AgentRunResult, error) {
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

	scratchpad := make([]string, 0, uc.limits.MaxIterations)
	thinkingLines := make([]string, 0, uc.limits.MaxIterations)
	toolEvents := make([]domain.AgentToolEvent, 0, uc.limits.MaxIterations)
	toolsInvoked := make([]string, 0, uc.limits.MaxIterations)
	toolSet := make(map[string]struct{})
	finalAnswer := ""
	fallbackReason := ""
	iterations := 0

	webSearchAvailable := uc.webSearcher != nil

	for i := 1; i <= uc.limits.MaxIterations; i++ {
		if loopCtx.Err() != nil {
			fallbackReason = "timeout"
			break
		}

		iterations = i
		plannerCtx, plannerCancel := context.WithTimeout(loopCtx, uc.limits.PlannerTimeout)
		planRaw, err := uc.querySvc.GenerateJSONFromPrompt(plannerCtx, buildPlannerPrompt(lastUserMessage, shortMemory, memoryHits, scratchpad, webSearchAvailable))
		plannerCancel()
		if err != nil {
			if isAgentTimeoutError(err) {
				fallbackReason = "timeout"
			} else {
				fallbackReason = "planner_error"
			}
			break
		}

		step, err := parseAgentStep(planRaw)
		if err != nil {
			repairCtx, repairCancel := context.WithTimeout(loopCtx, uc.limits.PlannerTimeout)
			repairedRaw, repairErr := uc.querySvc.GenerateJSONFromPrompt(repairCtx, buildPlannerRepairPrompt(planRaw))
			repairCancel()
			if repairErr != nil {
				if isAgentTimeoutError(repairErr) {
					fallbackReason = "timeout"
				} else {
					fallbackReason = "planner_invalid_json"
				}
				break
			}
			step, err = parseAgentStep(repairedRaw)
			if err != nil {
				fallbackReason = "planner_invalid_json"
				break
			}
		}

		if thinking := strings.TrimSpace(step.Thinking); thinking != "" {
			thinkingLines = append(thinkingLines, thinking)
		}

		switch strings.ToLower(strings.TrimSpace(step.Type)) {
		case "final":
			finalAnswer = strings.TrimSpace(step.Answer)
			if finalAnswer == "" {
				finalAnswer = "I could not produce a final answer from the current context."
				fallbackReason = "empty_final_answer"
			}
		case "tool":
			thinkingLines = append(thinkingLines, fmt.Sprintf("→ Using tool: %s", step.Tool))
			toolCtx, toolCancel := context.WithTimeout(loopCtx, uc.limits.ToolTimeout)
			event, execErr := uc.executeTool(toolCtx, userID, step, lastUserMessage)
			toolCancel()
			if execErr != nil {
				if isAgentTimeoutError(execErr) {
					fallbackReason = "timeout"
				}
				errorPayload, _ := json.Marshal(map[string]string{"error": execErr.Error()})
				event = domain.AgentToolEvent{
					Tool:   step.Tool,
					Status: "error",
					Output: string(errorPayload),
				}
				thinkingLines = append(thinkingLines, fmt.Sprintf("✗ Tool %s error: %s", step.Tool, execErr.Error()))
			} else {
				thinkingLines = append(thinkingLines, fmt.Sprintf("✓ Tool %s: ok", event.Tool))
			}
			toolEvents = append(toolEvents, event)
			if event.Tool != "" {
				if _, seen := toolSet[event.Tool]; !seen {
					toolSet[event.Tool] = struct{}{}
					toolsInvoked = append(toolsInvoked, event.Tool)
				}
			}
			scratchpad = append(scratchpad, fmt.Sprintf("%s:%s", event.Tool, event.Output))
			if fallbackReason == "timeout" {
				break
			}
		default:
			slog.Warn("unsupported_step_type", "raw", planRaw, "parsed_type", step.Type, "parsed_tool", step.Tool, "parsed_action", step.Action, "parsed_answer_len", len(step.Answer))
			fallbackReason = "unsupported_step_type"
		}

		if finalAnswer != "" || fallbackReason != "" {
			break
		}
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
			Content:        event.Output,
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
		Content:        finalAnswer,
		UserTurn:       turn,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("append assistant message: %w", err)
	}

	summaryCreated, err := uc.maybePersistSummary(ctx, userID, conversationID, turn, req.SessionEnd)
	if err != nil {
		return nil, err
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

// knownTools is the set of tool names the agent supports.
var knownTools = map[string]bool{
	"knowledge_search": true,
	"web_search":       true,
	"obsidian_write":   true,
	"task_tool":        true,
}

func parseAgentStep(raw string) (domain.AgentPlanStep, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return domain.AgentPlanStep{}, fmt.Errorf("empty planner response")
	}

	// Parse into both typed struct and generic map for flexible extraction.
	var step domain.AgentPlanStep
	if err := json.Unmarshal([]byte(raw), &step); err != nil {
		return domain.AgentPlanStep{}, fmt.Errorf("unmarshal planner json: %w", err)
	}

	var generic map[string]any
	_ = json.Unmarshal([]byte(raw), &generic)

	step.Type = strings.ToLower(strings.TrimSpace(step.Type))
	step.Tool = strings.ToLower(strings.TrimSpace(step.Tool))
	step.Action = strings.ToLower(strings.TrimSpace(step.Action))

	// Try to find a tool name anywhere in known fields if step.Tool is empty.
	if step.Tool == "" {
		for _, key := range []string{"tool", "tool_name", "name", "function"} {
			if v, ok := generic[key]; ok {
				if s, ok := v.(string); ok {
					s = strings.ToLower(strings.TrimSpace(s))
					if knownTools[s] {
						step.Tool = s
						break
					}
				}
			}
		}
	}

	// Infer step.Type when the model omits it.
	if step.Type == "" {
		switch {
		case step.Tool != "" && knownTools[step.Tool]:
			step.Type = "tool"
		case step.Action == "tool" || step.Action == "search":
			step.Type = "tool"
		case knownTools[step.Action]:
			step.Type = "tool"
			step.Tool = step.Action
			step.Action = ""
		case strings.TrimSpace(step.Answer) != "":
			step.Type = "final"
		case step.Action == "final" || step.Action == "answer" || step.Action == "respond":
			step.Type = "final"
		}
	}

	// Scan generic map for answer under alternate keys.
	if step.Type == "" {
		for _, key := range []string{"answer", "response", "result", "reply", "text"} {
			if v, ok := generic[key]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					step.Type = "final"
					step.Answer = s
					break
				}
			}
		}
	}

	return step, nil
}

func buildPlannerPrompt(userMessage string, shortMemory []domain.ConversationMessage, memoryHits []domain.MemoryHit, scratchpad []string, webSearchAvailable bool) string {
	shortLines := make([]string, 0, len(shortMemory))
	for _, msg := range shortMemory {
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		shortLines = append(shortLines, fmt.Sprintf("%s: %s", role, content))
	}
	hitLines := make([]string, 0, len(memoryHits))
	for _, hit := range memoryHits {
		hitLines = append(hitLines, fmt.Sprintf("- [score=%.3f] %s", hit.Score, strings.TrimSpace(hit.Summary.Summary)))
	}
	if len(shortLines) == 0 {
		shortLines = append(shortLines, "(empty)")
	}
	if len(hitLines) == 0 {
		hitLines = append(hitLines, "(empty)")
	}
	if len(scratchpad) == 0 {
		scratchpad = append(scratchpad, "(no tool outputs yet)")
	}

	webSearchSchema := ""
	if webSearchAvailable {
		webSearchSchema = `
{"type":"tool","tool":"web_search","thinking":"why searching the web","input":{"query":"search query","limit":5}}
`
	}

	return fmt.Sprintf(`You are a planning component for a personal AI assistant.
Your task: decide the SINGLE next step. Return ONE JSON object per call.

RULES:
- If scratchpad already contains tool output — you MUST return a "final" step with the answer. Do NOT call the same tool again.
- If scratchpad is empty — follow the cascade below.
- ALWAYS respond in Russian (except JSON keys).
- ALWAYS include a "thinking" field (in Russian).

CASCADE STRATEGY (follow in order):
1. knowledge_search — search the knowledge base first
2. If knowledge_search returned nothing useful — return "final" answer from your own knowledge, noting: "В базе знаний информация не найдена. Отвечаю из общих знаний."
3. If you don't know either — use web_search
4. After web_search — return "final" with the answer AND suggest: "Хотите сохранить эту информацию в Obsidian vault?"
5. Only use obsidian_write if the user explicitly asks to save

JSON schemas (use EXACTLY these field names):
{"type":"tool","tool":"knowledge_search","thinking":"...","input":{"question":"...","limit":5}}
%s{"type":"tool","tool":"obsidian_write","thinking":"...","input":{"vault":"...","title":"...","content":"...","folder":"..."}}
{"type":"tool","tool":"task_tool","thinking":"...","action":"create|list|get|update|delete|complete","input":{...}}
{"type":"final","thinking":"...","answer":"full answer text in Russian"}

Conversation short memory:
%s

Relevant long-term memory summaries:
%s

Scratchpad with previous tool outputs:
%s

Current user request:
%s
`, webSearchSchema, strings.Join(shortLines, "\n"), strings.Join(hitLines, "\n"), strings.Join(scratchpad, "\n"), userMessage)
}

func buildPlannerRepairPrompt(raw string) string {
	return fmt.Sprintf(`Convert the following text into a valid JSON object for this schema:
{"type":"tool","tool":"knowledge_search","thinking":"...","input":{"question":"...","limit":5}}
or {"type":"tool","tool":"web_search","thinking":"...","input":{"query":"...","limit":5}}
or {"type":"tool","tool":"obsidian_write","thinking":"...","input":{"vault":"...","title":"...","content":"...","folder":"..."}}
or {"type":"tool","tool":"task_tool","thinking":"...","action":"create|list|get|update|delete|complete","input":{...}}
or {"type":"final","thinking":"...","answer":"..."}
Return only JSON.
Text:
%s`, raw)
}

func (uc *AgentChatUseCase) executeTool(ctx context.Context, userID string, step domain.AgentPlanStep, fallbackQuestion string) (domain.AgentToolEvent, error) {
	switch step.Tool {
	case agentToolKnowledgeSearch:
		question := stringInput(step.Input, "question", fallbackQuestion)
		limit := intInput(step.Input, "limit", uc.limits.KnowledgeTopK)
		answer, err := uc.querySvc.Answer(ctx, question, limit, domain.SearchFilter{})
		if err != nil {
			return domain.AgentToolEvent{}, fmt.Errorf("knowledge search: %w", err)
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"question": question,
			"answer":   answer.Text,
			"sources":  answer.Sources,
		})
		return domain.AgentToolEvent{
			Tool:   agentToolKnowledgeSearch,
			Status: "ok",
			Output: string(payload),
		}, nil
	case agentToolWebSearch:
		return uc.executeWebSearch(ctx, step, fallbackQuestion)
	case agentToolObsidianWrite:
		return uc.executeObsidianWrite(ctx, step)
	case agentToolTask:
		return uc.executeTaskTool(ctx, userID, step)
	default:
		return domain.AgentToolEvent{}, fmt.Errorf("unsupported tool: %s", step.Tool)
	}
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
