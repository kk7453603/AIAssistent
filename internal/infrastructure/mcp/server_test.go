package mcp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// ---------- Fakes ----------

type fakeQuerySvc struct {
	answerText string
	answerErr  error
}

func (f *fakeQuerySvc) Answer(_ context.Context, _ string, _ int, _ domain.SearchFilter) (*domain.Answer, error) {
	if f.answerErr != nil {
		return nil, f.answerErr
	}
	return &domain.Answer{Text: f.answerText}, nil
}
func (f *fakeQuerySvc) GenerateFromPrompt(context.Context, string) (string, error) {
	return "", nil
}
func (f *fakeQuerySvc) GenerateJSONFromPrompt(context.Context, string) (string, error) {
	return "", nil
}
func (f *fakeQuerySvc) ChatWithTools(context.Context, []domain.ChatMessage, []domain.ToolSchema) (*domain.ChatToolsResult, error) {
	return nil, nil
}

type fakeWebSearcher struct {
	results []domain.WebSearchResult
	err     error
}

func (f *fakeWebSearcher) Search(_ context.Context, _ string, _ int) ([]domain.WebSearchResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

type fakeObsidianWriter struct {
	path string
	err  error
}

func (f *fakeObsidianWriter) CreateNote(_ context.Context, _, _, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.path, nil
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

func (f *fakeTaskStore) ListTasks(_ context.Context, userID string, _ bool) ([]domain.Task, error) {
	var out []domain.Task
	for _, t := range f.tasks {
		if t.UserID == userID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeTaskStore) GetTaskByID(_ context.Context, userID, taskID string) (*domain.Task, error) {
	t, ok := f.tasks[taskID]
	if !ok || t.UserID != userID {
		return nil, fmt.Errorf("not found")
	}
	return &t, nil
}

func (f *fakeTaskStore) UpdateTask(_ context.Context, task *domain.Task) error {
	f.tasks[task.ID] = *task
	return nil
}

func (f *fakeTaskStore) SoftDeleteTask(_ context.Context, _, taskID string) error {
	t, ok := f.tasks[taskID]
	if !ok {
		return fmt.Errorf("not found")
	}
	now := time.Now().UTC()
	t.DeletedAt = &now
	f.tasks[taskID] = t
	return nil
}

// ---------- Tests ----------

func TestNewMCPHandler_NotNil(t *testing.T) {
	handler := NewMCPHandler(ServerDeps{
		QuerySvc:      &fakeQuerySvc{answerText: "test"},
		WebSearcher:   &fakeWebSearcher{},
		ObsidianWriter: &fakeObsidianWriter{path: "/test"},
		Tasks:         &fakeTaskStore{},
		KnowledgeTopK: 5,
	})
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNewMCPHandler_NilWebSearcher(t *testing.T) {
	// Should not panic when WebSearcher is nil (web_search tool just won't be registered)
	handler := NewMCPHandler(ServerDeps{
		QuerySvc:      &fakeQuerySvc{answerText: "test"},
		WebSearcher:   nil,
		ObsidianWriter: nil,
		Tasks:         &fakeTaskStore{},
		KnowledgeTopK: 5,
	})
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestMarshalResult(t *testing.T) {
	result, err := marshalResult(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("marshalResult error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == "" {
		t.Fatal("expected non-empty ID")
	}
	if id1 == id2 {
		t.Fatal("expected unique IDs")
	}
}

// Verify ServerDeps implements expected interfaces
var _ ports.DocumentQueryService = (*fakeQuerySvc)(nil)
var _ ports.WebSearcher = (*fakeWebSearcher)(nil)
var _ ports.ObsidianNoteWriter = (*fakeObsidianWriter)(nil)
var _ ports.TaskStore = (*fakeTaskStore)(nil)
