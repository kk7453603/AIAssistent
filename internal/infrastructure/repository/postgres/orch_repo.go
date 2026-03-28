package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// OrchestrationRepository persists multi-agent orchestration history to Postgres.
type OrchestrationRepository struct {
	db *sql.DB
}

// NewOrchestrationRepository creates a new OrchestrationRepository.
func NewOrchestrationRepository(db *sql.DB) *OrchestrationRepository {
	return &OrchestrationRepository{db: db}
}

// Create inserts a new orchestration record.
func (r *OrchestrationRepository) Create(ctx context.Context, orch *domain.Orchestration) error {
	plan := orch.Plan
	if plan == nil {
		plan = []domain.OrchestrationPlanStep{}
	}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	steps := orch.Steps
	if steps == nil {
		steps = []domain.OrchestrationStep{}
	}
	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO orchestrations (id, user_id, conversation_id, request, plan, steps, status, created_at, completed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`,
		orch.ID, orch.UserID, orch.ConversationID, orch.Request,
		planJSON, stepsJSON, orch.Status, orch.CreatedAt, orch.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert orchestration: %w", err)
	}
	return nil
}

// AddStep appends a new step to the orchestration's steps JSONB column.
func (r *OrchestrationRepository) AddStep(ctx context.Context, orchID string, step domain.OrchestrationStep) error {
	stepJSON, err := json.Marshal(step)
	if err != nil {
		return fmt.Errorf("marshal step: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
UPDATE orchestrations
SET steps = steps || $2::jsonb
WHERE id = $1
`, orchID, fmt.Sprintf("[%s]", stepJSON))
	if err != nil {
		return fmt.Errorf("add orchestration step: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for add orchestration step: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("orchestration not found: id=%s", orchID)
	}
	return nil
}

// Complete sets the orchestration status and completed_at timestamp.
func (r *OrchestrationRepository) Complete(ctx context.Context, orchID string, status string) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
UPDATE orchestrations
SET status = $2, completed_at = $3
WHERE id = $1
`, orchID, status, now)
	if err != nil {
		return fmt.Errorf("complete orchestration: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for complete orchestration: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("orchestration not found: id=%s", orchID)
	}
	return nil
}

// GetByID retrieves a single orchestration by its ID.
func (r *OrchestrationRepository) GetByID(ctx context.Context, orchID string) (*domain.Orchestration, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, user_id, conversation_id, request, plan, steps, status, created_at, completed_at
FROM orchestrations
WHERE id = $1
`, orchID)

	var orch domain.Orchestration
	var planRaw, stepsRaw []byte

	err := row.Scan(
		&orch.ID, &orch.UserID, &orch.ConversationID, &orch.Request,
		&planRaw, &stepsRaw, &orch.Status, &orch.CreatedAt, &orch.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("orchestration not found: id=%s", orchID)
		}
		return nil, fmt.Errorf("scan orchestration: %w", err)
	}

	if err := json.Unmarshal(planRaw, &orch.Plan); err != nil {
		return nil, fmt.Errorf("unmarshal plan: %w", err)
	}
	if err := json.Unmarshal(stepsRaw, &orch.Steps); err != nil {
		return nil, fmt.Errorf("unmarshal steps: %w", err)
	}
	return &orch, nil
}

// ListByUser retrieves the most recent orchestrations for a given user.
func (r *OrchestrationRepository) ListByUser(ctx context.Context, userID string, limit int) ([]domain.Orchestration, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, conversation_id, request, plan, steps, status, created_at, completed_at
FROM orchestrations
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2
`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list orchestrations by user: %w", err)
	}
	defer rows.Close()

	var result []domain.Orchestration
	for rows.Next() {
		var orch domain.Orchestration
		var planRaw, stepsRaw []byte

		if err := rows.Scan(
			&orch.ID, &orch.UserID, &orch.ConversationID, &orch.Request,
			&planRaw, &stepsRaw, &orch.Status, &orch.CreatedAt, &orch.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan orchestration row: %w", err)
		}

		if err := json.Unmarshal(planRaw, &orch.Plan); err != nil {
			return nil, fmt.Errorf("unmarshal plan: %w", err)
		}
		if err := json.Unmarshal(stepsRaw, &orch.Steps); err != nil {
			return nil, fmt.Errorf("unmarshal steps: %w", err)
		}
		result = append(result, orch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orchestration rows: %w", err)
	}
	return result, nil
}
