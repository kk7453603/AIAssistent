package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) CreateTask(ctx context.Context, task *domain.Task) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO tasks (id, user_id, title, details, status, due_at, created_at, updated_at, deleted_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
`, task.ID, task.UserID, task.Title, task.Details, string(task.Status), task.DueAt, task.CreatedAt, task.UpdatedAt, task.DeletedAt)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (r *TaskRepository) ListTasks(ctx context.Context, userID string, includeDeleted bool) ([]domain.Task, error) {
	query := `
SELECT id, user_id, title, details, status, due_at, created_at, updated_at, deleted_at
FROM tasks
WHERE user_id = $1
`
	if !includeDeleted {
		query += "AND deleted_at IS NULL\n"
	}
	query += "ORDER BY updated_at DESC"

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return out, nil
}

func (r *TaskRepository) GetTaskByID(ctx context.Context, userID, taskID string) (*domain.Task, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, user_id, title, details, status, due_at, created_at, updated_at, deleted_at
FROM tasks
WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
`, userID, taskID)

	task, err := scanTaskRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("task not found: id=%s", taskID)
		}
		return nil, fmt.Errorf("get task by id: %w", err)
	}
	return &task, nil
}

func (r *TaskRepository) UpdateTask(ctx context.Context, task *domain.Task) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE tasks
SET title = $3, details = $4, status = $5, due_at = $6, updated_at = $7
WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
`, task.UserID, task.ID, task.Title, task.Details, string(task.Status), task.DueAt, task.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update task rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("task not found: id=%s", task.ID)
	}
	return nil
}

func (r *TaskRepository) SoftDeleteTask(ctx context.Context, userID, taskID string) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
UPDATE tasks
SET deleted_at = $3, updated_at = $3
WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
`, userID, taskID, now)
	if err != nil {
		return fmt.Errorf("soft delete task: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("soft delete task rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("task not found: id=%s", taskID)
	}
	return nil
}

type taskScanner interface {
	Scan(dest ...interface{}) error
}

func scanTaskRow(row taskScanner) (domain.Task, error) {
	return scanTask(row)
}

func scanTask(row taskScanner) (domain.Task, error) {
	var task domain.Task
	var status string
	err := row.Scan(
		&task.ID,
		&task.UserID,
		&task.Title,
		&task.Details,
		&status,
		&task.DueAt,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.DeletedAt,
	)
	if err != nil {
		return domain.Task{}, err
	}
	task.Status = domain.TaskStatus(status)
	return task, nil
}
