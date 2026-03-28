package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type ScheduleRepository struct {
	db *sql.DB
}

func NewScheduleRepository(db *sql.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

func (r *ScheduleRepository) Create(ctx context.Context, task *domain.ScheduledTask) error {
	task.ID = uuid.NewString()
	now := time.Now().UTC()
	task.CreatedAt = now
	task.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
INSERT INTO scheduled_tasks (
	id, user_id, cron_expr, prompt, condition, webhook_url, enabled,
	last_run_at, last_result, last_status, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
`,
		task.ID, task.UserID, task.CronExpr, task.Prompt, task.Condition, task.WebhookURL, task.Enabled,
		nullTime(task.LastRunAt), task.LastResult, task.LastStatus, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert scheduled_task: %w", err)
	}
	return nil
}

func (r *ScheduleRepository) ListByUser(ctx context.Context, userID string) ([]domain.ScheduledTask, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, cron_expr, prompt, condition, webhook_url, enabled,
	last_run_at, last_result, last_status, created_at, updated_at
FROM scheduled_tasks
WHERE user_id = $1
ORDER BY created_at DESC
`, userID)
	if err != nil {
		return nil, fmt.Errorf("list scheduled_tasks by user: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanScheduledTasks(rows)
}

func (r *ScheduleRepository) ListEnabled(ctx context.Context) ([]domain.ScheduledTask, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, cron_expr, prompt, condition, webhook_url, enabled,
	last_run_at, last_result, last_status, created_at, updated_at
FROM scheduled_tasks
WHERE enabled = true
`)
	if err != nil {
		return nil, fmt.Errorf("list enabled scheduled_tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanScheduledTasks(rows)
}

func (r *ScheduleRepository) GetByID(ctx context.Context, id string) (*domain.ScheduledTask, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, user_id, cron_expr, prompt, condition, webhook_url, enabled,
	last_run_at, last_result, last_status, created_at, updated_at
FROM scheduled_tasks
WHERE id = $1
`, id)

	task, err := scanScheduledTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("scheduled_task not found: id=%s", id)
		}
		return nil, fmt.Errorf("get scheduled_task by id: %w", err)
	}
	return task, nil
}

func (r *ScheduleRepository) Update(ctx context.Context, task *domain.ScheduledTask) error {
	task.UpdatedAt = time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
UPDATE scheduled_tasks
SET cron_expr = $2, prompt = $3, condition = $4, webhook_url = $5, enabled = $6, updated_at = $7
WHERE id = $1
`,
		task.ID, task.CronExpr, task.Prompt, task.Condition, task.WebhookURL, task.Enabled, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update scheduled_task: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for update scheduled_task: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("scheduled_task not found: id=%s", task.ID)
	}
	return nil
}

func (r *ScheduleRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM scheduled_tasks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete scheduled_task: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for delete scheduled_task: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("scheduled_task not found: id=%s", id)
	}
	return nil
}

func (r *ScheduleRepository) RecordRun(ctx context.Context, id string, result string, status string) error {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE scheduled_tasks
SET last_run_at = $2, last_result = $3, last_status = $4, updated_at = $5
WHERE id = $1
`, id, now, result, status, now)
	if err != nil {
		return fmt.Errorf("record run for scheduled_task: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for record run scheduled_task: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("scheduled_task not found: id=%s", id)
	}
	return nil
}

// scanScheduledTask scans a single row into a ScheduledTask.
func scanScheduledTask(row *sql.Row) (*domain.ScheduledTask, error) {
	var t domain.ScheduledTask
	var lastRunAt sql.NullTime
	var condition sql.NullString
	var webhookURL sql.NullString
	var lastResult sql.NullString
	var lastStatus sql.NullString

	err := row.Scan(
		&t.ID, &t.UserID, &t.CronExpr, &t.Prompt,
		&condition, &webhookURL, &t.Enabled,
		&lastRunAt, &lastResult, &lastStatus,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if lastRunAt.Valid {
		t.LastRunAt = &lastRunAt.Time
	}
	t.Condition = condition.String
	t.WebhookURL = webhookURL.String
	t.LastResult = lastResult.String
	t.LastStatus = lastStatus.String
	return &t, nil
}

// scanScheduledTasks scans multiple rows into a slice of ScheduledTask.
func scanScheduledTasks(rows *sql.Rows) ([]domain.ScheduledTask, error) {
	var tasks []domain.ScheduledTask
	for rows.Next() {
		var t domain.ScheduledTask
		var lastRunAt sql.NullTime
		var condition sql.NullString
		var webhookURL sql.NullString
		var lastResult sql.NullString
		var lastStatus sql.NullString

		if err := rows.Scan(
			&t.ID, &t.UserID, &t.CronExpr, &t.Prompt,
			&condition, &webhookURL, &t.Enabled,
			&lastRunAt, &lastResult, &lastStatus,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan scheduled_task row: %w", err)
		}

		if lastRunAt.Valid {
			t.LastRunAt = &lastRunAt.Time
		}
		t.Condition = condition.String
		t.WebhookURL = webhookURL.String
		t.LastResult = lastResult.String
		t.LastStatus = lastStatus.String
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled_task rows: %w", err)
	}
	return tasks, nil
}

// nullTime converts a *time.Time to sql.NullTime.
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
