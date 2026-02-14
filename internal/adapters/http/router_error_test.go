package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/config"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type ingestErrFake struct {
	err error
}

func (f ingestErrFake) Upload(context.Context, string, string, io.Reader) (*domain.Document, error) {
	return nil, f.err
}

type queryErrFake struct {
	err error
}

func (f queryErrFake) Answer(context.Context, string, int, domain.SearchFilter) (*domain.Answer, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Answer{Text: "ok"}, nil
}

func (f queryErrFake) GenerateFromPrompt(context.Context, string) (string, error) { return "ok", nil }

type docsErrFake struct {
	err error
}

func (f docsErrFake) GetByID(context.Context, string) (*domain.Document, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Document{ID: "doc-1", Filename: "a", MimeType: "text/plain", StoragePath: "a", Status: domain.StatusReady}, nil
}

func TestQueryRagMapsDomainInvalidInputTo400(t *testing.T) {
	handler := NewRouter(
		config.Config{RAGTopK: 5},
		nil,
		queryErrFake{err: domain.WrapError(domain.ErrInvalidInput, "answer", errors.New("bad query"))},
		docsErrFake{},
	).Handler()

	payload, _ := json.Marshal(map[string]any{"question": "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/rag/query", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestGetDocumentByIDReturns404ForNotFound(t *testing.T) {
	handler := NewRouter(
		config.Config{RAGTopK: 5},
		nil,
		queryErrFake{},
		docsErrFake{err: domain.WrapError(domain.ErrDocumentNotFound, "get", errors.New("id=missing"))},
	).Handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/documents/missing", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.Code)
	}
}
