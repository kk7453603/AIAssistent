package ports

import (
	"context"
	"io"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// DocumentIngestor is the inbound contract for document upload orchestration.
type DocumentIngestor interface {
	Upload(ctx context.Context, filename, mimeType string, body io.Reader) (*domain.Document, error)
}

// DocumentQueryService is the inbound contract for RAG and prompt-based generation.
type DocumentQueryService interface {
	Answer(ctx context.Context, question string, limit int, filter domain.SearchFilter) (*domain.Answer, error)
	GenerateFromPrompt(ctx context.Context, prompt string) (string, error)
}

// DocumentReader is the inbound read model for document metadata/state.
type DocumentReader interface {
	GetByID(ctx context.Context, id string) (*domain.Document, error)
}

// DocumentProcessor is the inbound contract for asynchronous document processing.
type DocumentProcessor interface {
	ProcessByID(ctx context.Context, documentID string) error
}
