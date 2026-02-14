package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func newRepoWithMock(t *testing.T) (*DocumentRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	return &DocumentRepository{db: db}, mock, func() { _ = db.Close() }
}

func TestGetByIDReturnsDomainNotFound(t *testing.T) {
	repo, mock, done := newRepoWithMock(t)
	defer done()

	mock.ExpectQuery("SELECT id, filename, mime_type, storage_path").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetByID(context.Background(), "missing")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !domain.IsKind(err, domain.ErrDocumentNotFound) {
		t.Fatalf("expected ErrDocumentNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpdateStatusReturnsDomainNotFoundWhenNoRowsAffected(t *testing.T) {
	repo, mock, done := newRepoWithMock(t)
	defer done()

	mock.ExpectExec("UPDATE documents").
		WithArgs("missing", string(domain.StatusProcessing), "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateStatus(context.Background(), "missing", domain.StatusProcessing, "")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !domain.IsKind(err, domain.ErrDocumentNotFound) {
		t.Fatalf("expected ErrDocumentNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSaveClassificationReturnsDomainNotFoundWhenNoRowsAffected(t *testing.T) {
	repo, mock, done := newRepoWithMock(t)
	defer done()

	mock.ExpectExec("UPDATE documents").
		WithArgs("missing", "cat", "sub", sqlmock.AnyArg(), 0.9, "sum", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.SaveClassification(context.Background(), "missing", domain.Classification{
		Category:    "cat",
		Subcategory: "sub",
		Tags:        []string{"tag"},
		Confidence:  0.9,
		Summary:     "sum",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !domain.IsKind(err, domain.ErrDocumentNotFound) {
		t.Fatalf("expected ErrDocumentNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
