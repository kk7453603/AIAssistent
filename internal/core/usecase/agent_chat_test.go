package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type fakeAgentQueryService struct {
	generateResponses []string
	answerText        string
}

func (f *fakeAgentQueryService) Answer(_ context.Context, _ string, _ int, _ domain.SearchFilter) (*domain.Answer, error) {
	text := f.answerText
	if text == "" {
		text = "knowledge answer"
	}
	return &domain.Answer{
		Text: text,
		Sources: []domain.RetrievedChunk{
			{DocumentID: "doc-1", ChunkIndex: 0, Text: "chunk"},
		},
	}, nil
}

func (f *fakeAgentQueryService) GenerateFromPrompt(_ context.Context, _ string) (string, error) {
	if len(f.generateResponses) == 0 {
		return `{"type":"final","answer":"fallback"}`, nil
	}
	out := f.generateResponses[0]
	f.generateResponses = f.generateResponses[1:]
	return out, nil
}

type fakeAgentEmbedder struct{}

func (f *fakeAgentEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return nil, nil
}

func (f *fakeAgentEmbedder) EmbedQuery(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

type fakeConversationStore struct {
	currentTurn   int
	lastSummary   int
	messages      []domain.ConversationMessage
	conversation  domain.Conversation
}

func (f *fakeConversationStore) EnsureConversation(_ context.Context, userID, conversationID string) (*domain.Conversation, error) {
	if f.conversation.UserID == "" {
		now := time.Now().UTC()
		f.conversation = domain.Conversation{
			UserID:             userID,
			ConversationID:     conversationID,
			CurrentUserTurn:    0,
			LastSummaryEndTurn: f.lastSummary,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
	}
	return &f.conversation, nil
}

func (f *fakeConversationStore) NextUserTurn(_ context.Context, _ string, _ string) (int, error) {
	f.currentTurn++
	f.conversation.CurrentUserTurn = f.currentTurn
	return f.currentTurn, nil
}

func (f *fakeConversationStore) AppendMessage(_ context.Context, message domain.ConversationMessage) error {
	f.messages = append(f.messages, message)
	return nil
}

func (f *fakeConversationStore) ListRecentMessages(_ context.Context, _, _ string, limit int) ([]domain.ConversationMessage, error) {
	if limit <= 0 || len(f.messages) == 0 {
		return nil, nil
	}
	if len(f.messages) <= limit {
		return append([]domain.ConversationMessage(nil), f.messages...), nil
	}
	return append([]domain.ConversationMessage(nil), f.messages[len(f.messages)-limit:]...), nil
}

func (f *fakeConversationStore) ListMessagesByTurnRange(_ context.Context, _, _ string, turnFrom, turnTo int) ([]domain.ConversationMessage, error) {
	out := make([]domain.ConversationMessage, 0)
	for _, msg := range f.messages {
		if msg.UserTurn >= turnFrom && msg.UserTurn <= turnTo {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (f *fakeConversationStore) UpdateLastSummaryEndTurn(_ context.Context, _, _ string, turn int) error {
	f.lastSummary = turn
	return nil
}

type fakeTaskStore struct {
	tasks map[string]domain.Task
}

func (f *fakeTaskStore) CreateTask(_ context.Context, task *domain.Task) error {
	if f.tasks == nil {
		f.tasks = make(map[string]domain.Task)
	}
	f.tasks[task.ID] = *task
	return nil
}

func (f *fakeTaskStore) ListTasks(_ context.Context, userID string, includeDeleted bool) ([]domain.Task, error) {
	out := make([]domain.Task, 0)
	for _, task := range f.tasks {
		if task.UserID != userID {
			continue
		}
		if !includeDeleted && task.DeletedAt != nil {
			continue
		}
		out = append(out, task)
	}
	return out, nil
}

func (f *fakeTaskStore) GetTaskByID(_ context.Context, userID, taskID string) (*domain.Task, error) {
	task, ok := f.tasks[taskID]
	if !ok || task.UserID != userID {
		return nil, fmt.Errorf("task not found")
	}
	return &task, nil
}

func (f *fakeTaskStore) UpdateTask(_ context.Context, task *domain.Task) error {
	if f.tasks == nil {
		f.tasks = make(map[string]domain.Task)
	}
	f.tasks[task.ID] = *task
	return nil
}

func (f *fakeTaskStore) SoftDeleteTask(_ context.Context, _, taskID string) error {
	task, ok := f.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found")
	}
	now := time.Now().UTC()
	task.DeletedAt = &now
	f.tasks[taskID] = task
	return nil
}

type fakeMemoryStore struct {
	lastTurn int
	summaries []domain.MemorySummary
}

func (f *fakeMemoryStore) CreateSummary(_ context.Context, summary *domain.MemorySummary) error {
	f.summaries = append(f.summaries, *summary)
	f.lastTurn = summary.TurnTo
	return nil
}

func (f *fakeMemoryStore) GetLastSummaryEndTurn(_ context.Context, _, _ string) (int, error) {
	return f.lastTurn, nil
}

type fakeMemoryVectorStore struct {
	hits    []domain.MemoryHit
	indexed []domain.MemorySummary
}

func (f *fakeMemoryVectorStore) IndexSummary(_ context.Context, summary domain.MemorySummary, _ []float32) error {
	f.indexed = append(f.indexed, summary)
	return nil
}

func (f *fakeMemoryVectorStore) SearchSummaries(_ context.Context, _, _ string, _ []float32, _ int) ([]domain.MemoryHit, error) {
	return append([]domain.MemoryHit(nil), f.hits...), nil
}

func TestAgentChatUseCaseFinalStep(t *testing.T) {
	query := &fakeAgentQueryService{
		generateResponses: []string{
			`{"type":"final","answer":"done"}`,
		},
	}
	conversations := &fakeConversationStore{}
	uc := NewAgentChatUseCase(
		query,
		&fakeAgentEmbedder{},
		conversations,
		&fakeTaskStore{},
		&fakeMemoryStore{},
		&fakeMemoryVectorStore{
			hits: []domain.MemoryHit{{Score: 0.8, Summary: domain.MemorySummary{Summary: "old context"}}},
		},
		domain.AgentLimits{},
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID: "u-1",
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.Answer != "done" {
		t.Fatalf("expected final answer 'done', got %q", result.Answer)
	}
	if result.Iterations != 1 {
		t.Fatalf("expected 1 iteration, got %d", result.Iterations)
	}
	if result.MemoryHits != 1 {
		t.Fatalf("expected 1 memory hit, got %d", result.MemoryHits)
	}
	if len(conversations.messages) != 2 {
		t.Fatalf("expected user+assistant messages persisted, got %d", len(conversations.messages))
	}
}

func TestAgentChatUseCaseToolThenFinal(t *testing.T) {
	query := &fakeAgentQueryService{
		generateResponses: []string{
			`{"type":"tool","tool":"knowledge_search","input":{"question":"q"}}`,
			`{"type":"final","answer":"knowledge merged"}`,
		},
	}
	conversations := &fakeConversationStore{}
	uc := NewAgentChatUseCase(
		query,
		&fakeAgentEmbedder{},
		conversations,
		&fakeTaskStore{},
		&fakeMemoryStore{},
		&fakeMemoryVectorStore{},
		domain.AgentLimits{},
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID: "u-1",
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "find info"},
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if len(result.ToolsInvoked) != 1 || result.ToolsInvoked[0] != "knowledge_search" {
		t.Fatalf("expected knowledge_search tool invoked, got %#v", result.ToolsInvoked)
	}
	if len(result.ToolEvents) == 0 || result.ToolEvents[0].Status != "ok" {
		t.Fatalf("expected successful tool event, got %#v", result.ToolEvents)
	}
}

func TestAgentChatUseCaseMaxIterationsFallback(t *testing.T) {
	query := &fakeAgentQueryService{
		generateResponses: []string{
			`{"type":"tool","tool":"task_tool","action":"list","input":{}}`,
			`{"type":"tool","tool":"task_tool","action":"list","input":{}}`,
		},
	}
	uc := NewAgentChatUseCase(
		query,
		&fakeAgentEmbedder{},
		&fakeConversationStore{},
		&fakeTaskStore{},
		&fakeMemoryStore{},
		&fakeMemoryVectorStore{},
		domain.AgentLimits{MaxIterations: 2},
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID: "u-1",
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "loop"},
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.FallbackReason != "max_iterations" {
		t.Fatalf("expected fallback max_iterations, got %q", result.FallbackReason)
	}
	if !strings.Contains(result.Answer, "execution limits") {
		t.Fatalf("expected deterministic fallback answer, got %q", result.Answer)
	}
}

func TestAgentChatUseCaseCreatesSummaryOnSessionEnd(t *testing.T) {
	query := &fakeAgentQueryService{
		generateResponses: []string{
			`{"type":"final","answer":"done"}`,
			"summary text",
		},
	}
	memoryStore := &fakeMemoryStore{}
	memoryVector := &fakeMemoryVectorStore{}
	uc := NewAgentChatUseCase(
		query,
		&fakeAgentEmbedder{},
		&fakeConversationStore{},
		&fakeTaskStore{},
		memoryStore,
		memoryVector,
		domain.AgentLimits{SummaryEveryTurns: 6},
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID:     "u-1",
		SessionEnd: true,
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "wrap up"},
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if !result.SummaryCreated {
		t.Fatalf("expected summary to be created")
	}
	if len(memoryStore.summaries) != 1 || len(memoryVector.indexed) != 1 {
		t.Fatalf("expected summary persisted/indexed once, persisted=%d indexed=%d", len(memoryStore.summaries), len(memoryVector.indexed))
	}
}

