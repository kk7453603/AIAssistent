package httpadapter

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
	"github.com/kirillkom/personal-ai-assistant/internal/core/usecase"
)

type Router struct {
	ingestUC *usecase.IngestDocumentUseCase
	queryUC  *usecase.QueryUseCase
	repo     ports.DocumentRepository
}

func NewRouter(
	ingestUC *usecase.IngestDocumentUseCase,
	queryUC *usecase.QueryUseCase,
	repo ports.DocumentRepository,
) *Router {
	return &Router{
		ingestUC: ingestUC,
		queryUC:  queryUC,
		repo:     repo,
	}
}

func (rt *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", rt.healthz)
	mux.HandleFunc("/v1/documents", rt.uploadDocument)
	mux.HandleFunc("/v1/documents/", rt.getDocumentByID)
	mux.HandleFunc("/v1/rag/query", rt.queryRAG)
	return loggingMiddleware(mux)
}

func (rt *Router) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (rt *Router) uploadDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart field 'file' is required"})
		return
	}
	defer file.Close()

	doc, err := rt.ingestUC.Upload(
		r.Context(),
		fileHeader.Filename,
		fileHeader.Header.Get("Content-Type"),
		file,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, doc)
}

func (rt *Router) getDocumentByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/documents/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document id is required"})
		return
	}

	doc, err := rt.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (rt *Router) queryRAG(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Question string `json:"question"`
		Limit    int    `json:"limit"`
		Category string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.Question) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question is required"})
		return
	}

	answer, err := rt.queryUC.Answer(r.Context(), req.Question, req.Limit, domain.SearchFilter{
		Category: req.Category,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, answer)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
