package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := New(server.URL, "test-key", "test-model")
	return server, client
}

func TestChatCompletion_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "hello world"}}},
		})
	})

	result, err := client.chatCompletion(context.Background(), []chatMessage{{Role: "user", Content: "hi"}}, false)
	if err != nil {
		t.Fatalf("chatCompletion error: %v", err)
	}
	if result != "hello world" {
		t.Fatalf("expected 'hello world', got %q", result)
	}
}

func TestChatCompletion_EmptyChoices(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(chatResponse{})
	})

	_, err := client.chatCompletion(context.Background(), []chatMessage{{Role: "user", Content: "hi"}}, false)
	if err == nil || !strings.Contains(err.Error(), "empty choices") {
		t.Fatalf("expected empty choices error, got: %v", err)
	}
}

func TestChatCompletion_HTTPError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	})

	_, err := client.chatCompletion(context.Background(), []chatMessage{{Role: "user", Content: "hi"}}, false)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	var pe *ProviderError
	if !isProviderError(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.StatusCode != 500 {
		t.Fatalf("expected status 500, got %d", pe.StatusCode)
	}
}

func TestChatCompletion_JSONMode(t *testing.T) {
	var gotFormat bool
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if rf, ok := body["response_format"]; ok {
			rfMap := rf.(map[string]any)
			if rfMap["type"] == "json_object" {
				gotFormat = true
			}
		}
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: `{"key":"value"}`}}},
		})
	})

	_, err := client.chatCompletion(context.Background(), []chatMessage{{Role: "user", Content: "hi"}}, true)
	if err != nil {
		t.Fatalf("chatCompletion error: %v", err)
	}
	if !gotFormat {
		t.Fatal("expected response_format to be set for JSON mode")
	}
}

func TestEmbedTexts_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embedResponse{
			Data: []struct {
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}{
				{Index: 0, Embedding: []float32{0.1, 0.2}},
				{Index: 1, Embedding: []float32{0.3, 0.4}},
			},
		})
	})

	vectors, err := client.embedTexts(context.Background(), "model", []string{"a", "b"})
	if err != nil {
		t.Fatalf("embedTexts error: %v", err)
	}
	if len(vectors) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vectors))
	}
	if vectors[0][0] != 0.1 {
		t.Fatalf("expected first vector[0]=0.1, got %f", vectors[0][0])
	}
}

func TestPostJSON_AuthHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer server.Close()

	client := New(server.URL, "my-secret-key", "model")
	var out map[string]any
	_ = client.postJSON(context.Background(), "/test", map[string]any{}, &out, "test")
	if gotAuth != "Bearer my-secret-key" {
		t.Fatalf("expected Bearer auth, got %q", gotAuth)
	}
}

func TestPostJSON_NoAuthWhenKeyEmpty(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer server.Close()

	client := New(server.URL, "", "model")
	var out map[string]any
	_ = client.postJSON(context.Background(), "/test", map[string]any{}, &out, "test")
	if gotAuth != "" {
		t.Fatalf("expected no auth header, got %q", gotAuth)
	}
}

func TestPostJSON_ExtraHeaders(t *testing.T) {
	var gotCustom string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCustom = r.Header.Get("X-Custom")
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer server.Close()

	client := New(server.URL, "", "model", Options{ExtraHeaders: map[string]string{"X-Custom": "test-value"}})
	var out map[string]any
	_ = client.postJSON(context.Background(), "/test", map[string]any{}, &out, "test")
	if gotCustom != "test-value" {
		t.Fatalf("expected X-Custom='test-value', got %q", gotCustom)
	}
}

func TestChatWithTools_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": "",
					"tool_calls": []map[string]any{{
						"id": "call-123",
						"function": map[string]any{
							"name":      "test_tool",
							"arguments": map[string]any{"key": "value"},
						},
					}},
				},
			}},
		})
	})

	gen := NewGenerator(client)
	result, err := gen.ChatWithTools(context.Background(),
		[]domain.ChatMessage{{Role: "user", Content: "hi"}},
		[]domain.ToolSchema{{Type: "function", Function: domain.FunctionSchema{Name: "test_tool"}}},
	)
	if err != nil {
		t.Fatalf("ChatWithTools error: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Function.Name != "test_tool" {
		t.Fatalf("expected tool name 'test_tool', got %q", result.ToolCalls[0].Function.Name)
	}
}

func TestChatWithTools_NoToolCalls(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": "just text",
				},
			}},
		})
	})

	gen := NewGenerator(client)
	result, err := gen.ChatWithTools(context.Background(),
		[]domain.ChatMessage{{Role: "user", Content: "hi"}},
		nil,
	)
	if err != nil {
		t.Fatalf("ChatWithTools error: %v", err)
	}
	if result.Content != "just text" {
		t.Fatalf("expected content 'just text', got %q", result.Content)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected 0 tool calls, got %d", len(result.ToolCalls))
	}
}

func TestModel(t *testing.T) {
	client := New("http://localhost", "key", "my-model")
	if client.Model() != "my-model" {
		t.Fatalf("expected 'my-model', got %q", client.Model())
	}
}

func isProviderError(err error, target **ProviderError) bool {
	for err != nil {
		if pe, ok := err.(*ProviderError); ok {
			*target = pe
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}
