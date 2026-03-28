package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func newEventRepoWithMock(t *testing.T) (*EventRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	return NewEventRepository(db), mock, func() { _ = db.Close() }
}

func TestEventRecord_AutoID(t *testing.T) {
	repo, mock, done := newEventRepoWithMock(t)
	defer done()

	mock.ExpectExec("INSERT INTO agent_events").
		WithArgs(sqlmock.AnyArg(), "u-1", "c-1", "tool_call", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	event := &domain.AgentEvent{
		UserID: "u-1", ConversationID: "c-1", EventType: "tool_call",
		Details: map[string]any{"tool": "web_search"},
	}
	if err := repo.Record(context.Background(), event); err != nil {
		t.Fatalf("Record error: %v", err)
	}
	if event.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEventListByType_Success(t *testing.T) {
	repo, mock, done := newEventRepoWithMock(t)
	defer done()

	now := time.Now().UTC()
	detailsJSON, _ := json.Marshal(map[string]any{"key": "value"})

	mock.ExpectQuery("SELECT id, user_id, conversation_id, event_type, details, created_at").
		WithArgs("tool_call", sqlmock.AnyArg(), 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "conversation_id", "event_type", "details", "created_at"}).
			AddRow("e-1", "u-1", "c-1", "tool_call", detailsJSON, now))

	events, err := repo.ListByType(context.Background(), "tool_call", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("ListByType error: %v", err)
	}
	if len(events) != 1 || events[0].ID != "e-1" {
		t.Fatalf("unexpected events: %+v", events)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEventCountByType_Success(t *testing.T) {
	repo, mock, done := newEventRepoWithMock(t)
	defer done()

	mock.ExpectQuery("SELECT event_type, COUNT").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "cnt"}).
			AddRow("tool_call", 5).
			AddRow("error", 2))

	counts, err := repo.CountByType(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("CountByType error: %v", err)
	}
	if counts["tool_call"] != 5 || counts["error"] != 2 {
		t.Fatalf("unexpected counts: %v", counts)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
