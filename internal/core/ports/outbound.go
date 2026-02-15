package ports

import (
	"context"
	"io"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

// DocumentRepository persists and reads document state.
type DocumentRepository interface {
	Create(ctx context.Context, doc *domain.Document) error
	GetByID(ctx context.Context, id string) (*domain.Document, error)
	UpdateStatus(ctx context.Context, id string, status domain.DocumentStatus, errMessage string) error
	SaveClassification(ctx context.Context, id string, cls domain.Classification) error
}

// ObjectStorage stores source documents.
type ObjectStorage interface {
	Save(ctx context.Context, key string, data io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// MessageQueue publishes/consumes ingestion events.
type MessageQueue interface {
	PublishDocumentIngested(ctx context.Context, documentID string) error
	SubscribeDocumentIngested(ctx context.Context, handler func(context.Context, string) error) error
}

// TextExtractor extracts plain text from a stored document.
type TextExtractor interface {
	Extract(ctx context.Context, doc *domain.Document) (string, error)
}

// DocumentClassifier classifies extracted text.
type DocumentClassifier interface {
	Classify(ctx context.Context, text string) (domain.Classification, error)
}

// Embedder builds vectors for chunks and query text.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

// Chunker splits text into semantically usable chunks.
type Chunker interface {
	Split(text string) []string
}

// VectorStore indexes chunks and performs semantic search.
type VectorStore interface {
	IndexChunks(ctx context.Context, doc *domain.Document, chunks []string, vectors [][]float32) error
	Search(ctx context.Context, queryVector []float32, limit int, filter domain.SearchFilter) ([]domain.RetrievedChunk, error)
	SearchLexical(ctx context.Context, queryText string, limit int, filter domain.SearchFilter) ([]domain.RetrievedChunk, error)
}

// AnswerGenerator creates the final user-facing answer.
type AnswerGenerator interface {
	GenerateAnswer(ctx context.Context, question string, chunks []domain.RetrievedChunk) (string, error)
	GenerateFromPrompt(ctx context.Context, prompt string) (string, error)
	GenerateJSONFromPrompt(ctx context.Context, prompt string) (string, error)
}

// ConversationStore persists conversation state and messages.
type ConversationStore interface {
	EnsureConversation(ctx context.Context, userID, conversationID string) (*domain.Conversation, error)
	NextUserTurn(ctx context.Context, userID, conversationID string) (int, error)
	AppendMessage(ctx context.Context, message domain.ConversationMessage) error
	ListRecentMessages(ctx context.Context, userID, conversationID string, limit int) ([]domain.ConversationMessage, error)
	ListMessagesByTurnRange(ctx context.Context, userID, conversationID string, turnFrom, turnTo int) ([]domain.ConversationMessage, error)
	UpdateLastSummaryEndTurn(ctx context.Context, userID, conversationID string, turn int) error
}

// TaskStore persists and retrieves user tasks.
type TaskStore interface {
	CreateTask(ctx context.Context, task *domain.Task) error
	ListTasks(ctx context.Context, userID string, includeDeleted bool) ([]domain.Task, error)
	GetTaskByID(ctx context.Context, userID, taskID string) (*domain.Task, error)
	UpdateTask(ctx context.Context, task *domain.Task) error
	SoftDeleteTask(ctx context.Context, userID, taskID string) error
}

// MemoryStore persists memory summaries.
type MemoryStore interface {
	CreateSummary(ctx context.Context, summary *domain.MemorySummary) error
	GetLastSummaryEndTurn(ctx context.Context, userID, conversationID string) (int, error)
}

// MemoryVectorStore indexes and searches memory summaries semantically.
type MemoryVectorStore interface {
	IndexSummary(ctx context.Context, summary domain.MemorySummary, vector []float32) error
	SearchSummaries(ctx context.Context, userID, conversationID string, queryVector []float32, limit int) ([]domain.MemoryHit, error)
}
