package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
	"github.com/kirillkom/personal-ai-assistant/internal/config"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
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
	openAICompatContextMessages  int
	openAICompatStreamChunkChars int
	toolTriggerKeywords          []string
	ragTopK                      int
	agentModeEnabled             bool
	httpMetrics                  *metrics.HTTPServerMetrics
	apiRateLimiter               *rate.Limiter
	apiBackpressureMaxInFlight   int
	apiBackpressureWaitTimeout   time.Duration
}

func NewRouter(
	cfg config.Config,
	ingestor ports.DocumentIngestor,
	querySvc ports.DocumentQueryService,
	docs ports.DocumentReader,
	agentSvc ports.AgentChatService,
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
	for _, keyword := range strings.Split(cfg.OpenAICompatToolTriggerKeywords, ",") {
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
		apiRateLimitBurst = int(math.Ceil(apiRateLimitRPS))
		if apiRateLimitBurst < 1 {
			apiRateLimitBurst = 1
		}
	}
	if apiRateLimitRPS > 0 && apiRateLimitBurst > 0 {
		apiRateLimiter = rate.NewLimiter(rate.Limit(apiRateLimitRPS), apiRateLimitBurst)
	}
	apiBackpressureWait := time.Duration(cfg.APIBackpressureWaitMS) * time.Millisecond
	if cfg.APIBackpressureWaitMS < 0 {
		apiBackpressureWait = 0
	}

	return &Router{
		ingestor: ingestor,
		querySvc: querySvc,
		docs:     docs,
		agentSvc: agentSvc,

		openAICompatAPIKey:           cfg.OpenAICompatAPIKey,
		openAICompatModelID:          cfg.OpenAICompatModelID,
		openAICompatContextMessages:  contextMessages,
		openAICompatStreamChunkChars: streamChunkChars,
		toolTriggerKeywords:          toolKeywords,
		ragTopK:                      ragTopK,
		agentModeEnabled:             cfg.AgentModeEnabled,
		httpMetrics:                  metrics.NewHTTPServerMetrics("api"),
		apiRateLimiter:               apiRateLimiter,
		apiBackpressureMaxInFlight:   cfg.APIBackpressureMaxInFlight,
		apiBackpressureWaitTimeout:   apiBackpressureWait,
	}
}

func (rt *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /openapi.json", serveOpenAPISpecJSON)
	mux.Handle("GET /metrics", rt.httpMetrics.Handler())

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
	defer part.Close()

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
		filter.Category = *request.Body.Category
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
		part.Close()
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
