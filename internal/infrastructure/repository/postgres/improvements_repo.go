package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type ImprovementRepository struct {
	db *sql.DB
}

func NewImprovementRepository(db *sql.DB) *ImprovementRepository {
	return &ImprovementRepository{db: db}
}

func (r *ImprovementRepository) Create(ctx context.Context, imp *domain.AgentImprovement) error {
	if imp.ID == "" {
		imp.ID = uuid.NewString()
	}
	if imp.CreatedAt.IsZero() {
		imp.CreatedAt = time.Now().UTC()
	}

	actionJSON, err := json.Marshal(imp.Action)
	if err != nil {
		return fmt.Errorf("marshal improvement action: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO agent_improvements (id, category, description, action, status, created_at, applied_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`,
		imp.ID, imp.Category, imp.Description, actionJSON, imp.Status, imp.CreatedAt, imp.AppliedAt,
	)
	if err != nil {
		return fmt.Errorf("insert agent improvement: %w", err)
	}
	return nil
}

func (r *ImprovementRepository) ListPending(ctx context.Context) ([]domain.AgentImprovement, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, category, description, action, status, created_at, applied_at
FROM agent_improvements
WHERE status = 'pending'
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, fmt.Errorf("query pending improvements: %w", err)
	}
	defer rows.Close()

	var results []domain.AgentImprovement
	for rows.Next() {
		var imp domain.AgentImprovement
		var actionRaw []byte
		if err := rows.Scan(&imp.ID, &imp.Category, &imp.Description, &actionRaw, &imp.Status, &imp.CreatedAt, &imp.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan agent improvement: %w", err)
		}
		if err := json.Unmarshal(actionRaw, &imp.Action); err != nil {
			return nil, fmt.Errorf("unmarshal improvement action: %w", err)
		}
		results = append(results, imp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error improvements: %w", err)
	}
	return results, nil
}

func (r *ImprovementRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE agent_improvements SET status = $2 WHERE id = $1
`, id, status)
	if err != nil {
		return fmt.Errorf("update improvement status: %w", err)
	}
	return nil
}

func (r *ImprovementRepository) MarkApplied(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
UPDATE agent_improvements SET status = 'applied', applied_at = $2 WHERE id = $1
`, id, now)
	if err != nil {
		return fmt.Errorf("mark improvement applied: %w", err)
	}
	return nil
}
