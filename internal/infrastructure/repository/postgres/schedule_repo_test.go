package postgres

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func newSchedRepoWithMock(t *testing.T) (*ScheduleRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	return NewScheduleRepository(db), mock, func() { _ = db.Close() }
}

func scheduleColumns() []string {
	return []string{"id", "user_id", "cron_expr", "prompt", "condition", "webhook_url", "enabled", "last_run_at", "last_result", "last_status", "created_at", "updated_at"}
}

func TestScheduleCreate_Success(t *testing.T) {
	repo, mock, done := newSchedRepoWithMock(t)
	defer done()

	mock.ExpectExec("INSERT INTO scheduled_tasks").
		WithArgs(sqlmock.AnyArg(), "u-1", "0 9 * * *", "do stuff", "", "", true, sqlmock.AnyArg(), "", "", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	task := &domain.ScheduledTask{
		UserID:   "u-1",
		CronExpr: "0 9 * * *",
		Prompt:   "do stuff",
		Enabled:  true,
	}
	if err := repo.Create(context.Background(), task); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestScheduleListByUser_Success(t *testing.T) {
	repo, mock, done := newSchedRepoWithMock(t)
	defer done()

	now := time.Now().UTC()
	mock.ExpectQuery("SELECT id, user_id, cron_expr").
		WithArgs("u-1").
		WillReturnRows(sqlmock.NewRows(scheduleColumns()).
			AddRow("s-1", "u-1", "0 9 * * *", "prompt", sql.NullString{}, sql.NullString{}, true, sql.NullTime{}, sql.NullString{}, sql.NullString{}, now, now))

	tasks, err := repo.ListByUser(context.Background(), "u-1")
	if err != nil {
		t.Fatalf("ListByUser error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "s-1" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestScheduleListEnabled_Success(t *testing.T) {
	repo, mock, done := newSchedRepoWithMock(t)
	defer done()

	now := time.Now().UTC()
	mock.ExpectQuery("SELECT id, user_id, cron_expr").
		WillReturnRows(sqlmock.NewRows(scheduleColumns()).
			AddRow("s-1", "u-1", "0 9 * * *", "prompt", sql.NullString{}, sql.NullString{}, true, sql.NullTime{}, sql.NullString{}, sql.NullString{}, now, now))

	tasks, err := repo.ListEnabled(context.Background())
	if err != nil {
		t.Fatalf("ListEnabled error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestScheduleGetByID_Success(t *testing.T) {
	repo, mock, done := newSchedRepoWithMock(t)
	defer done()

	now := time.Now().UTC()
	mock.ExpectQuery("SELECT id, user_id, cron_expr").
		WithArgs("s-1").
		WillReturnRows(sqlmock.NewRows(scheduleColumns()).
			AddRow("s-1", "u-1", "0 9 * * *", "prompt", sql.NullString{}, sql.NullString{}, true, sql.NullTime{}, sql.NullString{}, sql.NullString{}, now, now))

	task, err := repo.GetByID(context.Background(), "s-1")
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if task.ID != "s-1" {
		t.Fatalf("expected ID 's-1', got %q", task.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestScheduleGetByID_NotFound(t *testing.T) {
	repo, mock, done := newSchedRepoWithMock(t)
	defer done()

	mock.ExpectQuery("SELECT id, user_id, cron_expr").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetByID(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestScheduleUpdate_Success(t *testing.T) {
	repo, mock, done := newSchedRepoWithMock(t)
	defer done()

	mock.ExpectExec("UPDATE scheduled_tasks").
		WithArgs("s-1", "0 10 * * *", "new prompt", "", "", true, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	task := &domain.ScheduledTask{
		ID: "s-1", CronExpr: "0 10 * * *", Prompt: "new prompt", Enabled: true,
	}
	if err := repo.Update(context.Background(), task); err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestScheduleUpdate_NotFound(t *testing.T) {
	repo, mock, done := newSchedRepoWithMock(t)
	defer done()

	mock.ExpectExec("UPDATE scheduled_tasks").
		WithArgs("missing", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	task := &domain.ScheduledTask{ID: "missing"}
	err := repo.Update(context.Background(), task)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestScheduleDelete_Success(t *testing.T) {
	repo, mock, done := newSchedRepoWithMock(t)
	defer done()

	mock.ExpectExec("DELETE FROM scheduled_tasks").
		WithArgs("s-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Delete(context.Background(), "s-1"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestScheduleDelete_NotFound(t *testing.T) {
	repo, mock, done := newSchedRepoWithMock(t)
	defer done()

	mock.ExpectExec("DELETE FROM scheduled_tasks").
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Delete(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestScheduleRecordRun_Success(t *testing.T) {
	repo, mock, done := newSchedRepoWithMock(t)
	defer done()

	mock.ExpectExec("UPDATE scheduled_tasks").
		WithArgs("s-1", sqlmock.AnyArg(), "result text", "ok", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.RecordRun(context.Background(), "s-1", "result text", "ok"); err != nil {
		t.Fatalf("RecordRun error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestNullTime(t *testing.T) {
	nt := nullTime(nil)
	if nt.Valid {
		t.Fatal("expected invalid NullTime for nil")
	}

	now := time.Now().UTC()
	nt = nullTime(&now)
	if !nt.Valid {
		t.Fatal("expected valid NullTime for non-nil")
	}
	if nt.Time != now {
		t.Fatalf("expected %v, got %v", now, nt.Time)
	}
}
