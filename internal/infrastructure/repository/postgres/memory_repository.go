package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type MemoryRepository struct {
	db *sql.DB
}

func NewMemoryRepository(db *sql.DB) *MemoryRepository {
	return &MemoryRepository{db: db}
}

func (r *MemoryRepository) CreateSummary(ctx context.Context, summary *domain.MemorySummary) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO memory_summaries (id, user_id, conversation_id, turn_from, turn_to, summary, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)
`, summary.ID, summary.UserID, summary.ConversationID, summary.TurnFrom, summary.TurnTo, summary.Summary, summary.CreatedAt)
	if err != nil {
		return fmt.Errorf("create memory summary: %w", err)
	}
	return nil
}

func (r *MemoryRepository) GetLastSummaryEndTurn(ctx context.Context, userID, conversationID string) (int, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT COALESCE(MAX(turn_to), 0)
FROM memory_summaries
WHERE user_id = $1 AND conversation_id = $2
`, userID, conversationID)

	var turn int
	if err := row.Scan(&turn); err != nil {
		return 0, fmt.Errorf("get last summary end turn: %w", err)
	}
	return turn, nil
}
