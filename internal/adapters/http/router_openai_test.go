package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apigen "github.com/kirillkom/personal-ai-assistant/internal/adapters/http/openapi"
	"github.com/kirillkom/personal-ai-assistant/internal/config"
	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/usecase"
)

type fakeDocumentRepo struct{}

func (f fakeDocumentRepo) Create(context.Context, *domain.Document) error { return nil }
func (f fakeDocumentRepo) GetByID(context.Context, string) (*domain.Document, error) {
	return &domain.Document{ID: "doc-1", Filename: "doc.txt", MimeType: "text/plain", StoragePath: "doc.txt", Status: domain.StatusReady, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (f fakeDocumentRepo) UpdateStatus(context.Context, string, domain.DocumentStatus, string) error {
	return nil
}
func (f fakeDocumentRepo) SaveClassification(context.Context, string, domain.Classification) error {
	return nil
}

type fakeEmbedder struct{}

func (f fakeEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return [][]float32{{0.1, 0.2}}, nil
}
func (f fakeEmbedder) EmbedQuery(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

type fakeVectorStore struct{}

func (f fakeVectorStore) IndexChunks(context.Context, *domain.Document, []string, [][]float32) error {
	return nil
}
func (f fakeVectorStore) Search(context.Context, []float32, int, domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	return []domain.RetrievedChunk{{
		DocumentID: "doc-1",
		Filename:   "doc.txt",
		Category:   "general",
		ChunkIndex: 0,
		Text:       "chunk",
		Score:      0.77,
	}}, nil
}

func (f fakeVectorStore) SearchLexical(context.Context, string, int, domain.SearchFilter) ([]domain.RetrievedChunk, error) {
	return []domain.RetrievedChunk{{
		DocumentID: "doc-1",
		Filename:   "doc.txt",
		Category:   "general",
		ChunkIndex: 0,
		Text:       "chunk",
		Score:      0.75,
	}}, nil
}

type fakeAnswerGenerator struct{}

func (f fakeAnswerGenerator) GenerateAnswer(context.Context, string, []domain.RetrievedChunk) (string, error) {
	return "rag answer", nil
}

func (f fakeAnswerGenerator) GenerateFromPrompt(context.Context, string) (string, error) {
	return "post processed answer", nil
}

func newTestHandler(cfg config.Config) http.Handler {
	queryUC := usecase.NewQueryUseCase(fakeEmbedder{}, fakeVectorStore{}, fakeAnswerGenerator{}, usecase.QueryOptions{})
	router := NewRouter(cfg, nil, queryUC, fakeDocumentRepo{})
	return router.Handler()
}

func TestListModelsAuthModes(t *testing.T) {
	handlerNoAuth := newTestHandler(config.Config{OpenAICompatModelID: "paa-rag-v1", RAGTopK: 5})

	reqNoAuth := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	resNoAuth := httptest.NewRecorder()
	handlerNoAuth.ServeHTTP(resNoAuth, reqNoAuth)
	if resNoAuth.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resNoAuth.Code)
	}

	var listResp apigen.ModelListResponse
	if err := json.NewDecoder(resNoAuth.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if listResp.Object != "list" {
		t.Fatalf("expected object=list, got %s", listResp.Object)
	}
	if len(listResp.Data) == 0 || listResp.Data[0].Id != "paa-rag-v1" {
		t.Fatalf("unexpected model list response: %+v", listResp.Data)
	}

	handlerWithAuth := newTestHandler(config.Config{
		OpenAICompatAPIKey:  "secret-key",
		OpenAICompatModelID: "paa-rag-v1",
		RAGTopK:             5,
	})

	reqUnauthorized := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	resUnauthorized := httptest.NewRecorder()
	handlerWithAuth.ServeHTTP(resUnauthorized, reqUnauthorized)
	if resUnauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resUnauthorized.Code)
	}

	reqAuthorized := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	reqAuthorized.Header.Set("Authorization", "Bearer secret-key")
	resAuthorized := httptest.NewRecorder()
	handlerWithAuth.ServeHTTP(resAuthorized, reqAuthorized)
	if resAuthorized.Code != http.StatusOK {
		t.Fatalf("expected 200 with auth, got %d", resAuthorized.Code)
	}

	chatPayload, _ := json.Marshal(map[string]interface{}{
		"model": "paa-rag-v1",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "test"},
		},
	})
	reqChatUnauthorized := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(chatPayload))
	reqChatUnauthorized.Header.Set("Content-Type", "application/json")
	resChatUnauthorized := httptest.NewRecorder()
	handlerWithAuth.ServeHTTP(resChatUnauthorized, reqChatUnauthorized)
	if resChatUnauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for chat without auth, got %d", resChatUnauthorized.Code)
	}
}

func TestChatCompletionsJSONAndStream(t *testing.T) {
	handler := newTestHandler(config.Config{
		OpenAICompatModelID:             "paa-rag-v1",
		OpenAICompatContextMessages:     5,
		OpenAICompatStreamChunkChars:    8,
		OpenAICompatToolTriggerKeywords: "upload,file,document",
		RAGTopK:                         5,
	})

	jsonReqBody := map[string]interface{}{
		"model": "paa-rag-v1",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "tell me about this document"},
		},
	}
	jsonPayload, _ := json.Marshal(jsonReqBody)

	jsonReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(jsonPayload))
	jsonReq.Header.Set("Content-Type", "application/json")
	jsonRes := httptest.NewRecorder()
	handler.ServeHTTP(jsonRes, jsonReq)
	if jsonRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", jsonRes.Code)
	}

	var chatResp apigen.ChatCompletionResponse
	if err := json.NewDecoder(jsonRes.Body).Decode(&chatResp); err != nil {
		t.Fatalf("decode chat response: %v", err)
	}
	if len(chatResp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(chatResp.Choices))
	}
	if chatResp.Choices[0].Message.Content == nil {
		t.Fatalf("expected content in assistant message")
	}

	streamReqBody := map[string]interface{}{
		"model":  "paa-rag-v1",
		"stream": true,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "tell me about this document"},
		},
	}
	streamPayload, _ := json.Marshal(streamReqBody)
	streamReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(streamPayload))
	streamReq.Header.Set("Content-Type", "application/json")
	streamRes := httptest.NewRecorder()
	handler.ServeHTTP(streamRes, streamReq)
	if streamRes.Code != http.StatusOK {
		t.Fatalf("expected 200 for stream, got %d", streamRes.Code)
	}
	if !strings.Contains(streamRes.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %s", streamRes.Header().Get("Content-Type"))
	}

	streamBody, _ := io.ReadAll(streamRes.Body)
	streamString := string(streamBody)
	if !strings.Contains(streamString, "chat.completion.chunk") {
		t.Fatalf("stream response does not contain chunks: %s", streamString)
	}
	if !strings.Contains(streamString, "data: [DONE]") {
		t.Fatalf("stream response does not contain DONE marker: %s", streamString)
	}
}

func TestChatCompletionsToolCalls(t *testing.T) {
	handler := newTestHandler(config.Config{
		OpenAICompatModelID:             "paa-rag-v1",
		OpenAICompatToolTriggerKeywords: "upload,file,document",
		RAGTopK:                         5,
	})

	payload := map[string]interface{}{
		"model": "paa-rag-v1",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "please upload this file"},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name": "ingest_and_query",
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	var chatResp apigen.ChatCompletionResponse
	if err := json.NewDecoder(res.Body).Decode(&chatResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(chatResp.Choices) != 1 || chatResp.Choices[0].FinishReason == nil || *chatResp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("expected finish_reason tool_calls, got %+v", chatResp.Choices)
	}
	if chatResp.Choices[0].Message.ToolCalls == nil || len(*chatResp.Choices[0].Message.ToolCalls) == 0 {
		t.Fatalf("expected tool_calls in message")
	}
}

func TestChatCompletionsPostToolProcessing(t *testing.T) {
	handler := newTestHandler(config.Config{
		OpenAICompatModelID: "paa-rag-v1",
		RAGTopK:             5,
	})

	payload := map[string]interface{}{
		"model": "paa-rag-v1",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "summarize uploaded files"},
			{"role": "tool", "content": "{\"results\":[{\"file_name\":\"a.txt\",\"rag_status\":\"ok\"}]}"},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	var chatResp apigen.ChatCompletionResponse
	if err := json.NewDecoder(res.Body).Decode(&chatResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message.Content == nil {
		t.Fatalf("expected message content")
	}
	content, ok := (*chatResp.Choices[0].Message.Content).(string)
	if !ok {
		t.Fatalf("expected string content, got %#v", *chatResp.Choices[0].Message.Content)
	}
	if content != "post processed answer" {
		t.Fatalf("expected post processed answer, got %s", content)
	}
}
