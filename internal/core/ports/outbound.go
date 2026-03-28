package ports

import (
	"context"
	"io"
	"time"

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
	PublishDocumentEnrich(ctx context.Context, documentID string) error
	SubscribeDocumentEnrich(ctx context.Context, handler func(context.Context, string) error) error
}

// TextExtractor extracts plain text from a stored document.
type TextExtractor interface {
	Extract(ctx context.Context, doc *domain.Document) (string, error)
}

// ExtractorRegistry selects a TextExtractor based on MIME type.
type ExtractorRegistry interface {
	ForMimeType(mimeType string) TextExtractor
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

// ChunkerRegistry selects a Chunker based on source type.
type ChunkerRegistry interface {
	ForSource(sourceType string) Chunker
}

// VectorStore indexes chunks and performs semantic search.
type VectorStore interface {
	IndexChunks(ctx context.Context, doc *domain.Document, chunks []string, vectors [][]float32) error
	Search(ctx context.Context, queryVector []float32, limit int, filter domain.SearchFilter) ([]domain.RetrievedChunk, error)
	SearchLexical(ctx context.Context, queryText string, limit int, filter domain.SearchFilter) ([]domain.RetrievedChunk, error)
	UpdateChunksPayload(ctx context.Context, docID string, sourceType string, payload map[string]any) error
}

// AnswerGenerator creates the final user-facing answer.
type AnswerGenerator interface {
	GenerateAnswer(ctx context.Context, question string, chunks []domain.RetrievedChunk) (string, error)
	GenerateFromPrompt(ctx context.Context, prompt string) (string, error)
	GenerateJSONFromPrompt(ctx context.Context, prompt string) (string, error)
	ChatWithTools(ctx context.Context, messages []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error)
}

// Reranker rescores candidate chunks against the query.
type Reranker interface {
	Rerank(ctx context.Context, query string, chunks []domain.RetrievedChunk, topN int) ([]domain.RetrievedChunk, error)
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

// WebSearcher performs web searches via an external search engine.
type WebSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]domain.WebSearchResult, error)
}

// ObsidianNoteWriter creates notes in Obsidian vaults.
type ObsidianNoteWriter interface {
	CreateNote(ctx context.Context, vaultID, title, content, folder string) (string, error)
}

// MetadataExtractor extracts document metadata deterministically (no LLM).
type MetadataExtractor interface {
	ExtractMetadata(ctx context.Context, doc *domain.Document, text string) (domain.DocumentMetadata, error)
}

// SourceAdapter normalizes content from any source into an ingestable document.
type SourceAdapter interface {
	Ingest(ctx context.Context, req domain.SourceRequest) (*domain.IngestResult, error)
	SourceType() string
}

// GraphStore manages the document knowledge graph.
type GraphStore interface {
	UpsertDocument(ctx context.Context, doc domain.GraphNode) error
	AddLink(ctx context.Context, sourceID, targetID string, linkType string) error
	AddSimilarity(ctx context.Context, sourceID, targetID string, score float64) error
	RemoveSimilarities(ctx context.Context, docID string) error
	GetRelated(ctx context.Context, docID string, maxDepth int, limit int) ([]domain.GraphRelation, error)
	FindByTitle(ctx context.Context, title string) ([]domain.GraphNode, error)
	GetGraph(ctx context.Context, filter domain.GraphFilter) (*domain.Graph, error)
}

// OrchestrationStore persists multi-agent orchestration history.
type OrchestrationStore interface {
	Create(ctx context.Context, orch *domain.Orchestration) error
	AddStep(ctx context.Context, orchID string, step domain.OrchestrationStep) error
	Complete(ctx context.Context, orchID string, status string) error
	GetByID(ctx context.Context, orchID string) (*domain.Orchestration, error)
	ListByUser(ctx context.Context, userID string, limit int) ([]domain.Orchestration, error)
}

// EventStore records and queries agent execution events.
type EventStore interface {
	Record(ctx context.Context, event *domain.AgentEvent) error
	ListByType(ctx context.Context, eventType string, since time.Time, limit int) ([]domain.AgentEvent, error)
	CountByType(ctx context.Context, since time.Time) (map[string]int, error)
}

// FeedbackStore persists user feedback on agent responses.
type FeedbackStore interface {
	Create(ctx context.Context, fb *domain.AgentFeedback) error
	ListRecent(ctx context.Context, since time.Time, limit int) ([]domain.AgentFeedback, error)
	CountByRating(ctx context.Context, since time.Time) (map[string]int, error)
}

// ImprovementStore manages generated improvement suggestions.
type ImprovementStore interface {
	Create(ctx context.Context, imp *domain.AgentImprovement) error
	ListPending(ctx context.Context) ([]domain.AgentImprovement, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	MarkApplied(ctx context.Context, id string) error
}

// ScheduleStore persists and queries scheduled tasks.
type ScheduleStore interface {
	Create(ctx context.Context, task *domain.ScheduledTask) error
	ListByUser(ctx context.Context, userID string) ([]domain.ScheduledTask, error)
	ListEnabled(ctx context.Context) ([]domain.ScheduledTask, error)
	GetByID(ctx context.Context, id string) (*domain.ScheduledTask, error)
	Update(ctx context.Context, task *domain.ScheduledTask) error
	Delete(ctx context.Context, id string) error
	RecordRun(ctx context.Context, id string, result string, status string) error
}
