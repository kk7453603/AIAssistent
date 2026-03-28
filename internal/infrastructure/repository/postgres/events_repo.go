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

type EventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) Record(ctx context.Context, event *domain.AgentEvent) error {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	detailsJSON, err := json.Marshal(event.Details)
	if err != nil {
		return fmt.Errorf("marshal event details: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO agent_events (id, user_id, conversation_id, event_type, details, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
`,
		event.ID, event.UserID, event.ConversationID, event.EventType, detailsJSON, event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert agent event: %w", err)
	}
	return nil
}

func (r *EventRepository) ListByType(ctx context.Context, eventType string, since time.Time, limit int) ([]domain.AgentEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, conversation_id, event_type, details, created_at
FROM agent_events
WHERE event_type = $1 AND created_at >= $2
ORDER BY created_at DESC
LIMIT $3
`, eventType, since, limit)
	if err != nil {
		return nil, fmt.Errorf("query agent events by type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []domain.AgentEvent
	for rows.Next() {
		var e domain.AgentEvent
		var detailsRaw []byte
		if err := rows.Scan(&e.ID, &e.UserID, &e.ConversationID, &e.EventType, &detailsRaw, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan agent event: %w", err)
		}
		if err := json.Unmarshal(detailsRaw, &e.Details); err != nil {
			return nil, fmt.Errorf("unmarshal event details: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error agent events: %w", err)
	}
	return events, nil
}

func (r *EventRepository) CountByType(ctx context.Context, since time.Time) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT event_type, COUNT(*) AS cnt
FROM agent_events
WHERE created_at >= $1
GROUP BY event_type
`, since)
	if err != nil {
		return nil, fmt.Errorf("count agent events by type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int)
	for rows.Next() {
		var eventType string
		var cnt int
		if err := rows.Scan(&eventType, &cnt); err != nil {
			return nil, fmt.Errorf("scan event type count: %w", err)
		}
		counts[eventType] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error event count: %w", err)
	}
	return counts, nil
}
