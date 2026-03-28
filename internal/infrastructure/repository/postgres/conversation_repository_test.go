package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func newConvRepoWithMock(t *testing.T) (*ConversationRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	return NewConversationRepository(db), mock, func() { _ = db.Close() }
}

func TestEnsureConversation_Success(t *testing.T) {
	repo, mock, done := newConvRepoWithMock(t)
	defer done()

	now := time.Now().UTC()
	mock.ExpectExec("INSERT INTO conversations").
		WithArgs("u-1", "c-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("SELECT user_id, conversation_id").
		WithArgs("u-1", "c-1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "conversation_id", "current_user_turn", "last_summary_end_turn", "created_at", "updated_at"}).
			AddRow("u-1", "c-1", 0, 0, now, now))

	conv, err := repo.EnsureConversation(context.Background(), "u-1", "c-1")
	if err != nil {
		t.Fatalf("EnsureConversation error: %v", err)
	}
	if conv.UserID != "u-1" || conv.ConversationID != "c-1" {
		t.Fatalf("unexpected conv: %+v", conv)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestNextUserTurn_Success(t *testing.T) {
	repo, mock, done := newConvRepoWithMock(t)
	defer done()

	mock.ExpectQuery("UPDATE conversations").
		WithArgs("u-1", "c-1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"current_user_turn"}).AddRow(5))

	turn, err := repo.NextUserTurn(context.Background(), "u-1", "c-1")
	if err != nil {
		t.Fatalf("NextUserTurn error: %v", err)
	}
	if turn != 5 {
		t.Fatalf("expected turn 5, got %d", turn)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestAppendMessage_Success(t *testing.T) {
	repo, mock, done := newConvRepoWithMock(t)
	defer done()

	mock.ExpectExec("INSERT INTO conversation_messages").
		WithArgs("msg-1", "u-1", "c-1", "user", "hello", nil, 1, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.AppendMessage(context.Background(), domain.ConversationMessage{
		ID:             "msg-1",
		UserID:         "u-1",
		ConversationID: "c-1",
		Role:           "user",
		Content:        "hello",
		UserTurn:       1,
		CreatedAt:      time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("AppendMessage error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestListRecentMessages_Success(t *testing.T) {
	repo, mock, done := newConvRepoWithMock(t)
	defer done()

	now := time.Now().UTC()
	mock.ExpectQuery("SELECT id, user_id, conversation_id").
		WithArgs("u-1", "c-1", 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "conversation_id", "role", "content", "tool_name", "user_turn", "created_at"}).
			AddRow("m2", "u-1", "c-1", "assistant", "reply", "", 1, now).
			AddRow("m1", "u-1", "c-1", "user", "hello", "", 1, now.Add(-time.Second)))

	msgs, err := repo.ListRecentMessages(context.Background(), "u-1", "c-1", 10)
	if err != nil {
		t.Fatalf("ListRecentMessages error: %v", err)
	}
	// Should be reversed to chronological order
	if len(msgs) != 2 || msgs[0].ID != "m1" {
		t.Fatalf("expected reversed order, got %+v", msgs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestListRecentMessages_ZeroLimit(t *testing.T) {
	repo, _, done := newConvRepoWithMock(t)
	defer done()

	msgs, err := repo.ListRecentMessages(context.Background(), "u-1", "c-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgs != nil {
		t.Fatalf("expected nil for zero limit, got %v", msgs)
	}
}

func TestListMessagesByTurnRange_Success(t *testing.T) {
	repo, mock, done := newConvRepoWithMock(t)
	defer done()

	now := time.Now().UTC()
	mock.ExpectQuery("SELECT id, user_id, conversation_id").
		WithArgs("u-1", "c-1", 1, 3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "conversation_id", "role", "content", "tool_name", "user_turn", "created_at"}).
			AddRow("m1", "u-1", "c-1", "user", "hello", "", 1, now))

	msgs, err := repo.ListMessagesByTurnRange(context.Background(), "u-1", "c-1", 1, 3)
	if err != nil {
		t.Fatalf("ListMessagesByTurnRange error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpdateLastSummaryEndTurn_Success(t *testing.T) {
	repo, mock, done := newConvRepoWithMock(t)
	defer done()

	mock.ExpectExec("UPDATE conversations").
		WithArgs("u-1", "c-1", 5, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateLastSummaryEndTurn(context.Background(), "u-1", "c-1", 5)
	if err != nil {
		t.Fatalf("UpdateLastSummaryEndTurn error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSanitizeUTF8pg(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid", "hello", "hello"},
		{"cyrillic", "Привет", "Привет"},
		{"invalid_bytes", "hello\x80world", "helloworld"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeUTF8pg(tt.input); got != tt.want {
				t.Fatalf("sanitizeUTF8pg() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNullableString(t *testing.T) {
	if nullableString("") != nil {
		t.Fatal("expected nil for empty string")
	}
	if nullableString("hello") != "hello" {
		t.Fatal("expected 'hello'")
	}
}
