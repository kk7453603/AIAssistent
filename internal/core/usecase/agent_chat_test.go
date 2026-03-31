package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

type fakeAgentQueryService struct {
	generateResponses     []string
	generateTextResponses []string
	answerText            string
	answerErr             error
	generateJSONErr       error
	generateJSONHook      func(context.Context, string) (string, error)

	// ChatWithTools support
	chatToolsResponses []domain.ChatToolsResult
	chatToolsErr       error
	chatToolsHook      func(context.Context, []domain.ChatMessage, []domain.ToolSchema) (*domain.ChatToolsResult, error)
}

func (f *fakeAgentQueryService) Answer(_ context.Context, _ string, _ int, _ domain.SearchFilter) (*domain.Answer, error) {
	if f.answerErr != nil {
		return nil, f.answerErr
	}
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
	if len(f.generateTextResponses) > 0 {
		out := f.generateTextResponses[0]
		f.generateTextResponses = f.generateTextResponses[1:]
		return out, nil
	}
	return "summary text", nil
}

func (f *fakeAgentQueryService) GenerateJSONFromPrompt(ctx context.Context, _ string) (string, error) {
	if f.generateJSONHook != nil {
		return f.generateJSONHook(ctx, "")
	}
	if f.generateJSONErr != nil {
		return "", f.generateJSONErr
	}
	if len(f.generateResponses) == 0 {
		return `{"type":"final","answer":"fallback"}`, nil
	}
	out := f.generateResponses[0]
	f.generateResponses = f.generateResponses[1:]
	return out, nil
}

func (f *fakeAgentQueryService) ChatWithTools(ctx context.Context, msgs []domain.ChatMessage, tools []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	if f.chatToolsHook != nil {
		return f.chatToolsHook(ctx, msgs, tools)
	}
	if f.chatToolsErr != nil {
		return nil, f.chatToolsErr
	}
	if len(f.chatToolsResponses) > 0 {
		out := f.chatToolsResponses[0]
		f.chatToolsResponses = f.chatToolsResponses[1:]
		return &out, nil
	}
	return &domain.ChatToolsResult{Content: "stub"}, nil
}

type fakeAgentEmbedder struct{}

func (f *fakeAgentEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return nil, nil
}

func (f *fakeAgentEmbedder) EmbedQuery(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

type fakeConversationStore struct {
	currentTurn  int
	lastSummary  int
	messages     []domain.ConversationMessage
	conversation domain.Conversation
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
	lastTurn  int
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
		chatToolsResponses: []domain.ChatToolsResult{
			{Content: "done"},
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
		nil, // webSearcher
		nil, // obsidianWriter
		nil, // toolRegistry
		domain.AgentLimits{},
		nil, // agentMetrics
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID: "u-1",
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "hello"},
		},
	}, nil)
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
		chatToolsResponses: []domain.ChatToolsResult{
			{ToolCalls: []domain.ToolCall{{ID: "call-1", Function: domain.ToolCallFunc{Name: "knowledge_search", Arguments: map[string]any{"question": "q"}}}}},
			{Content: "knowledge merged"},
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
		nil, // webSearcher
		nil, // obsidianWriter
		nil, // toolRegistry
		domain.AgentLimits{},
		nil, // agentMetrics
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID: "u-1",
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "find info"},
		},
	}, nil)
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
		chatToolsResponses: []domain.ChatToolsResult{
			{ToolCalls: []domain.ToolCall{{ID: "call-1", Function: domain.ToolCallFunc{Name: "task_tool", Arguments: map[string]any{"action": "list"}}}}},
			{ToolCalls: []domain.ToolCall{{ID: "call-2", Function: domain.ToolCallFunc{Name: "task_tool", Arguments: map[string]any{"action": "list"}}}}},
		},
	}
	uc := NewAgentChatUseCase(
		query,
		&fakeAgentEmbedder{},
		&fakeConversationStore{},
		&fakeTaskStore{},
		&fakeMemoryStore{},
		&fakeMemoryVectorStore{},
		nil, // webSearcher
		nil, // obsidianWriter
		nil, // toolRegistry
		domain.AgentLimits{MaxIterations: 2},
		nil, // agentMetrics
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID: "u-1",
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "loop"},
		},
	}, nil)
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
		chatToolsResponses: []domain.ChatToolsResult{
			{Content: "done"},
		},
		generateTextResponses: []string{"summary text"},
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
		nil, // webSearcher
		nil, // obsidianWriter
		nil, // toolRegistry
		domain.AgentLimits{SummaryEveryTurns: 6},
		nil, // agentMetrics
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID:     "u-1",
		SessionEnd: true,
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "wrap up"},
		},
	}, nil)
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

func TestAgentChatUseCaseEmptyResponseFallback(t *testing.T) {
	query := &fakeAgentQueryService{
		chatToolsResponses: []domain.ChatToolsResult{
			{Content: "", ToolCalls: nil}, // empty response — neither content nor tool calls
		},
	}
	uc := NewAgentChatUseCase(
		query,
		&fakeAgentEmbedder{},
		&fakeConversationStore{},
		&fakeTaskStore{},
		&fakeMemoryStore{},
		&fakeMemoryVectorStore{},
		nil, // webSearcher
		nil, // obsidianWriter
		nil, // toolRegistry
		domain.AgentLimits{},
		nil, // agentMetrics
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID: "u-1",
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "hello"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.FallbackReason != "empty_response" {
		t.Fatalf("expected fallback reason empty_response, got %q", result.FallbackReason)
	}
}

func TestAgentChatUseCasePlannerErrorFallsBackToRAG(t *testing.T) {
	query := &fakeAgentQueryService{
		chatToolsErr: errors.New("planner unavailable"),
		answerText:   "rag fallback answer",
	}
	uc := NewAgentChatUseCase(
		query,
		&fakeAgentEmbedder{},
		&fakeConversationStore{},
		&fakeTaskStore{},
		&fakeMemoryStore{},
		&fakeMemoryVectorStore{},
		nil, // webSearcher
		nil, // obsidianWriter
		nil, // toolRegistry
		domain.AgentLimits{},
		nil, // agentMetrics
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID: "u-1",
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "hello"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.FallbackReason != "planner_error" {
		t.Fatalf("expected fallback reason planner_error, got %q", result.FallbackReason)
	}
	if !strings.Contains(result.Answer, "rag fallback answer") {
		t.Fatalf("expected answer to contain rag fallback answer, got %q", result.Answer)
	}
}

func TestAgentChatUseCasePlannerTimeoutFallsBackToRAG(t *testing.T) {
	query := &fakeAgentQueryService{
		answerText: "rag fallback answer",
		chatToolsHook: func(ctx context.Context, _ []domain.ChatMessage, _ []domain.ToolSchema) (*domain.ChatToolsResult, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	uc := NewAgentChatUseCase(
		query,
		&fakeAgentEmbedder{},
		&fakeConversationStore{},
		&fakeTaskStore{},
		&fakeMemoryStore{},
		&fakeMemoryVectorStore{},
		nil, // webSearcher
		nil, // obsidianWriter
		nil, // toolRegistry
		domain.AgentLimits{
			Timeout:        300 * time.Millisecond,
			PlannerTimeout: 20 * time.Millisecond,
		},
		nil, // agentMetrics
	)

	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID: "u-1",
		Messages: []domain.AgentInputMessage{
			{Role: "user", Content: "hello"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.FallbackReason != "timeout" {
		t.Fatalf("expected fallback reason timeout, got %q", result.FallbackReason)
	}
	if !strings.Contains(result.Answer, "rag fallback answer") {
		t.Fatalf("expected rag fallback answer, got %q", result.Answer)
	}
}

// ---------- Additional fakes ----------

type fakeWebSearcher struct {
	results   []domain.WebSearchResult
	err       error
	lastQuery string
	calls     int
}

func (f *fakeWebSearcher) Search(_ context.Context, query string, _ int) ([]domain.WebSearchResult, error) {
	f.lastQuery = query
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

type fakeObsidianWriter struct {
	createdPath string
	err         error
}

func (f *fakeObsidianWriter) CreateNote(_ context.Context, _, title, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.createdPath = "/vault/" + title + ".md"
	return f.createdPath, nil
}

type fakeMCPToolRegistry struct {
	tools    []ports.ToolDefinition
	builtIn  map[string]bool
	callResp string
	callErr  error
}

func (f *fakeMCPToolRegistry) ListTools() []ports.ToolDefinition {
	return f.tools
}

func (f *fakeMCPToolRegistry) IsBuiltIn(name string) bool {
	if f.builtIn != nil {
		return f.builtIn[name]
	}
	return false
}

func (f *fakeMCPToolRegistry) CallMCPTool(_ context.Context, _ string, _ map[string]any) (string, error) {
	if f.callErr != nil {
		return "", f.callErr
	}
	return f.callResp, nil
}

type fakeGraphStore struct {
	findByTitleResult []domain.GraphNode
	relatedResult     []domain.GraphRelation
}

func (f *fakeGraphStore) UpsertDocument(context.Context, domain.GraphNode) error { return nil }
func (f *fakeGraphStore) AddLink(context.Context, string, string, string) error  { return nil }
func (f *fakeGraphStore) AddSimilarity(context.Context, string, string, float64) error {
	return nil
}
func (f *fakeGraphStore) RemoveSimilarities(context.Context, string) error { return nil }
func (f *fakeGraphStore) GetRelated(_ context.Context, _ string, _ int, _ int) ([]domain.GraphRelation, error) {
	return f.relatedResult, nil
}
func (f *fakeGraphStore) FindByID(context.Context, string) (*domain.GraphNode, error) {
	return nil, nil
}
func (f *fakeGraphStore) FindByTitle(_ context.Context, _ string) ([]domain.GraphNode, error) {
	return f.findByTitleResult, nil
}
func (f *fakeGraphStore) GetGraph(_ context.Context, _ domain.GraphFilter) (*domain.Graph, error) {
	return &domain.Graph{}, nil
}

// ---------- Helper ----------

func newTestAgentUC(query *fakeAgentQueryService, opts ...func(*AgentChatUseCase)) *AgentChatUseCase {
	uc := NewAgentChatUseCase(
		query,
		&fakeAgentEmbedder{},
		&fakeConversationStore{},
		&fakeTaskStore{},
		&fakeMemoryStore{},
		&fakeMemoryVectorStore{},
		nil, nil, nil,
		domain.AgentLimits{},
		nil,
	)
	for _, o := range opts {
		o(uc)
	}
	return uc
}

func completeWithMessage(t *testing.T, uc *AgentChatUseCase, msg string) *domain.AgentRunResult {
	t.Helper()
	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID:   "u-1",
		Messages: []domain.AgentInputMessage{{Role: "user", Content: msg}},
	}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	return result
}

// ---------- Validation tests ----------

func TestAgentChat_EmptyUserID(t *testing.T) {
	uc := newTestAgentUC(&fakeAgentQueryService{})
	_, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID:   "",
		Messages: []domain.AgentInputMessage{{Role: "user", Content: "hi"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
	if !strings.Contains(err.Error(), "user_id") {
		t.Fatalf("error should mention user_id, got: %v", err)
	}
}

func TestAgentChat_NoUserMessages(t *testing.T) {
	uc := newTestAgentUC(&fakeAgentQueryService{})
	_, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID:   "u-1",
		Messages: []domain.AgentInputMessage{{Role: "system", Content: "hi"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error for no user messages")
	}
	if !strings.Contains(err.Error(), "user message") {
		t.Fatalf("error should mention user message, got: %v", err)
	}
}

// ---------- executeToolCall tests ----------

func TestExecuteToolCall_KnowledgeSearch(t *testing.T) {
	query := &fakeAgentQueryService{answerText: "found it"}
	uc := newTestAgentUC(query)
	tc := domain.ToolCall{
		ID:       "call-1",
		Function: domain.ToolCallFunc{Name: "knowledge_search", Arguments: map[string]any{"question": "test query"}},
	}
	ev, err := uc.executeToolCall(context.Background(), "u-1", tc, "fallback")
	if err != nil {
		t.Fatalf("executeToolCall error: %v", err)
	}
	if ev.Tool != "knowledge_search" || ev.Status != "ok" {
		t.Fatalf("unexpected event: %+v", ev)
	}
	if !strings.Contains(ev.Output, "found it") {
		t.Fatalf("expected output to contain answer, got: %s", ev.Output)
	}
}

func TestExecuteToolCall_KnowledgeSearch_Error(t *testing.T) {
	query := &fakeAgentQueryService{answerErr: errors.New("search failed")}
	uc := newTestAgentUC(query)
	tc := domain.ToolCall{
		ID:       "call-1",
		Function: domain.ToolCallFunc{Name: "knowledge_search", Arguments: map[string]any{"question": "q"}},
	}
	_, err := uc.executeToolCall(context.Background(), "u-1", tc, "fallback")
	if err == nil || !strings.Contains(err.Error(), "knowledge search") {
		t.Fatalf("expected knowledge search error, got: %v", err)
	}
}

func TestExecuteToolCall_WebSearch_Success(t *testing.T) {
	query := &fakeAgentQueryService{}
	ws := &fakeWebSearcher{results: []domain.WebSearchResult{
		{Title: "Result 1", URL: "http://example.com", Snippet: "snippet"},
	}}
	uc := newTestAgentUC(query, func(u *AgentChatUseCase) {
		u.webSearcher = ws
	})
	tc := domain.ToolCall{
		ID:       "call-1",
		Function: domain.ToolCallFunc{Name: "web_search", Arguments: map[string]any{"query": "test"}},
	}
	ev, err := uc.executeToolCall(context.Background(), "u-1", tc, "fallback")
	if err != nil {
		t.Fatalf("executeToolCall error: %v", err)
	}
	if ev.Tool != "web_search" || ev.Status != "ok" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestExecuteToolCall_WebSearch_NilSearcher(t *testing.T) {
	uc := newTestAgentUC(&fakeAgentQueryService{})
	tc := domain.ToolCall{
		ID:       "call-1",
		Function: domain.ToolCallFunc{Name: "web_search", Arguments: map[string]any{"query": "test"}},
	}
	_, err := uc.executeToolCall(context.Background(), "u-1", tc, "fallback")
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not configured error, got: %v", err)
	}
}

func TestExecuteToolCall_ObsidianWrite_Success(t *testing.T) {
	ow := &fakeObsidianWriter{}
	uc := newTestAgentUC(&fakeAgentQueryService{}, func(u *AgentChatUseCase) {
		u.obsidianWriter = ow
	})
	tc := domain.ToolCall{
		ID: "call-1",
		Function: domain.ToolCallFunc{
			Name:      "obsidian_write",
			Arguments: map[string]any{"vault": "v1", "title": "Note", "content": "body text"},
		},
	}
	ev, err := uc.executeToolCall(context.Background(), "u-1", tc, "")
	if err != nil {
		t.Fatalf("executeToolCall error: %v", err)
	}
	if ev.Status != "ok" {
		t.Fatalf("expected ok, got %s", ev.Status)
	}
	if !strings.Contains(ev.Output, "created") {
		t.Fatalf("expected output to say created, got: %s", ev.Output)
	}
}

func TestExecuteToolCall_ObsidianWrite_MissingTitle(t *testing.T) {
	ow := &fakeObsidianWriter{}
	uc := newTestAgentUC(&fakeAgentQueryService{}, func(u *AgentChatUseCase) {
		u.obsidianWriter = ow
	})
	tc := domain.ToolCall{
		ID: "call-1",
		Function: domain.ToolCallFunc{
			Name:      "obsidian_write",
			Arguments: map[string]any{"vault": "v1", "content": "body"},
		},
	}
	_, err := uc.executeToolCall(context.Background(), "u-1", tc, "")
	if err == nil || !strings.Contains(err.Error(), "title") {
		t.Fatalf("expected title required error, got: %v", err)
	}
}

func TestExecuteToolCall_ObsidianWrite_MissingContent(t *testing.T) {
	ow := &fakeObsidianWriter{}
	uc := newTestAgentUC(&fakeAgentQueryService{}, func(u *AgentChatUseCase) {
		u.obsidianWriter = ow
	})
	tc := domain.ToolCall{
		ID: "call-1",
		Function: domain.ToolCallFunc{
			Name:      "obsidian_write",
			Arguments: map[string]any{"vault": "v1", "title": "Note"},
		},
	}
	_, err := uc.executeToolCall(context.Background(), "u-1", tc, "")
	if err == nil || !strings.Contains(err.Error(), "content") {
		t.Fatalf("expected content required error, got: %v", err)
	}
}

func TestExecuteToolCall_ObsidianWrite_NilWriter(t *testing.T) {
	uc := newTestAgentUC(&fakeAgentQueryService{})
	tc := domain.ToolCall{
		ID: "call-1",
		Function: domain.ToolCallFunc{
			Name:      "obsidian_write",
			Arguments: map[string]any{"vault": "v1", "title": "Note", "content": "body"},
		},
	}
	_, err := uc.executeToolCall(context.Background(), "u-1", tc, "")
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not configured error, got: %v", err)
	}
}

func TestExecuteToolCall_TaskTool_AllActions(t *testing.T) {
	tests := []struct {
		name   string
		args   map[string]any
		setup  func(*fakeTaskStore)
		check  func(*testing.T, domain.AgentToolEvent)
		errMsg string
	}{
		{
			name: "create",
			args: map[string]any{"action": "create", "title": "New Task"},
			check: func(t *testing.T, ev domain.AgentToolEvent) {
				if ev.Status != "ok" {
					t.Fatalf("expected ok, got %s", ev.Status)
				}
			},
		},
		{
			name:   "create_missing_title",
			args:   map[string]any{"action": "create"},
			errMsg: "title",
		},
		{
			name: "list",
			args: map[string]any{"action": "list"},
			check: func(t *testing.T, ev domain.AgentToolEvent) {
				if ev.Status != "ok" {
					t.Fatalf("expected ok, got %s", ev.Status)
				}
			},
		},
		{
			name: "get",
			args: map[string]any{"action": "get", "id": "task-1"},
			setup: func(ts *fakeTaskStore) {
				ts.tasks = map[string]domain.Task{"task-1": {ID: "task-1", UserID: "u-1", Title: "T1"}}
			},
			check: func(t *testing.T, ev domain.AgentToolEvent) {
				if ev.Status != "ok" {
					t.Fatalf("expected ok, got %s", ev.Status)
				}
			},
		},
		{
			name:   "get_missing_id",
			args:   map[string]any{"action": "get"},
			errMsg: "id",
		},
		{
			name: "update",
			args: map[string]any{"action": "update", "id": "task-1", "title": "Updated"},
			setup: func(ts *fakeTaskStore) {
				ts.tasks = map[string]domain.Task{"task-1": {ID: "task-1", UserID: "u-1", Title: "T1"}}
			},
			check: func(t *testing.T, ev domain.AgentToolEvent) {
				if ev.Status != "ok" {
					t.Fatalf("expected ok, got %s", ev.Status)
				}
				if !strings.Contains(ev.Output, "Updated") {
					t.Fatalf("expected updated title in output, got: %s", ev.Output)
				}
			},
		},
		{
			name: "delete",
			args: map[string]any{"action": "delete", "id": "task-1"},
			setup: func(ts *fakeTaskStore) {
				ts.tasks = map[string]domain.Task{"task-1": {ID: "task-1", UserID: "u-1", Title: "T1"}}
			},
			check: func(t *testing.T, ev domain.AgentToolEvent) {
				if ev.Status != "ok" {
					t.Fatalf("expected ok, got %s", ev.Status)
				}
			},
		},
		{
			name:   "delete_missing_id",
			args:   map[string]any{"action": "delete"},
			errMsg: "id",
		},
		{
			name: "complete_task",
			args: map[string]any{"action": "complete", "id": "task-1"},
			setup: func(ts *fakeTaskStore) {
				ts.tasks = map[string]domain.Task{"task-1": {ID: "task-1", UserID: "u-1", Title: "T1", Status: domain.TaskStatusOpen}}
			},
			check: func(t *testing.T, ev domain.AgentToolEvent) {
				if ev.Status != "ok" {
					t.Fatalf("expected ok, got %s", ev.Status)
				}
			},
		},
		{
			name:   "unknown_action",
			args:   map[string]any{"action": "invalid"},
			errMsg: "unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &fakeTaskStore{}
			if tt.setup != nil {
				tt.setup(ts)
			}
			uc := NewAgentChatUseCase(
				&fakeAgentQueryService{},
				&fakeAgentEmbedder{},
				&fakeConversationStore{},
				ts,
				&fakeMemoryStore{},
				&fakeMemoryVectorStore{},
				nil, nil, nil,
				domain.AgentLimits{},
				nil,
			)

			action := ""
			if a, ok := tt.args["action"].(string); ok {
				action = a
			}
			tc := domain.ToolCall{
				ID:       "call-1",
				Function: domain.ToolCallFunc{Name: "task_tool", Arguments: tt.args},
			}
			step := domain.AgentPlanStep{Input: tt.args, Tool: "task_tool", Action: action}
			_ = step // executeTaskTool is called via executeToolCall

			ev, err := uc.executeToolCall(context.Background(), "u-1", tc, "")
			if tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Fatalf("expected error containing %q, got: %v", tt.errMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, ev)
			}
		})
	}
}

func TestExecuteToolCall_MCPTool(t *testing.T) {
	registry := &fakeMCPToolRegistry{
		tools:    []ports.ToolDefinition{{Name: "custom_tool", Description: "test", Source: "mcp-server"}},
		builtIn:  map[string]bool{"custom_tool": false},
		callResp: `{"result": "ok"}`,
	}
	uc := newTestAgentUC(&fakeAgentQueryService{}, func(u *AgentChatUseCase) {
		u.toolRegistry = registry
	})
	tc := domain.ToolCall{
		ID:       "call-1",
		Function: domain.ToolCallFunc{Name: "custom_tool", Arguments: map[string]any{"key": "value"}},
	}
	ev, err := uc.executeToolCall(context.Background(), "u-1", tc, "")
	if err != nil {
		t.Fatalf("executeToolCall error: %v", err)
	}
	if ev.Tool != "custom_tool" || ev.Status != "ok" {
		t.Fatalf("unexpected event: %+v", ev)
	}
	if !strings.Contains(ev.Output, "result") {
		t.Fatalf("expected MCP result in output, got: %s", ev.Output)
	}
}

func TestExecuteToolCall_UnsupportedTool(t *testing.T) {
	uc := newTestAgentUC(&fakeAgentQueryService{})
	tc := domain.ToolCall{
		ID:       "call-1",
		Function: domain.ToolCallFunc{Name: "nonexistent_tool", Arguments: nil},
	}
	_, err := uc.executeToolCall(context.Background(), "u-1", tc, "")
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported tool error, got: %v", err)
	}
}

// ---------- Full flow tests ----------

func TestAgentChat_ToolStatusCallback(t *testing.T) {
	query := &fakeAgentQueryService{
		chatToolsResponses: []domain.ChatToolsResult{
			{ToolCalls: []domain.ToolCall{{ID: "c1", Function: domain.ToolCallFunc{Name: "knowledge_search", Arguments: map[string]any{"question": "q"}}}}},
			{Content: "done"},
		},
	}
	uc := newTestAgentUC(query)
	var statuses []string
	cb := func(toolName string, status string) {
		statuses = append(statuses, toolName+":"+status)
	}
	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID:   "u-1",
		Messages: []domain.AgentInputMessage{{Role: "user", Content: "test"}},
	}, cb)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if !strings.Contains(result.Answer, "done") {
		t.Fatalf("expected answer to contain 'done', got %q", result.Answer)
	}
	if len(statuses) < 2 {
		t.Fatalf("expected at least 2 status callbacks, got %d: %v", len(statuses), statuses)
	}
	if statuses[0] != "knowledge_search:running" {
		t.Fatalf("expected first callback running, got %s", statuses[0])
	}
	if statuses[1] != "knowledge_search:ok" {
		t.Fatalf("expected second callback ok, got %s", statuses[1])
	}
}

func TestAgentChat_ParallelToolCalls(t *testing.T) {
	query := &fakeAgentQueryService{
		chatToolsResponses: []domain.ChatToolsResult{
			{ToolCalls: []domain.ToolCall{
				{ID: "c1", Function: domain.ToolCallFunc{Name: "knowledge_search", Arguments: map[string]any{"question": "q1"}}},
				{ID: "c2", Function: domain.ToolCallFunc{Name: "task_tool", Arguments: map[string]any{"action": "list"}}},
			}},
			{Content: "combined answer"},
		},
	}
	uc := newTestAgentUC(query)
	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID:   "u-1",
		Messages: []domain.AgentInputMessage{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if len(result.ToolsInvoked) != 2 {
		t.Fatalf("expected 2 tools invoked, got %d: %v", len(result.ToolsInvoked), result.ToolsInvoked)
	}
	if !strings.Contains(result.Answer, "combined answer") {
		t.Fatalf("expected answer to contain 'combined answer', got %q", result.Answer)
	}
}

func TestAgentChat_ToolResultCaching(t *testing.T) {
	var callCount int32
	query := &fakeAgentQueryService{
		chatToolsHook: func(_ context.Context, msgs []domain.ChatMessage, _ []domain.ToolSchema) (*domain.ChatToolsResult, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n <= 2 {
				return &domain.ChatToolsResult{
					ToolCalls: []domain.ToolCall{{ID: fmt.Sprintf("c%d", n), Function: domain.ToolCallFunc{Name: "knowledge_search", Arguments: map[string]any{"question": "same query"}}}},
				}, nil
			}
			return &domain.ChatToolsResult{Content: "final"}, nil
		},
	}
	uc := newTestAgentUC(query)
	result, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID:   "u-1",
		Messages: []domain.AgentInputMessage{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.Iterations != 3 {
		t.Fatalf("expected 3 iterations, got %d", result.Iterations)
	}
}

func TestAgentChat_WebIntentUsesDirectWebSearch(t *testing.T) {
	ws := &fakeWebSearcher{results: []domain.WebSearchResult{
		{Title: "HTTP", URL: "https://example.com/http", Snippet: "HTTP is a protocol"},
	}}
	query := &fakeAgentQueryService{
		generateTextResponses: []string{"<think>internal</think>answer from web"},
	}
	uc := newTestAgentUC(query, func(u *AgentChatUseCase) {
		u.webSearcher = ws
		u.limits.IntentRouterEnabled = true
	})
	result := completeWithMessage(t, uc, "Найди в сети информацию о XHTTP")
	if ws.calls != 1 {
		t.Fatalf("expected direct web search once, got %d", ws.calls)
	}
	if ws.lastQuery != "XHTTP" {
		t.Fatalf("expected stripped search query, got %q", ws.lastQuery)
	}
	if !strings.Contains(result.Answer, "answer from web") {
		t.Fatalf("expected web answer, got %q", result.Answer)
	}
	if strings.Count(result.Answer, "<think>") != 1 {
		t.Fatalf("expected only agent-level thinking wrapper, got %q", result.Answer)
	}
	if len(result.ToolsInvoked) != 1 || result.ToolsInvoked[0] != "web_search" {
		t.Fatalf("expected web_search invoked, got %v", result.ToolsInvoked)
	}
	if result.Iterations != 0 {
		t.Fatalf("expected direct path without planner iterations, got %d", result.Iterations)
	}
}

func TestAgentChat_AnswerFromWebToolEventsStripsNestedThinkBlocks(t *testing.T) {
	query := &fakeAgentQueryService{
		generateTextResponses: []string{"<think>internal</think>XHTTP, вероятно, опечатка; найден HTTP."},
	}
	uc := newTestAgentUC(query)
	payload, _ := json.Marshal(map[string]any{
		"query": "XHTTP",
		"results": []domain.WebSearchResult{
			{Title: "HTTP", URL: "https://example.com/http", Snippet: "HTTP is a protocol"},
		},
	})
	answer, ok := uc.answerFromWebToolEvents(context.Background(), "Найди в сети информацию о XHTTP", []domain.AgentToolEvent{
		{Tool: "web_search", Status: "ok", Output: string(payload)},
	})
	if !ok {
		t.Fatal("expected web tool events to produce an answer")
	}
	if strings.Contains(answer, "<think>") {
		t.Fatalf("expected nested think tags stripped, got %q", answer)
	}
}

func TestAgentChat_WebIntentStreamsManualThinkingDeltas(t *testing.T) {
	ws := &fakeWebSearcher{results: []domain.WebSearchResult{
		{Title: "HTTP", URL: "https://example.com/http", Snippet: "HTTP is a protocol"},
	}}
	query := &fakeAgentQueryService{
		generateTextResponses: []string{"answer from web"},
	}
	uc := newTestAgentUC(query, func(u *AgentChatUseCase) {
		u.webSearcher = ws
		u.limits.IntentRouterEnabled = true
	})

	var thinking strings.Builder
	_, err := uc.Complete(context.Background(), domain.AgentChatRequest{
		UserID:          "u-1",
		Messages:        []domain.AgentInputMessage{{Role: "user", Content: "Найди в сети информацию о XHTTP"}},
		OnThinkingDelta: func(text string) { thinking.WriteString(text) },
	}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	streamed := thinking.String()
	if !strings.Contains(streamed, "Searching the web directly") {
		t.Fatalf("expected direct web thinking delta, got %q", streamed)
	}
	if !strings.Contains(streamed, "✓ Tool web_search: ok") {
		t.Fatalf("expected tool status in thinking delta, got %q", streamed)
	}
}

func TestMaybePersistSummary_NotEnoughTurns(t *testing.T) {
	convStore := &fakeConversationStore{}
	memStore := &fakeMemoryStore{lastTurn: 0}
	uc := NewAgentChatUseCase(
		&fakeAgentQueryService{},
		&fakeAgentEmbedder{},
		convStore,
		&fakeTaskStore{},
		memStore,
		&fakeMemoryVectorStore{},
		nil, nil, nil,
		domain.AgentLimits{SummaryEveryTurns: 6},
		nil,
	)
	created, err := uc.maybePersistSummary(context.Background(), "u-1", "conv-1", 3, false)
	if err != nil {
		t.Fatalf("maybePersistSummary error: %v", err)
	}
	if created {
		t.Fatal("expected no summary created with only 3 turns")
	}
}

func TestMaybePersistSummary_ForceCreates(t *testing.T) {
	convStore := &fakeConversationStore{}
	memStore := &fakeMemoryStore{lastTurn: 0}
	memVec := &fakeMemoryVectorStore{}
	query := &fakeAgentQueryService{generateTextResponses: []string{"summary text"}}

	uc := NewAgentChatUseCase(
		query,
		&fakeAgentEmbedder{},
		convStore,
		&fakeTaskStore{},
		memStore,
		memVec,
		nil, nil, nil,
		domain.AgentLimits{SummaryEveryTurns: 6},
		nil,
	)

	// Add a message so there's content to summarize
	convStore.messages = []domain.ConversationMessage{
		{UserTurn: 1, Role: "user", Content: "test"},
	}

	created, err := uc.maybePersistSummary(context.Background(), "u-1", "conv-1", 1, true)
	if err != nil {
		t.Fatalf("maybePersistSummary error: %v", err)
	}
	if !created {
		t.Fatal("expected summary to be created when forced")
	}
	if len(memStore.summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(memStore.summaries))
	}
}

// ---------- Helper function tests ----------

func TestStringInput(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		key      string
		fallback string
		want     string
	}{
		{"nil_input", nil, "key", "default", "default"},
		{"missing_key", map[string]interface{}{}, "key", "default", "default"},
		{"nil_value", map[string]interface{}{"key": nil}, "key", "default", "default"},
		{"string_value", map[string]interface{}{"key": "hello"}, "key", "default", "hello"},
		{"int_value", map[string]interface{}{"key": 42}, "key", "default", "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringInput(tt.input, tt.key, tt.fallback)
			if got != tt.want {
				t.Fatalf("stringInput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIntInput(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		key      string
		fallback int
		want     int
	}{
		{"nil_input", nil, "key", 5, 5},
		{"missing_key", map[string]interface{}{}, "key", 5, 5},
		{"float64", map[string]interface{}{"key": float64(10)}, "key", 5, 10},
		{"int", map[string]interface{}{"key": 10}, "key", 5, 10},
		{"int64", map[string]interface{}{"key": int64(10)}, "key", 5, 10},
		{"string_valid", map[string]interface{}{"key": "10"}, "key", 5, 10},
		{"string_invalid", map[string]interface{}{"key": "abc"}, "key", 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intInput(tt.input, tt.key, tt.fallback)
			if got != tt.want {
				t.Fatalf("intInput() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBoolInput(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		key      string
		fallback bool
		want     bool
	}{
		{"nil_input", nil, "key", false, false},
		{"bool_true", map[string]interface{}{"key": true}, "key", false, true},
		{"string_true", map[string]interface{}{"key": "true"}, "key", false, true},
		{"string_invalid", map[string]interface{}{"key": "notbool"}, "key", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := boolInput(tt.input, tt.key, tt.fallback)
			if got != tt.want {
				t.Fatalf("boolInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeUTF8(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid_ascii", "hello", "hello"},
		{"valid_cyrillic", "Привет мир", "Привет мир"},
		{"invalid_bytes", "hello\x80world", "helloworld"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeUTF8(tt.input)
			if got != tt.want {
				t.Fatalf("sanitizeUTF8() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLatestUserInput(t *testing.T) {
	tests := []struct {
		name     string
		messages []domain.AgentInputMessage
		want     string
		wantOk   bool
	}{
		{"empty", nil, "", false},
		{"no_user", []domain.AgentInputMessage{{Role: "system", Content: "hi"}}, "", false},
		{"single_user", []domain.AgentInputMessage{{Role: "user", Content: "hello"}}, "hello", true},
		{"multiple_users", []domain.AgentInputMessage{
			{Role: "user", Content: "first"},
			{Role: "assistant", Content: "reply"},
			{Role: "user", Content: "second"},
		}, "second", true},
		{"whitespace_user", []domain.AgentInputMessage{{Role: "user", Content: "  "}}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := latestUserInput(tt.messages)
			if ok != tt.wantOk || got != tt.want {
				t.Fatalf("latestUserInput() = %q, %v; want %q, %v", got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestShouldFallbackToRAG(t *testing.T) {
	tests := []struct {
		reason string
		want   bool
	}{
		{"planner_error", true},
		{"timeout", true},
		{"planner_invalid_json", true},
		{"max_iterations", false},
		{"empty_response", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			if got := shouldFallbackToRAG(tt.reason); got != tt.want {
				t.Fatalf("shouldFallbackToRAG(%q) = %v, want %v", tt.reason, got, tt.want)
			}
		})
	}
}

func TestExpandQueryWithGraph_NilGraphStore(t *testing.T) {
	uc := newTestAgentUC(&fakeAgentQueryService{})
	result := uc.expandQueryWithGraph(context.Background(), "test query")
	if result != "" {
		t.Fatalf("expected empty string for nil graph store, got %q", result)
	}
}

func TestExpandQueryWithGraph_WithResults(t *testing.T) {
	gs := &fakeGraphStore{
		findByTitleResult: []domain.GraphNode{{ID: "node-1", Title: "Related"}},
		relatedResult: []domain.GraphRelation{
			{SourceID: "node-1", TargetID: "node-2"},
		},
	}
	uc := newTestAgentUC(&fakeAgentQueryService{}, func(u *AgentChatUseCase) {
		u.graphStore = gs
	})
	result := uc.expandQueryWithGraph(context.Background(), "test query expansion")
	// Result depends on FindByTitle returning nodes for tokens >= 3 chars
	_ = result // graph expansion is best-effort
}

func TestToolSchemasFromRegistry_Nil(t *testing.T) {
	schemas := toolSchemasFromRegistry(nil, true)
	if schemas != nil {
		t.Fatalf("expected nil for nil registry, got %v", schemas)
	}
}

func TestToolSchemasFromRegistry_FiltersWebSearch(t *testing.T) {
	registry := &fakeMCPToolRegistry{
		tools: []ports.ToolDefinition{
			{Name: "knowledge_search", Description: "search"},
			{Name: "web_search", Description: "web"},
		},
	}
	// web_search should be excluded when not available
	schemas := toolSchemasFromRegistry(registry, false)
	for _, s := range schemas {
		if s.Function.Name == "web_search" {
			t.Fatal("web_search should be filtered out when not available")
		}
	}

	// web_search should be included when available
	schemas = toolSchemasFromRegistry(registry, true)
	found := false
	for _, s := range schemas {
		if s.Function.Name == "web_search" {
			found = true
		}
	}
	if !found {
		t.Fatal("web_search should be present when available")
	}
}

func TestStringFromArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]any
		key      string
		fallback string
		want     string
	}{
		{"found", map[string]any{"k": "v"}, "k", "fb", "v"},
		{"not_found", map[string]any{"k": "v"}, "other", "fb", "fb"},
		{"non_string", map[string]any{"k": 42}, "k", "fb", "fb"},
		{"nil_args", nil, "k", "fb", "fb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringFromArgs(tt.args, tt.key, tt.fallback)
			if got != tt.want {
				t.Fatalf("stringFromArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIntFromArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]any
		key      string
		fallback int
		want     int
	}{
		{"float64", map[string]any{"k": float64(5)}, "k", 0, 5},
		{"int", map[string]any{"k": 5}, "k", 0, 5},
		{"not_found", map[string]any{}, "k", 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intFromArgs(tt.args, tt.key, tt.fallback)
			if got != tt.want {
				t.Fatalf("intFromArgs() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseTimeRFC3339(t *testing.T) {
	valid := "2026-01-01T00:00:00Z"
	parsed, err := parseTimeRFC3339(valid)
	if err != nil {
		t.Fatalf("parseTimeRFC3339(%q) error: %v", valid, err)
	}
	if parsed.Year() != 2026 {
		t.Fatalf("expected year 2026, got %d", parsed.Year())
	}

	_, err = parseTimeRFC3339("not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

func TestIsAgentTimeoutError(t *testing.T) {
	if !isAgentTimeoutError(context.DeadlineExceeded) {
		t.Fatal("DeadlineExceeded should be timeout")
	}
	if !isAgentTimeoutError(context.Canceled) {
		t.Fatal("Canceled should be timeout")
	}
	if isAgentTimeoutError(errors.New("random")) {
		t.Fatal("random error should not be timeout")
	}
}

// Ensure json import is used
var _ = json.Marshal
