package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type FeedbackRepository struct {
	db *sql.DB
}

func NewFeedbackRepository(db *sql.DB) *FeedbackRepository {
	return &FeedbackRepository{db: db}
}

func (r *FeedbackRepository) Create(ctx context.Context, fb *domain.AgentFeedback) error {
	if fb.ID == "" {
		fb.ID = uuid.NewString()
	}
	if fb.CreatedAt.IsZero() {
		fb.CreatedAt = time.Now().UTC()
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO agent_feedback (id, user_id, conversation_id, message_id, rating, comment, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`,
		fb.ID, fb.UserID, fb.ConversationID, fb.MessageID, fb.Rating, fb.Comment, fb.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert agent feedback: %w", err)
	}
	return nil
}

func (r *FeedbackRepository) ListRecent(ctx context.Context, since time.Time, limit int) ([]domain.AgentFeedback, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, conversation_id, message_id, rating, comment, created_at
FROM agent_feedback
WHERE created_at >= $1
ORDER BY created_at DESC
LIMIT $2
`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent feedback: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []domain.AgentFeedback
	for rows.Next() {
		var fb domain.AgentFeedback
		if err := rows.Scan(&fb.ID, &fb.UserID, &fb.ConversationID, &fb.MessageID, &fb.Rating, &fb.Comment, &fb.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan agent feedback: %w", err)
		}
		results = append(results, fb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error feedback: %w", err)
	}
	return results, nil
}

func (r *FeedbackRepository) CountByRating(ctx context.Context, since time.Time) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT rating, COUNT(*) AS cnt
FROM agent_feedback
WHERE created_at >= $1
GROUP BY rating
`, since)
	if err != nil {
		return nil, fmt.Errorf("count feedback by rating: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int)
	for rows.Next() {
		var rating string
		var cnt int
		if err := rows.Scan(&rating, &cnt); err != nil {
			return nil, fmt.Errorf("scan rating count: %w", err)
		}
		counts[rating] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error rating count: %w", err)
	}
	return counts, nil
}
