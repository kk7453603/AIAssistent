package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type EventCollector struct {
	store ports.EventStore
}

func NewEventCollector(store ports.EventStore) *EventCollector {
	return &EventCollector{store: store}
}

func (c *EventCollector) RecordToolError(ctx context.Context, userID, convID, toolName, errMsg string) {
	c.record(ctx, userID, convID, "tool_error", map[string]any{"tool": toolName, "error": errMsg})
}

func (c *EventCollector) RecordEmptyRetrieval(ctx context.Context, userID, convID, query string) {
	c.record(ctx, userID, convID, "empty_retrieval", map[string]any{"query": query})
}

func (c *EventCollector) RecordFallback(ctx context.Context, userID, convID, from, to string) {
	c.record(ctx, userID, convID, "fallback", map[string]any{"from": from, "to": to})
}

func (c *EventCollector) RecordTimeout(ctx context.Context, userID, convID, model string, duration time.Duration) {
	c.record(ctx, userID, convID, "timeout", map[string]any{"model": model, "duration_ms": duration.Milliseconds()})
}

func (c *EventCollector) RecordCriticRejection(ctx context.Context, userID, convID, feedback string) {
	c.record(ctx, userID, convID, "critic_rejection", map[string]any{"feedback": feedback})
}

func (c *EventCollector) record(ctx context.Context, userID, convID, eventType string, details map[string]any) {
	if c.store == nil {
		return
	}
	event := &domain.AgentEvent{
		UserID: userID, ConversationID: convID, EventType: eventType, Details: details,
	}
	if err := c.store.Record(ctx, event); err != nil {
		slog.Warn("event_collector_record_failed", "event_type", eventType, "error", err)
	}
}
