package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type eventStoreFake struct {
	events []domain.AgentEvent
}

func (f *eventStoreFake) Record(_ context.Context, event *domain.AgentEvent) error {
	f.events = append(f.events, *event)
	return nil
}

func (f *eventStoreFake) ListByType(context.Context, string, time.Time, int) ([]domain.AgentEvent, error) {
	return nil, nil
}

func (f *eventStoreFake) CountByType(context.Context, time.Time) (map[string]int, error) {
	return nil, nil
}

func TestEventCollector_RecordToolError(t *testing.T) {
	store := &eventStoreFake{}
	collector := NewEventCollector(store)
	collector.RecordToolError(context.Background(), "user1", "conv1", "web_search", "timeout")
	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	if store.events[0].EventType != "tool_error" {
		t.Errorf("event_type = %q, want %q", store.events[0].EventType, "tool_error")
	}
}

func TestEventCollector_RecordEmptyRetrieval(t *testing.T) {
	store := &eventStoreFake{}
	collector := NewEventCollector(store)
	collector.RecordEmptyRetrieval(context.Background(), "user1", "conv1", "what is Go?")
	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	if store.events[0].EventType != "empty_retrieval" {
		t.Errorf("event_type = %q", store.events[0].EventType)
	}
}

func TestEventCollector_NilStore(t *testing.T) {
	collector := NewEventCollector(nil)
	// Should not panic.
	collector.RecordToolError(context.Background(), "user1", "conv1", "tool", "err")
}
