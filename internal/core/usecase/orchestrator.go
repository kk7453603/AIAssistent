package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type OrchestratorUseCase struct {
	agentChat    ports.AgentChatService
	registry     *AgentRegistry
	memoryVector ports.MemoryVectorStore
	embedder     ports.Embedder
	generator    ports.AnswerGenerator
	orchStore    ports.OrchestrationStore
	maxSteps     int
}

func NewOrchestratorUseCase(
	agentChat ports.AgentChatService,
	registry *AgentRegistry,
	memoryVector ports.MemoryVectorStore,
	embedder ports.Embedder,
	generator ports.AnswerGenerator,
	orchStore ports.OrchestrationStore,
	maxSteps int,
) *OrchestratorUseCase {
	if maxSteps <= 0 {
		maxSteps = 8
	}
	return &OrchestratorUseCase{
		agentChat:    agentChat,
		registry:     registry,
		memoryVector: memoryVector,
		embedder:     embedder,
		generator:    generator,
		orchStore:    orchStore,
		maxSteps:     maxSteps,
	}
}

func (uc *OrchestratorUseCase) Execute(
	ctx context.Context,
	req domain.AgentChatRequest,
	onToolStatus domain.ToolStatusCallback,
) (*domain.AgentRunResult, error) {
	orchID := uuid.NewString()
	now := time.Now().UTC()
	lastMessage := orchLastUserMsg(req.Messages)

	plan, err := uc.planSteps(ctx, lastMessage)
	if err != nil {
		slog.Warn("orchestrator_plan_failed", "error", err)
		return uc.agentChat.Complete(ctx, req, onToolStatus)
	}

	orch := &domain.Orchestration{
		ID:             orchID,
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		Request:        lastMessage,
		Plan:           plan,
		Status:         "running",
		CreatedAt:      now,
	}
	if uc.orchStore != nil {
		_ = uc.orchStore.Create(ctx, orch)
	}

	var lastResult string
	stepIndex := 0

	for stepIndex < len(plan) && stepIndex < uc.maxSteps {
		step := plan[stepIndex]
		spec, ok := uc.registry.Get(step.Agent)
		if !ok {
			slog.Warn("orchestrator_unknown_agent", "agent", step.Agent)
			stepIndex++
			continue
		}

		if onToolStatus != nil {
			onToolStatus(fmt.Sprintf("orchestrator:%s", step.Agent), "started")
		}

		startTime := time.Now()

		memoryContext := uc.gatherMemoryContext(ctx, req.UserID, req.ConversationID, step.Task)

		agentPrompt := fmt.Sprintf("%s\n\nTask: %s", spec.SystemPrompt, step.Task)
		if memoryContext != "" {
			agentPrompt += fmt.Sprintf("\n\nContext from previous steps:\n%s", memoryContext)
		}
		if lastResult != "" {
			agentPrompt += fmt.Sprintf("\n\nPrevious agent result:\n%s", orchTruncate(lastResult, 2000))
		}

		agentReq := domain.AgentChatRequest{
			UserID:         req.UserID,
			ConversationID: req.ConversationID + "_orch_" + orchID,
			Messages: []domain.AgentInputMessage{
				{Role: "system", Content: agentPrompt},
				{Role: "user", Content: lastMessage},
			},
		}

		result, err := uc.agentChat.Complete(ctx, agentReq, nil)

		duration := float64(time.Since(startTime).Microseconds()) / 1000.0
		orchStep := domain.OrchestrationStep{
			Index:      stepIndex,
			Agent:      step.Agent,
			Task:       step.Task,
			Status:     "completed",
			StartedAt:  startTime,
			DurationMS: duration,
		}

		if err != nil {
			orchStep.Status = "failed"
			orchStep.Result = err.Error()
			slog.Warn("orchestrator_step_failed", "agent", step.Agent, "error", err)
		} else {
			orchStep.Result = result.Answer
			lastResult = result.Answer
			uc.saveToMemory(ctx, req.UserID, req.ConversationID, step.Agent, result.Answer)
		}

		if uc.orchStore != nil {
			_ = uc.orchStore.AddStep(ctx, orchID, orchStep)
		}

		if onToolStatus != nil {
			status := "completed"
			if orchStep.Status == "failed" {
				status = "failed"
			}
			onToolStatus(fmt.Sprintf("orchestrator:%s", step.Agent), status)
		}

		if step.Agent == "critic" && orchStep.Status == "completed" {
			if orchContainsCriticIssues(orchStep.Result) && stepIndex+2 < uc.maxSteps {
				plan = append(plan,
					domain.OrchestrationPlanStep{Agent: "writer", Task: "Fix issues found by critic: " + orchTruncate(orchStep.Result, 500)},
					domain.OrchestrationPlanStep{Agent: "critic", Task: "Re-check the fixed answer"},
				)
			}
		}

		stepIndex++
	}

	if uc.orchStore != nil {
		_ = uc.orchStore.Complete(ctx, orchID, "completed")
	}

	slog.Info("orchestration_completed", "id", orchID, "steps", stepIndex)

	return &domain.AgentRunResult{
		Answer:     lastResult,
		ToolEvents: nil,
	}, nil
}

func (uc *OrchestratorUseCase) planSteps(ctx context.Context, userMessage string) ([]domain.OrchestrationPlanStep, error) {
	agentNames := uc.registry.Names()
	prompt := fmt.Sprintf(`You are an orchestrator. Given the user request, decide which specialist agents to call and in what order.

Available specialists: %s

Return ONLY a JSON object: {"steps": [{"agent": "<name>", "task": "<specific task for this agent>"}]}
Do not include any explanation, only the JSON.

User request: %s`, strings.Join(agentNames, ", "), userMessage)

	respText, err := uc.generator.GenerateJSONFromPrompt(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("plan generation: %w", err)
	}

	var plan struct {
		Steps []domain.OrchestrationPlanStep `json:"steps"`
	}
	if err := json.Unmarshal([]byte(respText), &plan); err != nil {
		start := strings.Index(respText, "{")
		end := strings.LastIndex(respText, "}")
		if start >= 0 && end > start {
			if err2 := json.Unmarshal([]byte(respText[start:end+1]), &plan); err2 != nil {
				return nil, fmt.Errorf("parse plan: %w", err)
			}
		} else {
			return nil, fmt.Errorf("parse plan: %w", err)
		}
	}

	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("empty plan")
	}

	return plan.Steps, nil
}

func (uc *OrchestratorUseCase) gatherMemoryContext(ctx context.Context, userID, conversationID, task string) string {
	vec, err := uc.embedder.EmbedQuery(ctx, task)
	if err != nil {
		return ""
	}
	hits, err := uc.memoryVector.SearchSummaries(ctx, userID, conversationID, vec, 3)
	if err != nil || len(hits) == 0 {
		return ""
	}
	var parts []string
	for _, h := range hits {
		parts = append(parts, h.Summary.Summary)
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func (uc *OrchestratorUseCase) saveToMemory(ctx context.Context, userID, conversationID, agentName, result string) {
	summary := domain.MemorySummary{
		ID:             uuid.NewString(),
		UserID:         userID,
		ConversationID: conversationID,
		TurnFrom:       0,
		TurnTo:         0,
		Summary:        fmt.Sprintf("[%s] %s", agentName, orchTruncate(result, 1000)),
		CreatedAt:      time.Now().UTC(),
	}
	vec, err := uc.embedder.EmbedQuery(ctx, result)
	if err != nil {
		return
	}
	_ = uc.memoryVector.IndexSummary(ctx, summary, vec)
}

func orchLastUserMsg(messages []domain.AgentInputMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func orchContainsCriticIssues(result string) bool {
	lower := strings.ToLower(result)
	issueKeywords := []string{"error", "incorrect", "missing", "hallucination", "ошибк", "неточн", "пропущен", "галлюцинац", "не хватает"}
	for _, kw := range issueKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func orchTruncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}
