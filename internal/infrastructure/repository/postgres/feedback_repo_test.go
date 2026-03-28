package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func newFeedbackRepoWithMock(t *testing.T) (*FeedbackRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	return NewFeedbackRepository(db), mock, func() { _ = db.Close() }
}

func TestFeedbackCreate_AutoID(t *testing.T) {
	repo, mock, done := newFeedbackRepoWithMock(t)
	defer done()

	mock.ExpectExec("INSERT INTO agent_feedback").
		WithArgs(sqlmock.AnyArg(), "u-1", "c-1", "msg-1", "positive", "great", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	fb := &domain.AgentFeedback{
		UserID: "u-1", ConversationID: "c-1", MessageID: "msg-1",
		Rating: "positive", Comment: "great",
	}
	if err := repo.Create(context.Background(), fb); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if fb.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestFeedbackListRecent_Success(t *testing.T) {
	repo, mock, done := newFeedbackRepoWithMock(t)
	defer done()

	now := time.Now().UTC()
	mock.ExpectQuery("SELECT id, user_id, conversation_id, message_id, rating, comment, created_at").
		WithArgs(sqlmock.AnyArg(), 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "conversation_id", "message_id", "rating", "comment", "created_at"}).
			AddRow("fb-1", "u-1", "c-1", "msg-1", "positive", "great", now))

	results, err := repo.ListRecent(context.Background(), now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("ListRecent error: %v", err)
	}
	if len(results) != 1 || results[0].Rating != "positive" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestFeedbackCountByRating_Success(t *testing.T) {
	repo, mock, done := newFeedbackRepoWithMock(t)
	defer done()

	mock.ExpectQuery("SELECT rating, COUNT").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"rating", "cnt"}).
			AddRow("positive", 10).
			AddRow("negative", 3))

	counts, err := repo.CountByRating(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("CountByRating error: %v", err)
	}
	if counts["positive"] != 10 || counts["negative"] != 3 {
		t.Fatalf("unexpected counts: %v", counts)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
