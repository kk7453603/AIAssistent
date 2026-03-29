package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
	"github.com/kirillkom/personal-ai-assistant/internal/config"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
	paamcp "github.com/kirillkom/personal-ai-assistant/internal/infrastructure/mcp"
	"github.com/kirillkom/personal-ai-assistant/internal/observability/metrics"
	"golang.org/x/time/rate"
)

type Router struct {
	ingestor ports.DocumentIngestor
	querySvc ports.DocumentQueryService
	docs     ports.DocumentReader
	agentSvc ports.AgentChatService

	openAICompatAPIKey           string
	openAICompatModelID          string
	modelProviderMap             map[string]string // model ID → provider name (e.g., "paa-huggingface" → "huggingface")
	openAICompatContextMessages  int
	openAICompatStreamChunkChars int
	toolTriggerKeywords          []string
	ragTopK                      int
	agentModeEnabled             bool
	httpMetrics                  *metrics.HTTPServerMetrics
	apiRateLimiter               *rate.Limiter
	apiBackpressureMaxInFlight   int
	apiBackpressureWaitTimeout   time.Duration

	obsidianConfigPath             string
	obsidianStateDir               string
	obsidianVaultsRoot             string
	obsidianDefaultIntervalMinutes int
	obsidianSyncTimeout            time.Duration
	obsidianSyncPoll               time.Duration
	obsidianMu                     sync.Mutex

	mcpHandler   http.Handler
	httpToolDefs []paamcp.HTTPToolDef

	graphStore       ports.GraphStore
	feedbackStore    ports.FeedbackStore
	eventStore       ports.EventStore
	improvementStore ports.ImprovementStore
	scheduleStore    ports.ScheduleStore
	docRepo          ports.DocumentRepository
	objectStorage    ports.ObjectStorage
}

func NewRouter(
	cfg config.Config,
	ingestor ports.DocumentIngestor,
	querySvc ports.DocumentQueryService,
	docs ports.DocumentReader,
	agentSvc ports.AgentChatService,
	modelProviderMap map[string]string,
) *Router {
	contextMessages := cfg.OpenAICompatContextMessages
	if contextMessages <= 0 {
		contextMessages = 5
	}
	streamChunkChars := cfg.OpenAICompatStreamChunkChars
	if streamChunkChars <= 0 {
		streamChunkChars = 120
	}
	ragTopK := cfg.RAGTopK
	if ragTopK <= 0 {
		ragTopK = 5
	}
	toolKeywords := make([]string, 0)
	for keyword := range strings.SplitSeq(cfg.OpenAICompatToolTriggerKeywords, ",") {
		keyword = strings.TrimSpace(strings.ToLower(keyword))
		if keyword == "" {
			continue
		}
		toolKeywords = append(toolKeywords, keyword)
	}
	var apiRateLimiter *rate.Limiter
	apiRateLimitRPS := cfg.APIRateLimitRPS
	apiRateLimitBurst := cfg.APIRateLimitBurst
	if apiRateLimitRPS > 0 && apiRateLimitBurst <= 0 {
		apiRateLimitBurst = max(int(math.Ceil(apiRateLimitRPS)), 1)
	}
	if apiRateLimitRPS > 0 && apiRateLimitBurst > 0 {
		apiRateLimiter = rate.NewLimiter(rate.Limit(apiRateLimitRPS), apiRateLimitBurst)
	}
	apiBackpressureWait := time.Duration(cfg.APIBackpressureWaitMS) * time.Millisecond
	if cfg.APIBackpressureWaitMS < 0 {
		apiBackpressureWait = 0
	}
	obsidianDefaultInterval := cfg.ObsidianDefaultIntervalMinutes
	if obsidianDefaultInterval <= 0 {
		obsidianDefaultInterval = 15
	}
	obsidianSyncTimeout := time.Duration(cfg.ObsidianSyncTimeoutSeconds) * time.Second
	if obsidianSyncTimeout <= 0 {
		obsidianSyncTimeout = 120 * time.Second
	}
	obsidianSyncPoll := time.Duration(cfg.ObsidianSyncPollSeconds) * time.Second
	if obsidianSyncPoll <= 0 {
		obsidianSyncPoll = 2 * time.Second
	}

	return &Router{
		ingestor: ingestor,
		querySvc: querySvc,
		docs:     docs,
		agentSvc: agentSvc,

		openAICompatAPIKey:           cfg.OpenAICompatAPIKey,
		openAICompatModelID:          cfg.OpenAICompatModelID,
		modelProviderMap:             modelProviderMap,
		openAICompatContextMessages:  contextMessages,
		openAICompatStreamChunkChars: streamChunkChars,
		toolTriggerKeywords:          toolKeywords,
		ragTopK:                      ragTopK,
		agentModeEnabled:             cfg.AgentModeEnabled,
		httpMetrics:                  metrics.NewHTTPServerMetrics("api"),
		apiRateLimiter:               apiRateLimiter,
		apiBackpressureMaxInFlight:   cfg.APIBackpressureMaxInFlight,
		apiBackpressureWaitTimeout:   apiBackpressureWait,

		obsidianConfigPath:             cfg.ObsidianConfigPath,
		obsidianStateDir:               cfg.ObsidianStateDir,
		obsidianVaultsRoot:             cfg.ObsidianVaultsRoot,
		obsidianDefaultIntervalMinutes: obsidianDefaultInterval,
		obsidianSyncTimeout:            obsidianSyncTimeout,
		obsidianSyncPoll:               obsidianSyncPoll,
	}
}

// SetMCPHandler sets the MCP server handler to be mounted on /mcp.
func (rt *Router) SetMCPHandler(h http.Handler) {
	rt.mcpHandler = h
}

// SetGraphStore sets the graph store used by the GET /v1/graph endpoint.
func (rt *Router) SetGraphStore(g ports.GraphStore) {
	rt.graphStore = g
}

// SetFeedbackStore sets the feedback store used by the POST /v1/feedback endpoint.
func (rt *Router) SetFeedbackStore(f ports.FeedbackStore) {
	rt.feedbackStore = f
}

// SetEventStore sets the event store used by the GET /v1/events/summary endpoint.
func (rt *Router) SetEventStore(e ports.EventStore) {
	rt.eventStore = e
}

// SetImprovementStore sets the improvement store used by the /v1/improvements endpoints.
func (rt *Router) SetImprovementStore(i ports.ImprovementStore) {
	rt.improvementStore = i
}

// SetScheduleStore sets the schedule store used by the /v1/schedules endpoints.
func (rt *Router) SetScheduleStore(s ports.ScheduleStore) {
	rt.scheduleStore = s
}

// SetDocumentRepository sets the document repository used by the GET /v1/documents endpoint.
func (rt *Router) SetDocumentRepository(r ports.DocumentRepository) {
	rt.docRepo = r
}

// SetObjectStorage sets the object storage for document content endpoint.
func (rt *Router) SetObjectStorage(s ports.ObjectStorage) {
	rt.objectStorage = s
}

// SetHTTPToolDefs stores the list of HTTP tool definitions for the GET /v1/tools endpoint.
func (rt *Router) SetHTTPToolDefs(defs []paamcp.HTTPToolDef) {
	rt.httpToolDefs = defs
}

func (rt *Router) handleListTools(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if rt.httpToolDefs == nil {
		_ = json.NewEncoder(w).Encode([]any{})
		return
	}
	_ = json.NewEncoder(w).Encode(rt.httpToolDefs)
}

func (rt *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /openapi.json", serveOpenAPISpecJSON)
	mux.Handle("GET /metrics", rt.httpMetrics.Handler())
	mux.HandleFunc("GET /ui/obsidian", rt.handleObsidianUI)
	mux.HandleFunc("GET /ui/obsidian/", rt.handleObsidianUI)
	mux.HandleFunc("GET /v1/obsidian/find", rt.handleObsidianFindFile)
	mux.HandleFunc("GET /v1/obsidian/vaults", rt.handleObsidianList)
	mux.HandleFunc("POST /v1/obsidian/vaults", rt.handleObsidianUpsert)
	mux.HandleFunc("DELETE /v1/obsidian/vaults/{id}", rt.handleObsidianRemove)
	mux.HandleFunc("POST /v1/obsidian/vaults/{id}/sync", rt.handleObsidianSync)
	mux.HandleFunc("POST /v1/obsidian/vaults/{id}/notes", rt.handleObsidianCreateNote)
	mux.HandleFunc("GET /v1/obsidian/vaults/{id}/files", rt.handleObsidianListFiles)
	mux.HandleFunc("GET /v1/obsidian/vaults/{id}/files/content", rt.handleObsidianFileContent)

	mux.HandleFunc("GET /v1/graph", rt.handleGetGraph)
	mux.HandleFunc("POST /v1/feedback", rt.handlePostFeedback)
	mux.HandleFunc("GET /v1/events/summary", rt.handleGetEventsSummary)
	mux.HandleFunc("GET /v1/feedback/summary", rt.handleGetFeedbackSummary)
	mux.HandleFunc("GET /v1/improvements", rt.handleGetImprovements)
	mux.HandleFunc("PATCH /v1/improvements/{id}", rt.handlePatchImprovement)

	mux.HandleFunc("POST /v1/schedules", rt.handleCreateSchedule)
	mux.HandleFunc("GET /v1/schedules", rt.handleListSchedules)
	mux.HandleFunc("DELETE /v1/schedules/{id}", rt.handleDeleteSchedule)
	mux.HandleFunc("PATCH /v1/schedules/{id}", rt.handleUpdateSchedule)

	mux.HandleFunc("GET /v1/documents", rt.handleListDocuments)
	mux.HandleFunc("GET /v1/documents/{id}/content", rt.handleGetDocumentContent)

	mux.HandleFunc("GET /v1/tools", rt.handleListTools)

	if rt.mcpHandler != nil {
		mux.Handle("/mcp", rt.mcpHandler)
	}

	strict := apigen.NewStrictHandlerWithOptions(rt, []apigen.StrictMiddlewareFunc{
		rt.openAICompatAuthMiddleware,
	}, apigen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusBadRequest, err)
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			if isClientDisconnectError(err) {
				return
			}
			writeError(w, mapErrorToHTTPStatus(err), err)
		},
	})

	handler := apigen.HandlerWithOptions(strict, apigen.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusBadRequest, err)
		},
	})
	handler = backpressureMiddleware(handler, rt.apiBackpressureMaxInFlight, rt.apiBackpressureWaitTimeout)
	handler = rateLimitMiddleware(handler, rt.apiRateLimiter)
	handler = rt.httpMetrics.Middleware("api", handler)
	handler = corsMiddleware(handler)
	handler = accessLogMiddleware(handler)
	handler = requestIDMiddleware(handler)
	return handler
}

var _ apigen.StrictServerInterface = (*Router)(nil)

func (rt *Router) Healthz(_ context.Context, _ apigen.HealthzRequestObject) (apigen.HealthzResponseObject, error) {
	return apigen.Healthz200JSONResponse{
		Status: "ok",
	}, nil
}

func (rt *Router) UploadDocument(ctx context.Context, request apigen.UploadDocumentRequestObject) (apigen.UploadDocumentResponseObject, error) {
	part, err := getMultipartFilePart(request.Body, "file")
	if err != nil {
		return apigen.UploadDocument400JSONResponse{
			Error: err.Error(),
		}, nil
	}
	defer func() { _ = part.Close() }()

	filename := part.FileName()
	if filename == "" {
		filename = "document.bin"
	}

	doc, err := rt.ingestor.Upload(
		ctx,
		filename,
		part.Header.Get("Content-Type"),
		part,
	)
	if err != nil {
		switch status := mapErrorToHTTPStatus(err); status {
		case http.StatusBadRequest:
			return apigen.UploadDocument400JSONResponse{Error: err.Error()}, nil
		case http.StatusServiceUnavailable:
			return apigen.UploadDocument503JSONResponse{Error: err.Error()}, nil
		}
		return apigen.UploadDocument500JSONResponse{
			Error: err.Error(),
		}, nil
	}

	return apigen.UploadDocument202JSONResponse(toAPIDocument(doc)), nil
}

func (rt *Router) GetDocumentById(ctx context.Context, request apigen.GetDocumentByIdRequestObject) (apigen.GetDocumentByIdResponseObject, error) {
	doc, err := rt.docs.GetByID(ctx, request.DocumentId)
	if err != nil {
		// Spec currently defines only 404 for this endpoint.
		return apigen.GetDocumentById404JSONResponse{
			Error: err.Error(),
		}, nil
	}

	return apigen.GetDocumentById200JSONResponse(toAPIDocument(doc)), nil
}

func (rt *Router) QueryRag(ctx context.Context, request apigen.QueryRagRequestObject) (apigen.QueryRagResponseObject, error) {
	if request.Body == nil || strings.TrimSpace(request.Body.Question) == "" {
		return apigen.QueryRag400JSONResponse{
			Error: "question is required",
		}, nil
	}

	limit := 0
	if request.Body.Limit != nil {
		limit = *request.Body.Limit
	}

	filter := domain.SearchFilter{}
	if request.Body.Category != nil {
		filter.Categories = []string{*request.Body.Category}
	}

	start := time.Now()
	answer, err := rt.querySvc.Answer(ctx, request.Body.Question, limit, filter)
	if err != nil {
		switch status := mapErrorToHTTPStatus(err); status {
		case http.StatusBadRequest:
			return apigen.QueryRag400JSONResponse{Error: err.Error()}, nil
		case http.StatusServiceUnavailable:
			return apigen.QueryRag503JSONResponse{Error: err.Error()}, nil
		}
		return apigen.QueryRag500JSONResponse{
			Error: err.Error(),
		}, nil
	}

	rt.httpMetrics.RecordRAGObservation("api", "query_rag", len(answer.Sources), time.Since(start))
	mode := string(answer.Retrieval.Mode)
	if mode == "" {
		mode = string(domain.RetrievalModeSemantic)
	}
	rt.httpMetrics.RecordRAGModeRequest("api", "query_rag", mode)
	rt.httpMetrics.RecordTokenUsage(
		"api",
		"query_rag",
		"rag-backend",
		estimateTokenCount(request.Body.Question),
		estimateTokenCount(answer.Text),
	)
	slog.Info("rag_retrieval",
		"request_id", requestIDFromContext(ctx),
		"endpoint", "query_rag",
		"retrieval_mode", mode,
		"semantic_candidates", answer.Retrieval.SemanticCandidates,
		"lexical_candidates", answer.Retrieval.LexicalCandidates,
		"rerank_applied", answer.Retrieval.RerankApplied,
		"retrieved_chunks", len(answer.Sources),
		"duration_ms", float64(time.Since(start).Microseconds())/1000.0,
	)

	return apigen.QueryRag200JSONResponse{
		Text:    answer.Text,
		Sources: toAPIRetrievedChunks(answer.Sources),
	}, nil
}

func getMultipartFilePart(reader *multipart.Reader, formField string) (*multipart.Part, error) {
	if reader == nil {
		return nil, errors.New("multipart field 'file' is required")
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("multipart field 'file' is required")
		}
		if err != nil {
			return nil, err
		}
		if part.FormName() == formField {
			return part, nil
		}
		_ = part.Close()
	}
}

func toAPIDocument(doc *domain.Document) apigen.Document {
	return apigen.Document{
		Id:          doc.ID,
		Filename:    doc.Filename,
		MimeType:    doc.MimeType,
		StoragePath: doc.StoragePath,
		Category:    nilIfEmpty(doc.Category),
		Subcategory: nilIfEmpty(doc.Subcategory),
		Tags:        nilIfEmptySlice(doc.Tags),
		Confidence:  nilIfZero(doc.Confidence),
		Summary:     nilIfEmpty(doc.Summary),
		Status:      apigen.DocumentStatus(doc.Status),
		Error:       nilIfEmpty(doc.Error),
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.UpdatedAt,
	}
}

func toAPIRetrievedChunks(chunks []domain.RetrievedChunk) []apigen.RetrievedChunk {
	out := make([]apigen.RetrievedChunk, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, apigen.RetrievedChunk{
			DocumentId: chunk.DocumentID,
			Filename:   chunk.Filename,
			Category:   chunk.Category,
			Text:       chunk.Text,
			Score:      chunk.Score,
		})
	}
	return out
}

func nilIfEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func nilIfZero(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}

func nilIfEmptySlice(v []string) *[]string {
	if len(v) == 0 {
		return nil
	}
	out := make([]string, len(v))
	copy(out, v)
	return &out
}

func writeError(w http.ResponseWriter, status int, err error) {
	msg := "internal server error"
	if err != nil && err.Error() != "" {
		msg = err.Error()
	}
	resp := apigen.ErrorResponse{Error: msg}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func isClientDisconnectError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "use of closed network connection")
}

func (rt *Router) handleGetGraph(w http.ResponseWriter, r *http.Request) {
	filter := domain.GraphFilter{
		MaxDepth: 2,
		MinScore: 0.5,
	}
	if st := r.URL.Query().Get("source_types"); st != "" {
		filter.SourceTypes = strings.Split(st, ",")
	}
	if ms := r.URL.Query().Get("min_score"); ms != "" {
		if v, err := strconv.ParseFloat(ms, 64); err == nil {
			filter.MinScore = v
		}
	}
	if md := r.URL.Query().Get("max_depth"); md != "" {
		if v, err := strconv.Atoi(md); err == nil {
			filter.MaxDepth = v
		}
	}

	store := rt.graphStore
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("graph store not configured"))
		return
	}

	graph, err := store.GetGraph(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Enrich graph nodes with category from document repository
	if graph != nil && rt.docRepo != nil {
		for i := range graph.Nodes {
			if graph.Nodes[i].Category == "" {
				doc, err := rt.docRepo.GetByID(r.Context(), graph.Nodes[i].ID)
				if err == nil && doc.Category != "" {
					graph.Nodes[i].Category = doc.Category
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(graph)
}

func (rt *Router) handlePostFeedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		MessageID      string `json:"message_id"`
		Rating         string `json:"rating"`
		Comment        string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Rating != "up" && req.Rating != "down" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("rating must be 'up' or 'down'"))
		return
	}
	if rt.feedbackStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("feedback not enabled"))
		return
	}

	fb := &domain.AgentFeedback{
		UserID:         r.Header.Get("X-User-ID"),
		ConversationID: req.ConversationID,
		MessageID:      req.MessageID,
		Rating:         req.Rating,
		Comment:        req.Comment,
	}
	if err := rt.feedbackStore.Create(r.Context(), fb); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": fb.ID})
}

func serveOpenAPISpecJSON(w http.ResponseWriter, _ *http.Request) {
	spec, err := apigen.GetSwagger()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(spec)
}

func (rt *Router) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CronExpr   string `json:"cron_expr"`
		Prompt     string `json:"prompt"`
		Condition  string `json:"condition"`
		WebhookURL string `json:"webhook_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.CronExpr == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("cron_expr is required"))
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("prompt is required"))
		return
	}
	if rt.scheduleStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("schedule store not configured"))
		return
	}

	task := &domain.ScheduledTask{
		UserID:     r.Header.Get("X-User-ID"),
		CronExpr:   req.CronExpr,
		Prompt:     req.Prompt,
		Condition:  req.Condition,
		WebhookURL: req.WebhookURL,
		Enabled:    true,
	}
	if err := rt.scheduleStore.Create(r.Context(), task); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(task)
}

func (rt *Router) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	if rt.scheduleStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("schedule store not configured"))
		return
	}

	userID := r.Header.Get("X-User-ID")
	tasks, err := rt.scheduleStore.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tasks)
}

func (rt *Router) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}
	if rt.scheduleStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("schedule store not configured"))
		return
	}

	if err := rt.scheduleStore.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (rt *Router) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}
	if rt.scheduleStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("schedule store not configured"))
		return
	}

	task, err := rt.scheduleStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	var patch struct {
		CronExpr   *string `json:"cron_expr"`
		Prompt     *string `json:"prompt"`
		Condition  *string `json:"condition"`
		WebhookURL *string `json:"webhook_url"`
		Enabled    *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if patch.CronExpr != nil {
		task.CronExpr = *patch.CronExpr
	}
	if patch.Prompt != nil {
		task.Prompt = *patch.Prompt
	}
	if patch.Condition != nil {
		task.Condition = *patch.Condition
	}
	if patch.WebhookURL != nil {
		task.WebhookURL = *patch.WebhookURL
	}
	if patch.Enabled != nil {
		task.Enabled = *patch.Enabled
	}

	if err := rt.scheduleStore.Update(r.Context(), task); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(task)
}

func (rt *Router) handleGetEventsSummary(w http.ResponseWriter, r *http.Request) {
	if rt.eventStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("event store not configured"))
		return
	}
	since := time.Now().AddDate(0, 0, -7)
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	counts, err := rt.eventStore.CountByType(r.Context(), since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(counts)
}

func (rt *Router) handleGetFeedbackSummary(w http.ResponseWriter, r *http.Request) {
	if rt.feedbackStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("feedback store not configured"))
		return
	}
	since := time.Now().AddDate(0, 0, -7)
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	counts, err := rt.feedbackStore.CountByRating(r.Context(), since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	recent, err := rt.feedbackStore.ListRecent(r.Context(), since, 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"counts": counts,
		"recent": recent,
	})
}

func (rt *Router) handleGetImprovements(w http.ResponseWriter, r *http.Request) {
	if rt.improvementStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("improvement store not configured"))
		return
	}
	items, err := rt.improvementStore.ListPending(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

func (rt *Router) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	if rt.docRepo == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("document repository not configured"))
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	docs, err := rt.docRepo.ListRecent(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if docs == nil {
		docs = []domain.Document{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(docs)
}

func (rt *Router) handleGetDocumentContent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}
	if rt.docRepo == nil || rt.objectStorage == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("document service not configured"))
		return
	}
	doc, err := rt.docRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	reader, err := rt.objectStorage.Open(r.Context(), doc.StoragePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("open file: %w", err))
		return
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(reader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("read file: %w", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":       doc.ID,
		"filename": doc.Filename,
		"content":  string(data),
	})
}

func (rt *Router) handlePatchImprovement(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}
	if rt.improvementStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("improvement store not configured"))
		return
	}
	var req struct {
		Status string `json:"status"` // "approved", "dismissed"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Status == "approved" {
		if err := rt.improvementStore.MarkApplied(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		if err := rt.improvementStore.UpdateStatus(r.Context(), id, req.Status); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
