package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func newImpRepoWithMock(t *testing.T) (*ImprovementRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	return NewImprovementRepository(db), mock, func() { _ = db.Close() }
}

func TestImprovementCreate_AutoID(t *testing.T) {
	repo, mock, done := newImpRepoWithMock(t)
	defer done()

	mock.ExpectExec("INSERT INTO agent_improvements").
		WithArgs(sqlmock.AnyArg(), "prompt", "improve X", sqlmock.AnyArg(), "pending", sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))

	imp := &domain.AgentImprovement{
		Category:    "prompt",
		Description: "improve X",
		Action:      map[string]any{"type": "update"},
		Status:      "pending",
	}
	if err := repo.Create(context.Background(), imp); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if imp.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestImprovementListPending_Success(t *testing.T) {
	repo, mock, done := newImpRepoWithMock(t)
	defer done()

	now := time.Now().UTC()
	actionJSON, _ := json.Marshal(map[string]any{"type": "update"})

	mock.ExpectQuery("SELECT id, category, description, action, status, created_at, applied_at").
		WillReturnRows(sqlmock.NewRows([]string{"id", "category", "description", "action", "status", "created_at", "applied_at"}).
			AddRow("imp-1", "prompt", "improve", actionJSON, "pending", now, nil))

	results, err := repo.ListPending(context.Background())
	if err != nil {
		t.Fatalf("ListPending error: %v", err)
	}
	if len(results) != 1 || results[0].ID != "imp-1" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestImprovementUpdateStatus_Success(t *testing.T) {
	repo, mock, done := newImpRepoWithMock(t)
	defer done()

	mock.ExpectExec("UPDATE agent_improvements SET status").
		WithArgs("imp-1", "applied").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdateStatus(context.Background(), "imp-1", "applied"); err != nil {
		t.Fatalf("UpdateStatus error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestImprovementMarkApplied_Success(t *testing.T) {
	repo, mock, done := newImpRepoWithMock(t)
	defer done()

	mock.ExpectExec("UPDATE agent_improvements SET status").
		WithArgs("imp-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.MarkApplied(context.Background(), "imp-1"); err != nil {
		t.Fatalf("MarkApplied error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
