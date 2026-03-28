package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type SelfImproveUseCase struct {
	events       ports.EventStore
	feedback     ports.FeedbackStore
	improvements ports.ImprovementStore
	generator    ports.AnswerGenerator
	autoApply    bool
}

func NewSelfImproveUseCase(
	events ports.EventStore,
	feedback ports.FeedbackStore,
	improvements ports.ImprovementStore,
	generator ports.AnswerGenerator,
	autoApply bool,
) *SelfImproveUseCase {
	return &SelfImproveUseCase{
		events: events, feedback: feedback, improvements: improvements,
		generator: generator, autoApply: autoApply,
	}
}

func (uc *SelfImproveUseCase) Analyze(ctx context.Context, since time.Time) error {
	eventCounts, err := uc.events.CountByType(ctx, since)
	if err != nil {
		return fmt.Errorf("count events: %w", err)
	}
	ratingCounts, err := uc.feedback.CountByRating(ctx, since)
	if err != nil {
		return fmt.Errorf("count ratings: %w", err)
	}

	var comments []string
	negativeFeedback, err := uc.feedback.ListRecent(ctx, since, 20)
	if err == nil {
		for _, fb := range negativeFeedback {
			if fb.Rating == "down" && fb.Comment != "" {
				comments = append(comments, fb.Comment)
			}
		}
	}

	totalEvents := 0
	for _, c := range eventCounts {
		totalEvents += c
	}
	totalFeedback := 0
	for _, c := range ratingCounts {
		totalFeedback += c
	}

	if totalEvents == 0 && totalFeedback == 0 {
		slog.Info("self_improve_no_data", "since", since)
		return nil
	}

	prompt := buildSelfImprovePrompt(eventCounts, ratingCounts, comments)
	respText, err := uc.generator.GenerateJSONFromPrompt(ctx, prompt)
	if err != nil {
		return fmt.Errorf("generate improvements: %w", err)
	}

	improvements := parseSelfImprovements(respText)
	for i := range improvements {
		improvements[i].Status = "pending"
		if err := uc.improvements.Create(ctx, &improvements[i]); err != nil {
			slog.Warn("self_improve_save_failed", "category", improvements[i].Category, "error", err)
			continue
		}
		if uc.autoApply && domain.AutoApplyCategories[improvements[i].Category] {
			slog.Info("self_improve_auto_apply", "category", improvements[i].Category, "description", improvements[i].Description)
			_ = uc.improvements.MarkApplied(ctx, improvements[i].ID)
		}
	}

	slog.Info("self_improve_completed", "improvements", len(improvements), "events", totalEvents, "feedback", totalFeedback)
	return nil
}

func buildSelfImprovePrompt(eventCounts, ratingCounts map[string]int, comments []string) string {
	var sb strings.Builder
	sb.WriteString("Analyze these agent performance metrics and suggest specific improvements.\n\nError summary:\n")
	for t, c := range eventCounts {
		sb.WriteString(fmt.Sprintf("- %s: %d occurrences\n", t, c))
	}
	sb.WriteString("\nUser feedback:\n")
	for r, c := range ratingCounts {
		sb.WriteString(fmt.Sprintf("- %s: %d\n", r, c))
	}
	if len(comments) > 0 {
		sb.WriteString("\nNegative comments:\n")
		for _, c := range comments {
			if len(c) > 200 {
				c = c[:200]
			}
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}
	sb.WriteString("\nReturn ONLY a JSON array: [{\"category\":\"system_prompt|intent_keywords|model_routing|reindex_document|eval_case|add_document\",\"description\":\"...\",\"action\":{}}]\n")
	return sb.String()
}

func parseSelfImprovements(raw string) []domain.AgentImprovement {
	var result []domain.AgentImprovement
	if err := json.Unmarshal([]byte(raw), &result); err == nil {
		return result
	}
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		_ = json.Unmarshal([]byte(raw[start:end+1]), &result)
	}
	return result
}
