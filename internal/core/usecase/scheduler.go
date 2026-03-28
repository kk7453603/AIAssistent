package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

const (
	schedulerWebhookTimeout   = 10 * time.Second
	schedulerConditionTimeout  = 30 * time.Second
	scheduleMaxResultLen       = 1000
)

// SchedulerUseCase handles cron-based task execution with conditional logic and webhooks.
type SchedulerUseCase struct {
	store     ports.ScheduleStore
	agentChat ports.AgentChatService
	generator ports.AnswerGenerator
	parser    cron.Parser
}

// NewSchedulerUseCase constructs a SchedulerUseCase with a standard cron parser.
func NewSchedulerUseCase(
	store ports.ScheduleStore,
	agentChat ports.AgentChatService,
	generator ports.AnswerGenerator,
) *SchedulerUseCase {
	return &SchedulerUseCase{
		store:     store,
		agentChat: agentChat,
		generator: generator,
		parser: cron.NewParser(
			cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		),
	}
}

// Tick checks all enabled scheduled tasks and fires those that are due.
// It should be called periodically (e.g., every minute).
func (s *SchedulerUseCase) Tick(ctx context.Context) {
	tasks, err := s.store.ListEnabled(ctx)
	if err != nil {
		log.Printf("[scheduler] ListEnabled error: %v", err)
		return
	}

	now := time.Now()
	for _, task := range tasks {
		if s.isDue(task, now) {
			taskCopy := task
			go func() {
				execCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				s.executeTask(execCtx, taskCopy)
			}()
		}
	}
}

// isDue returns true when the next cron firing after lastRun is before or equal to now.
func (s *SchedulerUseCase) isDue(task domain.ScheduledTask, now time.Time) bool {
	schedule, err := s.parser.Parse(task.CronExpr)
	if err != nil {
		log.Printf("[scheduler] invalid cron expr for task %s: %v", task.ID, err)
		return false
	}

	var lastRun time.Time
	if task.LastRunAt != nil {
		lastRun = *task.LastRunAt
	} else {
		// Never run: treat epoch as baseline so tasks fire immediately on first tick.
		lastRun = time.Time{}
	}

	next := schedule.Next(lastRun)
	return !next.After(now)
}

// executeTask optionally checks a condition, runs the agent prompt, records the result, and sends a webhook.
func (s *SchedulerUseCase) executeTask(ctx context.Context, task domain.ScheduledTask) {
	// Conditional guard: skip execution when condition evaluates to false.
	if task.Condition != "" {
		condCtx, cancel := context.WithTimeout(ctx, schedulerConditionTimeout)
		ok, err := s.evaluateCondition(condCtx, task.Condition)
		cancel()
		if err != nil {
			log.Printf("[scheduler] condition check error for task %s: %v", task.ID, err)
			if recordErr := s.store.RecordRun(ctx, task.ID, fmt.Sprintf("condition error: %v", err), "error"); recordErr != nil {
				log.Printf("[scheduler] RecordRun error: %v", recordErr)
			}
			return
		}
		if !ok {
			log.Printf("[scheduler] condition false, skipping task %s", task.ID)
			if recordErr := s.store.RecordRun(ctx, task.ID, "condition not met", "skipped"); recordErr != nil {
				log.Printf("[scheduler] RecordRun error: %v", recordErr)
			}
			return
		}
	}

	// Execute the prompt via agent.
	req := domain.AgentChatRequest{
		UserID: task.UserID,
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: task.Prompt},
		},
	}

	result, err := s.agentChat.Complete(ctx, req, nil)
	var runResult string
	var runStatus string
	if err != nil {
		log.Printf("[scheduler] agent complete error for task %s: %v", task.ID, err)
		runResult = fmt.Sprintf("error: %v", err)
		runStatus = "error"
	} else {
		runResult = truncateScheduleResult(result.Answer, scheduleMaxResultLen)
		runStatus = "success"
	}

	// Persist run record.
	if recordErr := s.store.RecordRun(ctx, task.ID, runResult, runStatus); recordErr != nil {
		log.Printf("[scheduler] RecordRun error for task %s: %v", task.ID, recordErr)
	}

	// Best-effort webhook delivery.
	if task.WebhookURL != "" {
		s.sendWebhook(task, runResult)
	}
}

// evaluateCondition asks the LLM to evaluate the condition and returns true for a "yes" answer.
func (s *SchedulerUseCase) evaluateCondition(ctx context.Context, condition string) (bool, error) {
	prompt := fmt.Sprintf(
		"Evaluate the following condition and answer strictly with 'yes' or 'no' (lowercase, no punctuation).\nCondition: %s",
		condition,
	)
	answer, err := s.generator.GenerateFromPrompt(ctx, prompt)
	if err != nil {
		return false, err
	}
	normalized := strings.TrimSpace(strings.ToLower(answer))
	return strings.HasPrefix(normalized, "yes"), nil
}

// sendWebhook POSTs a JSON payload to the task's webhook URL; errors are logged only.
func (s *SchedulerUseCase) sendWebhook(task domain.ScheduledTask, result string) {
	payload := map[string]string{
		"task_id": task.ID,
		"user_id": task.UserID,
		"result":  result,
		"status":  "success",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[scheduler] webhook marshal error for task %s: %v", task.ID, err)
		return
	}

	client := &http.Client{Timeout: schedulerWebhookTimeout}
	resp, err := client.Post(task.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[scheduler] webhook delivery error for task %s: %v", task.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[scheduler] webhook non-2xx for task %s: %d", task.ID, resp.StatusCode)
	}
}

// ParseNaturalScheduleResult holds the parsed cron expression and extracted prompt.
type ParseNaturalScheduleResult struct {
	CronExpr string `json:"cron"`
	Prompt   string `json:"prompt"`
}

// ParseNaturalSchedule uses the LLM to convert a natural language schedule description
// into a cron expression and an execution prompt.
func (s *SchedulerUseCase) ParseNaturalSchedule(ctx context.Context, input string) (*ParseNaturalScheduleResult, error) {
	prompt := fmt.Sprintf(`Convert the following natural language schedule description into a JSON object with two fields:
- "cron": a valid 5-field cron expression (minute hour dom month dow)
- "prompt": the task to execute on that schedule

Return only valid JSON, no markdown, no explanation.

Input: %s`, input)

	raw, err := s.generator.GenerateJSONFromPrompt(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM parse error: %w", err)
	}

	var result ParseNaturalScheduleResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("JSON unmarshal error: %w (raw: %s)", err, raw)
	}

	if result.CronExpr == "" {
		return nil, fmt.Errorf("LLM returned empty cron expression")
	}

	// Validate the cron expression.
	if _, err := s.parser.Parse(result.CronExpr); err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", result.CronExpr, err)
	}

	return &result, nil
}

// truncateScheduleResult truncates s to maxLen characters, appending "..." if truncated.
func truncateScheduleResult(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
