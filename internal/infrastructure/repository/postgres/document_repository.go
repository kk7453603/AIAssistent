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

	_, err = r.db.ExecContext(ctx, `
INSERT INTO documents (
	id, filename, mime_type, storage_path, category, subcategory, tags, confidence, summary, status, error_message, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
`,
		doc.ID, doc.Filename, doc.MimeType, doc.StoragePath, doc.Category, doc.Subcategory, tagsJSON,
		doc.Confidence, doc.Summary, string(doc.Status), doc.Error, doc.CreatedAt, doc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert document: %w", err)
	}
	return nil
}

func (r *DocumentRepository) GetByID(ctx context.Context, id string) (*domain.Document, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, filename, mime_type, storage_path, category, subcategory, tags, confidence, summary, status, error_message, created_at, updated_at
FROM documents
WHERE id = $1
`, id)

	var doc domain.Document
	var tagsRaw []byte
	var status string

	err := row.Scan(
		&doc.ID, &doc.Filename, &doc.MimeType, &doc.StoragePath, &doc.Category, &doc.Subcategory,
		&tagsRaw, &doc.Confidence, &doc.Summary, &status, &doc.Error, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("document not found: %s", id)
		}
		return nil, fmt.Errorf("scan document: %w", err)
	}

	if err := json.Unmarshal(tagsRaw, &doc.Tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}
	doc.Status = domain.DocumentStatus(status)
	return &doc, nil
}

func (r *DocumentRepository) UpdateStatus(ctx context.Context, id string, status domain.DocumentStatus, errMessage string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE documents
SET status = $2, error_message = $3, updated_at = $4
WHERE id = $1
`, id, string(status), errMessage, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update document status: %w", err)
	}
	return nil
}

func (r *DocumentRepository) SaveClassification(ctx context.Context, id string, cls domain.Classification) error {
	tagsJSON, err := json.Marshal(cls.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
UPDATE documents
SET category = $2, subcategory = $3, tags = $4, confidence = $5, summary = $6, updated_at = $7
WHERE id = $1
`, id, cls.Category, cls.Subcategory, tagsJSON, cls.Confidence, cls.Summary, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("save classification: %w", err)
	}
	return nil
}
