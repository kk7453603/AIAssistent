package upload

import (
	"context"
	"errors"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) SourceType() string { return "upload" }

func (a *Adapter) Ingest(_ context.Context, req domain.SourceRequest) (*domain.IngestResult, error) {
	if req.Body == nil {
		return nil, errors.New("upload: body is required")
	}
	return &domain.IngestResult{
		Filename:   req.Filename,
		MimeType:   req.MimeType,
		Body:       req.Body,
		SourceType: "upload",
		Path:       req.Filename,
	}, nil
}
