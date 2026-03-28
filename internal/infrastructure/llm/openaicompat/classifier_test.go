package openaicompat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassifier_Classify_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: `{"category":"tech","subcategory":"ai","tags":["ml","llm"],"confidence":0.9,"summary":"AI document"}`}}},
		})
	}))
	defer server.Close()

	client := New(server.URL, "", "model")
	classifier := NewClassifier(client)

	cls, err := classifier.Classify(context.Background(), "test document about AI")
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if cls.Category != "tech" {
		t.Fatalf("expected category 'tech', got %q", cls.Category)
	}
	if len(cls.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(cls.Tags))
	}
}

func TestClassifier_Classify_NilTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: `{"category":"doc","subcategory":"misc","confidence":0.5,"summary":"test"}`}}},
		})
	}))
	defer server.Close()

	client := New(server.URL, "", "model")
	classifier := NewClassifier(client)

	cls, err := classifier.Classify(context.Background(), "test")
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if cls.Tags == nil {
		t.Fatal("expected non-nil tags (should be initialized to empty slice)")
	}
}

func TestClassifier_Classify_Truncation(t *testing.T) {
	var gotLen int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body chatRequest
		json.NewDecoder(r.Body).Decode(&body)
		if len(body.Messages) > 0 {
			gotLen = len(body.Messages[0].Content)
		}
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: `{"category":"a","subcategory":"b","confidence":0.5,"summary":"s"}`}}},
		})
	}))
	defer server.Close()

	client := New(server.URL, "", "model")
	classifier := NewClassifier(client)

	// Create text larger than maxSnippet (4000)
	longText := make([]byte, 5000)
	for i := range longText {
		longText[i] = 'x'
	}
	_, err := classifier.Classify(context.Background(), string(longText))
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	// The prompt includes the header text, so gotLen should be less than 5000
	if gotLen >= 5000 {
		t.Fatalf("expected truncated prompt, got length %d", gotLen)
	}
}

func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain_json", `{"key":"val"}`, `{"key":"val"}`},
		{"with_prefix", `Here: {"key":"val"}`, `{"key":"val"}`},
		{"with_markdown", "```json\n{\"key\":\"val\"}\n```", `{"key":"val"}`},
		{"no_braces", "no json here", "no json here"},
		{"nested", `{"a":{"b":1}}`, `{"a":{"b":1}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONObject(tt.input)
			if got != tt.want {
				t.Fatalf("extractJSONObject() = %q, want %q", got, tt.want)
			}
		})
	}
}
