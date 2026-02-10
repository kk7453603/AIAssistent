package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
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
	mux.HandleFunc("GET /openapi.json", serveOpenAPISpecJSON)

	strict := apigen.NewStrictHandlerWithOptions(rt, nil, apigen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusBadRequest, err)
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusInternalServerError, err)
		},
	})

	handler := apigen.HandlerWithOptions(strict, apigen.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusBadRequest, err)
		},
	})
	return loggingMiddleware(handler)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
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

	doc, err := rt.ingestUC.Upload(
		ctx,
		filename,
		part.Header.Get("Content-Type"),
		part,
	)
	if err != nil {
		return apigen.UploadDocument500JSONResponse{
			Error: err.Error(),
		}, nil
	}

	return apigen.UploadDocument202JSONResponse(toAPIDocument(doc)), nil
}

func (rt *Router) GetDocumentById(ctx context.Context, request apigen.GetDocumentByIdRequestObject) (apigen.GetDocumentByIdResponseObject, error) {
	doc, err := rt.repo.GetByID(ctx, request.DocumentId)
	if err != nil {
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

	answer, err := rt.queryUC.Answer(ctx, request.Body.Question, limit, filter)
	if err != nil {
		return apigen.QueryRag500JSONResponse{
			Error: err.Error(),
		}, nil
	}

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
