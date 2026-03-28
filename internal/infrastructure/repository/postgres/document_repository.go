package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type DocumentRepository struct {
	db *sql.DB
}

func NewDocumentRepository(db *sql.DB) *DocumentRepository {
	return &DocumentRepository{db: db}
}

func OpenDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return db, nil
}

func (r *DocumentRepository) EnsureSchema(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Serialize bootstrap DDL across api/worker startups.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(2026021001)); err != nil {
		return fmt.Errorf("acquire schema lock: %w", err)
	}

	const query = `
CREATE TABLE IF NOT EXISTS documents (
	id TEXT PRIMARY KEY,
	filename TEXT NOT NULL,
	mime_type TEXT NOT NULL,
	storage_path TEXT NOT NULL,
	category TEXT,
	subcategory TEXT,
	tags JSONB NOT NULL DEFAULT '[]'::jsonb,
	confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
	summary TEXT,
	status TEXT NOT NULL,
	error_message TEXT,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at DESC);

CREATE TABLE IF NOT EXISTS conversations (
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	current_user_turn INTEGER NOT NULL DEFAULT 0,
	last_summary_end_turn INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (user_id, conversation_id)
);

CREATE TABLE IF NOT EXISTS conversation_messages (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	tool_name TEXT,
	user_turn INTEGER NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_conversation_messages_user_conv_created
	ON conversation_messages(user_id, conversation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_user_conv_turn
	ON conversation_messages(user_id, conversation_id, user_turn, created_at ASC);

CREATE TABLE IF NOT EXISTS tasks (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	title TEXT NOT NULL,
	details TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	due_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tasks_user_status_deleted_updated
	ON tasks(user_id, status, deleted_at, updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_summaries (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	turn_from INTEGER NOT NULL,
	turn_to INTEGER NOT NULL,
	summary TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_summaries_user_conv_created
	ON memory_summaries(user_id, conversation_id, created_at DESC);

ALTER TABLE documents ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN IF NOT EXISTS headers JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS path TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS orchestrations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    request TEXT NOT NULL,
    plan JSONB NOT NULL DEFAULT '[]'::jsonb,
    steps JSONB NOT NULL DEFAULT '[]'::jsonb,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_orchestrations_user_conv
    ON orchestrations(user_id, conversation_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_events (
	id TEXT PRIMARY KEY,
	user_id TEXT,
	conversation_id TEXT,
	event_type TEXT NOT NULL,
	details JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_events_type_created ON agent_events(event_type, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_feedback (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	message_id TEXT,
	rating TEXT NOT NULL,
	comment TEXT,
	created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_feedback_user_created ON agent_feedback(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_improvements (
	id TEXT PRIMARY KEY,
	category TEXT NOT NULL,
	description TEXT NOT NULL,
	action JSONB NOT NULL,
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	applied_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_agent_improvements_status ON agent_improvements(status, created_at DESC);

CREATE TABLE IF NOT EXISTS scheduled_tasks (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	cron_expr TEXT NOT NULL,
	prompt TEXT NOT NULL,
	condition TEXT,
	webhook_url TEXT,
	enabled BOOLEAN NOT NULL DEFAULT true,
	last_run_at TIMESTAMPTZ,
	last_result TEXT,
	last_status TEXT,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_user_enabled
	ON scheduled_tasks(user_id, enabled, updated_at DESC);
`
	if _, err := tx.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("execute schema ddl: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema tx: %w", err)
	}
	return nil
}

func (r *DocumentRepository) Create(ctx context.Context, doc *domain.Document) error {
	tagsJSON, err := json.Marshal(doc.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	headers := doc.Headers
	if headers == nil {
		headers = []string{}
	}
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO documents (
	id, filename, mime_type, storage_path, category, subcategory, tags, confidence, summary,
	source_type, title, headers, path,
	status, error_message, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
`,
		doc.ID, doc.Filename, doc.MimeType, doc.StoragePath, doc.Category, doc.Subcategory, tagsJSON,
		doc.Confidence, doc.Summary,
		doc.SourceType, doc.Title, headersJSON, doc.Path,
		string(doc.Status), doc.Error, doc.CreatedAt, doc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert document: %w", err)
	}
	return nil
}

func (r *DocumentRepository) GetByID(ctx context.Context, id string) (*domain.Document, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, filename, mime_type, storage_path, category, subcategory, tags, confidence, summary,
	source_type, title, headers, path,
	status, error_message, created_at, updated_at
FROM documents
WHERE id = $1
`, id)

	var doc domain.Document
	var tagsRaw []byte
	var headersRaw []byte
	var status string

	err := row.Scan(
		&doc.ID, &doc.Filename, &doc.MimeType, &doc.StoragePath, &doc.Category, &doc.Subcategory,
		&tagsRaw, &doc.Confidence, &doc.Summary,
		&doc.SourceType, &doc.Title, &headersRaw, &doc.Path,
		&status, &doc.Error, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.WrapError(domain.ErrDocumentNotFound, "get document by id", fmt.Errorf("id=%s", id))
		}
		return nil, fmt.Errorf("scan document: %w", err)
	}

	if err := json.Unmarshal(tagsRaw, &doc.Tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}
	if err := json.Unmarshal(headersRaw, &doc.Headers); err != nil {
		return nil, fmt.Errorf("unmarshal headers: %w", err)
	}
	doc.Status = domain.DocumentStatus(status)
	return &doc, nil
}

func (r *DocumentRepository) UpdateStatus(ctx context.Context, id string, status domain.DocumentStatus, errMessage string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE documents
SET status = $2, error_message = $3, updated_at = $4
WHERE id = $1
`, id, string(status), errMessage, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update document status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for update document status: %w", err)
	}
	if rows == 0 {
		return domain.WrapError(domain.ErrDocumentNotFound, "update document status", fmt.Errorf("id=%s", id))
	}
	return nil
}

func (r *DocumentRepository) ListRecent(ctx context.Context, limit int) ([]domain.Document, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, filename, mime_type, storage_path, category, subcategory, tags, confidence, summary,
	source_type, title, headers, path,
	status, error_message, created_at, updated_at
FROM documents
ORDER BY created_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent documents: %w", err)
	}
	defer rows.Close()

	var docs []domain.Document
	for rows.Next() {
		var doc domain.Document
		var tagsRaw []byte
		var headersRaw []byte
		var status string
		if err := rows.Scan(
			&doc.ID, &doc.Filename, &doc.MimeType, &doc.StoragePath, &doc.Category, &doc.Subcategory,
			&tagsRaw, &doc.Confidence, &doc.Summary,
			&doc.SourceType, &doc.Title, &headersRaw, &doc.Path,
			&status, &doc.Error, &doc.CreatedAt, &doc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan document row: %w", err)
		}
		if err := json.Unmarshal(tagsRaw, &doc.Tags); err != nil {
			return nil, fmt.Errorf("unmarshal tags: %w", err)
		}
		if err := json.Unmarshal(headersRaw, &doc.Headers); err != nil {
			return nil, fmt.Errorf("unmarshal headers: %w", err)
		}
		doc.Status = domain.DocumentStatus(status)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (r *DocumentRepository) SaveClassification(ctx context.Context, id string, cls domain.Classification) error {
	tagsJSON, err := json.Marshal(cls.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE documents
SET category = $2, subcategory = $3, tags = $4, confidence = $5, summary = $6, updated_at = $7
WHERE id = $1
`, id, cls.Category, cls.Subcategory, tagsJSON, cls.Confidence, cls.Summary, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("save classification: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for save classification: %w", err)
	}
	if rows == 0 {
		return domain.WrapError(domain.ErrDocumentNotFound, "save classification", fmt.Errorf("id=%s", id))
	}
	return nil
}
