package obsidian

import (
	"context"
	"errors"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// Adapter is a stub for Obsidian vault sync. Will be implemented when vault sync is built.
type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) SourceType() string { return "obsidian" }

func (a *Adapter) Ingest(_ context.Context, _ domain.SourceRequest) (*domain.IngestResult, error) {
	return nil, errors.New("obsidian source adapter not implemented")
}
