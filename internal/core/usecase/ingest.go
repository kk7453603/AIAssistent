package usecase

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type IngestDocumentUseCase struct {
	repo    ports.DocumentRepository
	storage ports.ObjectStorage
	queue   ports.MessageQueue
}

func NewIngestDocumentUseCase(
	repo ports.DocumentRepository,
	storage ports.ObjectStorage,
	queue ports.MessageQueue,
) *IngestDocumentUseCase {
	return &IngestDocumentUseCase{
		repo:    repo,
		storage: storage,
		queue:   queue,
	}
}

func (uc *IngestDocumentUseCase) Upload(
	ctx context.Context,
	filename, mimeType string,
	body io.Reader,
) (*domain.Document, error) {
	id := uuid.NewString()
	storageKey := fmt.Sprintf("%s_%s", id, sanitizeFilename(filename))
	now := time.Now().UTC()

	if err := uc.storage.Save(ctx, storageKey, body); err != nil {
		return nil, fmt.Errorf("save to object storage: %w", err)
	}

	doc := &domain.Document{
		ID:          id,
		Filename:    filename,
		MimeType:    mimeType,
		StoragePath: storageKey,
		Status:      domain.StatusUploaded,
		Tags:        []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := uc.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("create document metadata: %w", err)
	}

	if err := uc.queue.PublishDocumentIngested(ctx, doc.ID); err != nil {
		return nil, fmt.Errorf("publish ingestion event: %w", err)
	}

	return doc, nil
}

func sanitizeFilename(name string) string {
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, " ", "_")
	base = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, base)
	if base == "" {
		return "document.bin"
	}
	return base
}
