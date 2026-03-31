package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// ---------------------------------------------------------------------------
// Fake stores
// ---------------------------------------------------------------------------

type fakeGraphStore struct {
	graph *domain.Graph
	err   error
}

func (f *fakeGraphStore) UpsertDocument(context.Context, domain.GraphNode) error { return nil }
func (f *fakeGraphStore) AddLink(context.Context, string, string, string) error  { return nil }
func (f *fakeGraphStore) AddSimilarity(context.Context, string, string, float64) error {
	return nil
}
func (f *fakeGraphStore) RemoveSimilarities(context.Context, string) error { return nil }
func (f *fakeGraphStore) GetRelated(context.Context, string, int, int) ([]domain.GraphRelation, error) {
	return nil, nil
}
func (f *fakeGraphStore) FindByID(context.Context, string) (*domain.GraphNode, error) {
	return nil, nil
}
func (f *fakeGraphStore) FindByTitle(context.Context, string) ([]domain.GraphNode, error) {
	return nil, nil
}
func (f *fakeGraphStore) GetGraph(_ context.Context, _ domain.GraphFilter) (*domain.Graph, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.graph, nil
}

type fakeFeedbackStore struct {
	created []*domain.AgentFeedback
	err     error
}

func (f *fakeFeedbackStore) Create(_ context.Context, fb *domain.AgentFeedback) error {
	if f.err != nil {
		return f.err
	}
	fb.ID = "fb-1"
	fb.CreatedAt = time.Now()
	f.created = append(f.created, fb)
	return nil
}
func (f *fakeFeedbackStore) ListRecent(context.Context, time.Time, int) ([]domain.AgentFeedback, error) {
	return nil, nil
}
func (f *fakeFeedbackStore) CountByRating(context.Context, time.Time) (map[string]int, error) {
	return nil, nil
}

type fakeScheduleStore struct {
	tasks   []domain.ScheduledTask
	created []*domain.ScheduledTask
	err     error
}

func (f *fakeScheduleStore) Create(_ context.Context, task *domain.ScheduledTask) error {
	if f.err != nil {
		return f.err
	}
	task.ID = "sched-1"
	f.created = append(f.created, task)
	return nil
}
func (f *fakeScheduleStore) ListByUser(_ context.Context, _ string) ([]domain.ScheduledTask, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tasks, nil
}
func (f *fakeScheduleStore) ListEnabled(context.Context) ([]domain.ScheduledTask, error) {
	return f.tasks, nil
}
func (f *fakeScheduleStore) GetByID(_ context.Context, id string) (*domain.ScheduledTask, error) {
	for i := range f.tasks {
		if f.tasks[i].ID == id {
			cp := f.tasks[i]
			return &cp, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeScheduleStore) Update(_ context.Context, task *domain.ScheduledTask) error {
	for i := range f.tasks {
		if f.tasks[i].ID == task.ID {
			f.tasks[i] = *task
			return nil
		}
	}
	return nil
}
func (f *fakeScheduleStore) Delete(_ context.Context, id string) error {
	out := f.tasks[:0]
	for _, t := range f.tasks {
		if t.ID != id {
			out = append(out, t)
		}
	}
	f.tasks = out
	return nil
}
func (f *fakeScheduleStore) RecordRun(context.Context, string, string, string) error { return nil }

type fakeRuntimeModelConfig struct {
	config ports.RuntimeModelConfig
	err    error
}

func (f *fakeRuntimeModelConfig) GetRuntimeModelConfig() ports.RuntimeModelConfig {
	return f.config
}

func (f *fakeRuntimeModelConfig) SetRuntimeModelConfig(config ports.RuntimeModelConfig) error {
	if f.err != nil {
		return f.err
	}
	f.config = config
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newRouterWithStores creates a minimal Router with the given stores wired in.
func newRouterWithStores(gs *fakeGraphStore, fs *fakeFeedbackStore, ss *fakeScheduleStore) *Router {
	rt := &Router{}
	if gs != nil {
		rt.graphStore = gs
	}
	if fs != nil {
		rt.feedbackStore = fs
	}
	if ss != nil {
		rt.scheduleStore = ss
	}
	return rt
}

func TestHandleGetRuntimeModels_Success(t *testing.T) {
	rt := &Router{
		runtimeModelConfig: &fakeRuntimeModelConfig{
			config: ports.RuntimeModelConfig{
				GenerationModel: "qwen3.5:9b",
				PlannerModel:    "qwen3.5:9b",
				EmbeddingModel:  "bge-m3:latest",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/settings/models", nil)
	rec := httptest.NewRecorder()
	rt.handleGetRuntimeModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp runtimeModelsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.GenModel != "qwen3.5:9b" {
		t.Fatalf("expected gen model to match, got %q", resp.GenModel)
	}
	if !resp.RuntimeApplySupported {
		t.Fatalf("expected runtime apply to be supported")
	}
}

func TestHandlePutRuntimeModels_Success(t *testing.T) {
	runtimeCfg := &fakeRuntimeModelConfig{
		config: ports.RuntimeModelConfig{
			GenerationModel: "old-gen",
			PlannerModel:    "old-plan",
			EmbeddingModel:  "old-embed",
		},
	}
	rt := &Router{runtimeModelConfig: runtimeCfg}

	body := bytes.NewReader([]byte(`{"gen_model":"new-gen","planner_model":"new-plan","embedding_model":"new-embed"}`))
	req := httptest.NewRequest(http.MethodPut, "/v1/settings/models", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	rt.handlePutRuntimeModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if runtimeCfg.config.GenerationModel != "new-gen" {
		t.Fatalf("expected applied generation model, got %q", runtimeCfg.config.GenerationModel)
	}
	if runtimeCfg.config.PlannerModel != "new-plan" {
		t.Fatalf("expected applied planner model, got %q", runtimeCfg.config.PlannerModel)
	}
	if runtimeCfg.config.EmbeddingModel != "new-embed" {
		t.Fatalf("expected applied embedding model, got %q", runtimeCfg.config.EmbeddingModel)
	}
}

// ---------------------------------------------------------------------------
// Graph handler tests
// ---------------------------------------------------------------------------

func TestHandleGetGraph_Success(t *testing.T) {
	gs := &fakeGraphStore{
		graph: &domain.Graph{
			Nodes: []domain.GraphNode{
				{ID: "n1", Filename: "a.md", SourceType: "obsidian", Title: "Note A"},
				{ID: "n2", Filename: "b.md", SourceType: "obsidian", Title: "Note B"},
			},
			Edges: []domain.GraphRelation{
				{SourceID: "n1", TargetID: "n2", Type: "wikilink", Weight: 1.0},
			},
		},
	}
	rt := newRouterWithStores(gs, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/graph", nil)
	rec := httptest.NewRecorder()
	rt.handleGetGraph(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var graph domain.Graph
	if err := json.NewDecoder(rec.Body).Decode(&graph); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}
	if graph.Edges[0].Type != "wikilink" {
		t.Fatalf("expected edge type wikilink, got %s", graph.Edges[0].Type)
	}
}

// ---------------------------------------------------------------------------
// Feedback handler tests
// ---------------------------------------------------------------------------

func TestHandlePostFeedback_Success(t *testing.T) {
	fs := &fakeFeedbackStore{}
	rt := newRouterWithStores(nil, fs, nil)

	body, _ := json.Marshal(map[string]string{
		"conversation_id": "conv-1",
		"message_id":      "msg-1",
		"rating":          "up",
		"comment":         "great answer",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-42")
	rec := httptest.NewRecorder()
	rt.handlePostFeedback(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected status=ok, got %s", resp["status"])
	}
	if resp["id"] != "fb-1" {
		t.Fatalf("expected id=fb-1, got %s", resp["id"])
	}
	if len(fs.created) != 1 {
		t.Fatalf("expected 1 feedback created, got %d", len(fs.created))
	}
	if fs.created[0].UserID != "user-42" {
		t.Fatalf("expected user_id=user-42, got %s", fs.created[0].UserID)
	}
}

func TestHandlePostFeedback_InvalidJSON(t *testing.T) {
	fs := &fakeFeedbackStore{}
	rt := newRouterWithStores(nil, fs, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/feedback", bytes.NewReader([]byte(`{bad json`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	rt.handlePostFeedback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Schedule handler tests
// ---------------------------------------------------------------------------

func TestHandleCreateSchedule_Success(t *testing.T) {
	ss := &fakeScheduleStore{}
	rt := newRouterWithStores(nil, nil, ss)

	body, _ := json.Marshal(map[string]string{
		"cron_expr":   "0 9 * * *",
		"prompt":      "daily summary",
		"webhook_url": "https://example.com/hook",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/schedules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	rec := httptest.NewRecorder()
	rt.handleCreateSchedule(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var task domain.ScheduledTask
	if err := json.NewDecoder(rec.Body).Decode(&task); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if task.ID != "sched-1" {
		t.Fatalf("expected id=sched-1, got %s", task.ID)
	}
	if task.CronExpr != "0 9 * * *" {
		t.Fatalf("expected cron_expr='0 9 * * *', got %s", task.CronExpr)
	}
	if !task.Enabled {
		t.Fatal("expected task to be enabled by default")
	}
}

func TestHandleListSchedules_Success(t *testing.T) {
	ss := &fakeScheduleStore{
		tasks: []domain.ScheduledTask{
			{ID: "s1", UserID: "user-1", CronExpr: "0 9 * * *", Prompt: "morning", Enabled: true},
			{ID: "s2", UserID: "user-1", CronExpr: "0 18 * * *", Prompt: "evening", Enabled: false},
		},
	}
	rt := newRouterWithStores(nil, nil, ss)

	req := httptest.NewRequest(http.MethodGet, "/v1/schedules", nil)
	req.Header.Set("X-User-ID", "user-1")
	rec := httptest.NewRecorder()
	rt.handleListSchedules(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var tasks []domain.ScheduledTask
	if err := json.NewDecoder(rec.Body).Decode(&tasks); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestHandleDeleteSchedule_Success(t *testing.T) {
	ss := &fakeScheduleStore{
		tasks: []domain.ScheduledTask{
			{ID: "s1", UserID: "user-1", CronExpr: "0 9 * * *", Prompt: "morning", Enabled: true},
		},
	}
	rt := newRouterWithStores(nil, nil, ss)

	req := httptest.NewRequest(http.MethodDelete, "/v1/schedules/s1", nil)
	req.SetPathValue("id", "s1")
	rec := httptest.NewRecorder()
	rt.handleDeleteSchedule(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(ss.tasks) != 0 {
		t.Fatalf("expected 0 tasks after delete, got %d", len(ss.tasks))
	}
}

func TestHandleUpdateSchedule_Success(t *testing.T) {
	ss := &fakeScheduleStore{
		tasks: []domain.ScheduledTask{
			{ID: "s1", UserID: "user-1", CronExpr: "0 9 * * *", Prompt: "morning", Enabled: true},
		},
	}
	rt := newRouterWithStores(nil, nil, ss)

	newPrompt := "updated prompt"
	enabled := false
	body, _ := json.Marshal(map[string]interface{}{
		"prompt":  newPrompt,
		"enabled": enabled,
	})

	req := httptest.NewRequest(http.MethodPatch, "/v1/schedules/s1", bytes.NewReader(body))
	req.SetPathValue("id", "s1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	rt.handleUpdateSchedule(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var task domain.ScheduledTask
	if err := json.NewDecoder(rec.Body).Decode(&task); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if task.Prompt != "updated prompt" {
		t.Fatalf("expected prompt='updated prompt', got %s", task.Prompt)
	}
	if task.Enabled {
		t.Fatal("expected task to be disabled after update")
	}
	// Verify the store was also updated.
	if ss.tasks[0].Prompt != "updated prompt" {
		t.Fatalf("store not updated: prompt=%s", ss.tasks[0].Prompt)
	}
}
