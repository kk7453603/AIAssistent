package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func TestTaskRepositoryListTasksFiltersDeletedByDefault(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewTaskRepository(db)
	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "details", "status", "due_at", "created_at", "updated_at", "deleted_at"}).
		AddRow("t-1", "u-1", "title", "", string(domain.TaskStatusOpen), nil, time.Now(), time.Now(), nil)

	mock.ExpectQuery("FROM tasks").
		WithArgs("u-1").
		WillReturnRows(rows)

	tasks, err := repo.ListTasks(context.Background(), "u-1", false)
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestTaskRepositorySoftDeleteReturnsErrorWhenNoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewTaskRepository(db)
	mock.ExpectExec("UPDATE tasks").
		WithArgs("u-1", "missing", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.SoftDeleteTask(context.Background(), "u-1", "missing")
	if err == nil {
		t.Fatalf("expected error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

