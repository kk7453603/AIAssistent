package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/config"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

type ingestSuccessFake struct{}

func (f ingestSuccessFake) Upload(_ context.Context, filename, mimeType string, body io.Reader) (*domain.Document, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, domain.WrapError(domain.ErrInvalidInput, "upload", io.EOF)
	}

	now := time.Now().UTC()
	return &domain.Document{
		ID:          "doc-1",
		Filename:    filename,
		MimeType:    mimeType,
		StoragePath: "doc-1_file.txt",
		Status:      domain.StatusUploaded,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func newRouterForIngestTests() http.Handler {
	return NewRouter(
		config.Config{RAGTopK: 5},
		ingestSuccessFake{},
		queryErrFake{},
		docsErrFake{},
	).Handler()
}

func TestHealthzEndpoint(t *testing.T) {
	handler := newRouterForIngestTests()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestUploadDocumentSuccess(t *testing.T) {
	handler := newRouterForIngestTests()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "file.txt")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", res.Code)
	}

	var docResp map[string]any
	if err := json.NewDecoder(res.Body).Decode(&docResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if docResp["id"] != "doc-1" {
		t.Fatalf("unexpected response: %+v", docResp)
	}
}

func TestUploadDocumentMissingMultipartField(t *testing.T) {
	handler := newRouterForIngestTests()

	req := httptest.NewRequest(http.MethodPost, "/v1/documents", bytes.NewBufferString("plain-text"))
	req.Header.Set("Content-Type", "text/plain")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}
