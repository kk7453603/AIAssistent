package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func newMemRepoWithMock(t *testing.T) (*MemoryRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	return NewMemoryRepository(db), mock, func() { _ = db.Close() }
}

func TestCreateSummary_Success(t *testing.T) {
	repo, mock, done := newMemRepoWithMock(t)
	defer done()

	now := time.Now().UTC()
	mock.ExpectExec("INSERT INTO memory_summaries").
		WithArgs("s-1", "u-1", "c-1", 1, 5, "summary text", now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.CreateSummary(context.Background(), &domain.MemorySummary{
		ID: "s-1", UserID: "u-1", ConversationID: "c-1",
		TurnFrom: 1, TurnTo: 5, Summary: "summary text", CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateSummary error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGetLastSummaryEndTurn_Success(t *testing.T) {
	repo, mock, done := newMemRepoWithMock(t)
	defer done()

	mock.ExpectQuery("SELECT COALESCE").
		WithArgs("u-1", "c-1").
		WillReturnRows(sqlmock.NewRows([]string{"turn"}).AddRow(10))

	turn, err := repo.GetLastSummaryEndTurn(context.Background(), "u-1", "c-1")
	if err != nil {
		t.Fatalf("GetLastSummaryEndTurn error: %v", err)
	}
	if turn != 10 {
		t.Fatalf("expected turn 10, got %d", turn)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGetLastSummaryEndTurn_NoRows(t *testing.T) {
	repo, mock, done := newMemRepoWithMock(t)
	defer done()

	// COALESCE returns 0 when no rows
	mock.ExpectQuery("SELECT COALESCE").
		WithArgs("u-1", "c-1").
		WillReturnRows(sqlmock.NewRows([]string{"turn"}).AddRow(0))

	turn, err := repo.GetLastSummaryEndTurn(context.Background(), "u-1", "c-1")
	if err != nil {
		t.Fatalf("GetLastSummaryEndTurn error: %v", err)
	}
	if turn != 0 {
		t.Fatalf("expected turn 0, got %d", turn)
	}
}
