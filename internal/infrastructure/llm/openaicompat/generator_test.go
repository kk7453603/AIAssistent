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

func TestGenerator_GenerateAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body chatRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		// Verify system message is present
		if len(body.Messages) < 2 {
			t.Error("expected at least 2 messages (system + user)")
		}
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "Generated answer"}}},
		})
	}))
	defer server.Close()

	gen := NewGenerator(New(server.URL, "", "model"))
	answer, err := gen.GenerateAnswer(context.Background(), "What is AI?", []domain.RetrievedChunk{
		{Text: "AI is artificial intelligence", Filename: "doc.txt", Score: 0.9},
	})
	if err != nil {
		t.Fatalf("GenerateAnswer error: %v", err)
	}
	if answer != "Generated answer" {
		t.Fatalf("expected 'Generated answer', got %q", answer)
	}
}

func TestGenerator_GenerateFromPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "prompt response"}}},
		})
	}))
	defer server.Close()

	gen := NewGenerator(New(server.URL, "", "model"))
	result, err := gen.GenerateFromPrompt(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("GenerateFromPrompt error: %v", err)
	}
	if result != "prompt response" {
		t.Fatalf("expected 'prompt response', got %q", result)
	}
}

func TestGenerator_GenerateJSONFromPrompt(t *testing.T) {
	var gotJSONMode bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if rf, ok := body["response_format"]; ok {
			rfMap := rf.(map[string]any)
			if rfMap["type"] == "json_object" {
				gotJSONMode = true
			}
		}
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: `{"result":"ok"}`}}},
		})
	}))
	defer server.Close()

	gen := NewGenerator(New(server.URL, "", "model"))
	result, err := gen.GenerateJSONFromPrompt(context.Background(), "generate json")
	if err != nil {
		t.Fatalf("GenerateJSONFromPrompt error: %v", err)
	}
	if !gotJSONMode {
		t.Fatal("expected JSON mode to be set")
	}
	if !strings.Contains(result, "ok") {
		t.Fatalf("expected JSON result, got %q", result)
	}
}

func TestBuildAnswerPrompt(t *testing.T) {
	chunks := []domain.RetrievedChunk{
		{Text: "chunk 1", Filename: "file1.txt", Category: "cat1", Score: 0.9},
		{Text: "chunk 2", Filename: "file2.txt", Category: "cat2", Score: 0.8},
	}
	prompt := buildAnswerPrompt("test question", chunks)
	if !strings.Contains(prompt, "test question") {
		t.Fatal("prompt should contain the question")
	}
	if !strings.Contains(prompt, "chunk 1") || !strings.Contains(prompt, "chunk 2") {
		t.Fatal("prompt should contain chunk texts")
	}
	if !strings.Contains(prompt, "file1.txt") {
		t.Fatal("prompt should contain filenames")
	}
}
