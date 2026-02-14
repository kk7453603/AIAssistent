package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type ingestRepoFake struct {
	created *domain.Document
	err     error
}

func (f *ingestRepoFake) Create(_ context.Context, doc *domain.Document) error {
	if f.err != nil {
		return f.err
	}
	copyDoc := *doc
	f.created = &copyDoc
	return nil
}

func (f *ingestRepoFake) GetByID(context.Context, string) (*domain.Document, error) {
	return nil, errors.New("not implemented")
}
func (f *ingestRepoFake) UpdateStatus(context.Context, string, domain.DocumentStatus, string) error {
	return errors.New("not implemented")
}
func (f *ingestRepoFake) SaveClassification(context.Context, string, domain.Classification) error {
	return errors.New("not implemented")
}

type ingestStorageFake struct {
	savedKey  string
	savedBody string
	err       error
}

func (f *ingestStorageFake) Save(_ context.Context, key string, data io.Reader) error {
	if f.err != nil {
		return f.err
	}
	raw, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	f.savedKey = key
	f.savedBody = string(raw)
	return nil
}

func (f *ingestStorageFake) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

type ingestQueueFake struct {
	documentID string
	err        error
}

func (f *ingestQueueFake) PublishDocumentIngested(_ context.Context, documentID string) error {
	if f.err != nil {
		return f.err
	}
	f.documentID = documentID
	return nil
}

func (f *ingestQueueFake) SubscribeDocumentIngested(context.Context, func(context.Context, string) error) error {
	return errors.New("not implemented")
}

func TestIngestUploadSuccess(t *testing.T) {
	repo := &ingestRepoFake{}
	storage := &ingestStorageFake{}
	queue := &ingestQueueFake{}
	uc := NewIngestDocumentUseCase(repo, storage, queue)

	doc, err := uc.Upload(context.Background(), "report 1.txt", "text/plain", bytes.NewBufferString("hello"))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if doc.ID == "" {
		t.Fatalf("expected document id")
	}
	if doc.Status != domain.StatusUploaded {
		t.Fatalf("expected status uploaded, got %s", doc.Status)
	}
	if repo.created == nil {
		t.Fatalf("expected repo.Create call")
	}
	if queue.documentID != doc.ID {
		t.Fatalf("expected queued doc id %s, got %s", doc.ID, queue.documentID)
	}
	if !strings.Contains(storage.savedKey, "_report_1.txt") {
		t.Fatalf("expected sanitized key suffix, got %s", storage.savedKey)
	}
	if storage.savedBody != "hello" {
		t.Fatalf("expected saved body hello, got %s", storage.savedBody)
	}
}

func TestIngestUploadQueueError(t *testing.T) {
	repo := &ingestRepoFake{}
	storage := &ingestStorageFake{}
	queue := &ingestQueueFake{err: errors.New("queue down")}
	uc := NewIngestDocumentUseCase(repo, storage, queue)

	_, err := uc.Upload(context.Background(), "report.txt", "text/plain", bytes.NewBufferString("hello"))
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "publish ingestion event") {
		t.Fatalf("expected publish error, got %v", err)
	}
}
