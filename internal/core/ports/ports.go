package ports

import (
	"context"
	"io"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type DocumentRepository interface {
	Create(ctx context.Context, doc *domain.Document) error
	GetByID(ctx context.Context, id string) (*domain.Document, error)
	UpdateStatus(ctx context.Context, id string, status domain.DocumentStatus, errMessage string) error
	SaveClassification(ctx context.Context, id string, cls domain.Classification) error
}

type ObjectStorage interface {
	Save(ctx context.Context, key string, data io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

type MessageQueue interface {
	PublishDocumentIngested(ctx context.Context, documentID string) error
	SubscribeDocumentIngested(ctx context.Context, handler func(context.Context, string) error) error
}

type TextExtractor interface {
	Extract(ctx context.Context, doc *domain.Document) (string, error)
}

type DocumentClassifier interface {
	Classify(ctx context.Context, text string) (domain.Classification, error)
}

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

type Chunker interface {
	Split(text string) []string
}

type VectorStore interface {
	IndexChunks(ctx context.Context, doc *domain.Document, chunks []string, vectors [][]float32) error
	Search(ctx context.Context, queryVector []float32, limit int, filter domain.SearchFilter) ([]domain.RetrievedChunk, error)
}

type AnswerGenerator interface {
	GenerateAnswer(ctx context.Context, question string, chunks []domain.RetrievedChunk) (string, error)
	GenerateFromPrompt(ctx context.Context, prompt string) (string, error)
}
