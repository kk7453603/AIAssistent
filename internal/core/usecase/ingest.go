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
	repo     ports.DocumentRepository
	storage  ports.ObjectStorage
	queue    ports.MessageQueue
	adapters map[string]ports.SourceAdapter
}

func NewIngestDocumentUseCase(
	repo ports.DocumentRepository,
	storage ports.ObjectStorage,
	queue ports.MessageQueue,
	adapters map[string]ports.SourceAdapter,
) *IngestDocumentUseCase {
	return &IngestDocumentUseCase{
		repo:     repo,
		storage:  storage,
		queue:    queue,
		adapters: adapters,
	}
}

func (uc *IngestDocumentUseCase) Upload(
	ctx context.Context,
	filename, mimeType string,
	body io.Reader,
) (*domain.Document, error) {
	return uc.IngestFromSource(ctx, domain.SourceRequest{
		SourceType: "upload",
		Filename:   filename,
		MimeType:   mimeType,
		Body:       body,
	})
}

func (uc *IngestDocumentUseCase) IngestFromSource(
	ctx context.Context,
	req domain.SourceRequest,
) (*domain.Document, error) {
	adapter, ok := uc.adapters[req.SourceType]
	if !ok {
		return nil, fmt.Errorf("unknown source type: %q", req.SourceType)
	}

	result, err := adapter.Ingest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("source adapter %q: %w", req.SourceType, err)
	}

	id := uuid.NewString()
	storageKey := fmt.Sprintf("%s_%s", id, sanitizeFilename(result.Filename))
	now := time.Now().UTC()

	if err := uc.storage.Save(ctx, storageKey, result.Body); err != nil {
		return nil, fmt.Errorf("save to object storage: %w", err)
	}

	doc := &domain.Document{
		ID:          id,
		Filename:    result.Filename,
		MimeType:    result.MimeType,
		StoragePath: storageKey,
		SourceType:  result.SourceType,
		Path:        result.Path,
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
