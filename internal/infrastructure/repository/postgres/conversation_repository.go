package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type ConversationRepository struct {
	db *sql.DB
}

func NewConversationRepository(db *sql.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) EnsureConversation(ctx context.Context, userID, conversationID string) (*domain.Conversation, error) {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
INSERT INTO conversations (user_id, conversation_id, current_user_turn, last_summary_end_turn, created_at, updated_at)
VALUES ($1, $2, 0, 0, $3, $3)
ON CONFLICT (user_id, conversation_id) DO NOTHING
`, userID, conversationID, now)
	if err != nil {
		return nil, fmt.Errorf("ensure conversation insert: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
SELECT user_id, conversation_id, current_user_turn, last_summary_end_turn, created_at, updated_at
FROM conversations
WHERE user_id = $1 AND conversation_id = $2
`, userID, conversationID)

	var conv domain.Conversation
	if err := row.Scan(
		&conv.UserID,
		&conv.ConversationID,
		&conv.CurrentUserTurn,
		&conv.LastSummaryEndTurn,
		&conv.CreatedAt,
		&conv.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("ensure conversation select: %w", err)
	}
	return &conv, nil
}

func (r *ConversationRepository) NextUserTurn(ctx context.Context, userID, conversationID string) (int, error) {
	row := r.db.QueryRowContext(ctx, `
UPDATE conversations
SET current_user_turn = current_user_turn + 1, updated_at = $3
WHERE user_id = $1 AND conversation_id = $2
RETURNING current_user_turn
`, userID, conversationID, time.Now().UTC())

	var currentTurn int
	if err := row.Scan(&currentTurn); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if _, ensureErr := r.EnsureConversation(ctx, userID, conversationID); ensureErr != nil {
				return 0, ensureErr
			}
			return r.NextUserTurn(ctx, userID, conversationID)
		}
		return 0, fmt.Errorf("next user turn: %w", err)
	}
	return currentTurn, nil
}

func (r *ConversationRepository) AppendMessage(ctx context.Context, message domain.ConversationMessage) error {
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO conversation_messages (id, user_id, conversation_id, role, content, tool_name, user_turn, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
`, message.ID, message.UserID, message.ConversationID, message.Role, message.Content, nullableString(message.ToolName), message.UserTurn, message.CreatedAt)
	if err != nil {
		return fmt.Errorf("append message: %w", err)
	}
	return nil
}

func (r *ConversationRepository) ListRecentMessages(ctx context.Context, userID, conversationID string, limit int) ([]domain.ConversationMessage, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, conversation_id, role, content, COALESCE(tool_name, ''), user_turn, created_at
FROM conversation_messages
WHERE user_id = $1 AND conversation_id = $2
ORDER BY created_at DESC
LIMIT $3
`, userID, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent messages: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ConversationMessage, 0, limit)
	for rows.Next() {
		var msg domain.ConversationMessage
		if err := rows.Scan(
			&msg.ID,
			&msg.UserID,
			&msg.ConversationID,
			&msg.Role,
			&msg.Content,
			&msg.ToolName,
			&msg.UserTurn,
			&msg.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan recent message: %w", err)
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent messages: %w", err)
	}

	// Returned in descending order from SQL; reverse to keep chronological order.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (r *ConversationRepository) ListMessagesByTurnRange(ctx context.Context, userID, conversationID string, turnFrom, turnTo int) ([]domain.ConversationMessage, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, conversation_id, role, content, COALESCE(tool_name, ''), user_turn, created_at
FROM conversation_messages
WHERE user_id = $1 AND conversation_id = $2 AND user_turn >= $3 AND user_turn <= $4
ORDER BY user_turn ASC, created_at ASC
`, userID, conversationID, turnFrom, turnTo)
	if err != nil {
		return nil, fmt.Errorf("list messages by turn range: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ConversationMessage, 0)
	for rows.Next() {
		var msg domain.ConversationMessage
		if err := rows.Scan(
			&msg.ID,
			&msg.UserID,
			&msg.ConversationID,
			&msg.Role,
			&msg.Content,
			&msg.ToolName,
			&msg.UserTurn,
			&msg.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan range message: %w", err)
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate range messages: %w", err)
	}
	return out, nil
}

func (r *ConversationRepository) UpdateLastSummaryEndTurn(ctx context.Context, userID, conversationID string, turn int) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE conversations
SET last_summary_end_turn = GREATEST(last_summary_end_turn, $3), updated_at = $4
WHERE user_id = $1 AND conversation_id = $2
`, userID, conversationID, turn, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update last summary turn: %w", err)
	}
	return nil
}

func nullableString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}
