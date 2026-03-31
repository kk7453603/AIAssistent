package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

func TestGeneratorBuildsContextPrompt(t *testing.T) {
	var capturedPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		capturedPrompt, _ = payload["prompt"].(string)
		_, _ = w.Write([]byte(`{"response":"ok"}`))
	}))
	defer server.Close()

	client := New(server.URL, "gen", "embed")
	gen := NewGenerator(client)
	_, err := gen.GenerateAnswer(context.Background(), "question?", []domain.RetrievedChunk{{Filename: "a.txt", Category: "general", Text: "chunk text", Score: 0.99}})
	if err != nil {
		t.Fatalf("GenerateAnswer() error = %v", err)
	}
	if !strings.Contains(capturedPrompt, "question?") || !strings.Contains(capturedPrompt, "chunk text") {
		t.Fatalf("unexpected prompt: %s", capturedPrompt)
	}
}

func TestEmbedIncludesHTTPBodyInError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	client := New(server.URL, "gen", "embed")
	embedder := NewEmbedder(client)
	_, err := embedder.Embed(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "model unavailable") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestOllamaChatWithTools_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": {
				"role": "assistant",
				"content": "",
				"tool_calls": [
					{
						"function": {
							"name": "knowledge_search",
							"arguments": {"query": "golang concurrency"}
						}
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client := New(server.URL, "gen", "embed")
	gen := NewGenerator(client)

	messages := []domain.ChatMessage{
		{Role: "user", Content: "search for golang concurrency"},
	}
	tools := []domain.ToolSchema{
		{
			Type: "function",
			Function: domain.FunctionSchema{
				Name:        "knowledge_search",
				Description: "Search knowledge base",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}},
			},
		},
	}

	result, err := gen.ChatWithTools(context.Background(), messages, tools)
	if err != nil {
		t.Fatalf("ChatWithTools() error = %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	tc := result.ToolCalls[0]
	if tc.Function.Name != "knowledge_search" {
		t.Errorf("expected tool name=knowledge_search, got %q", tc.Function.Name)
	}
	query, ok := tc.Function.Arguments["query"].(string)
	if !ok || query != "golang concurrency" {
		t.Errorf("expected query=golang concurrency, got %v", tc.Function.Arguments["query"])
	}
	if result.Content != "" {
		t.Errorf("expected empty content for tool call response, got %q", result.Content)
	}
}

func TestOllamaChatWithTools_NoToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": {
				"role": "assistant",
				"content": "Here is the answer to your question."
			}
		}`))
	}))
	defer server.Close()

	client := New(server.URL, "gen", "embed")
	gen := NewGenerator(client)

	messages := []domain.ChatMessage{
		{Role: "user", Content: "what is 2+2?"},
	}

	result, err := gen.ChatWithTools(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("ChatWithTools() error = %v", err)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected 0 tool calls, got %d", len(result.ToolCalls))
	}
	if result.Content != "Here is the answer to your question." {
		t.Errorf("expected content text, got %q", result.Content)
	}
}

func TestOllamaReranker_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		// Return a score JSON for reranking
		_, _ = w.Write([]byte(`{"response":"{\"score\": 8}"}`))
	}))
	defer server.Close()

	client := New(server.URL, "gen", "embed")
	reranker := NewReranker(client)

	chunks := []domain.RetrievedChunk{
		{DocumentID: "d1", Filename: "a.txt", Text: "first chunk", Score: 0.5},
		{DocumentID: "d2", Filename: "b.txt", Text: "second chunk", Score: 0.6},
	}

	results, err := reranker.Rerank(context.Background(), "test query", chunks, 2)
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// All chunks should have score 0.8 (8/10) since mock returns same score
	for i, r := range results {
		if r.Score != 0.8 {
			t.Errorf("result[%d]: expected score=0.8, got %f", i, r.Score)
		}
	}
}

func TestOllamaReranker_Empty(t *testing.T) {
	client := New("http://localhost:11434", "gen", "embed")
	reranker := NewReranker(client)

	results, err := reranker.Rerank(context.Background(), "query", nil, 5)
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty input, got %d", len(results))
	}
}

// --- Content streaming with <think> tag detection tests ---

func TestContentStreaming_DetectsThinkTags(t *testing.T) {
	// Simulate NDJSON stream: <think>reasoning</think>answer
	chunks := []string{
		`{"message":{"content":"<think>"},"done":false}`,
		`{"message":{"content":"step 1, "},"done":false}`,
		`{"message":{"content":"step 2"},"done":false}`,
		`{"message":{"content":"</think>"},"done":false}`,
		`{"message":{"content":"final answer"},"done":false}`,
		`{"message":{"content":""},"done":true}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		for _, c := range chunks {
			_, _ = w.Write([]byte(c + "\n"))
		}
	}))
	defer server.Close()

	client := NewWithOptions(server.URL, "gen", "embed", Options{ThinkEnabled: true})

	var thinkingTokens []string
	onThinking := func(text string) {
		thinkingTokens = append(thinkingTokens, text)
	}

	result, err := client.chatWithToolsContentStreaming(
		context.Background(), "gen", nil, nil, onThinking,
	)
	if err != nil {
		t.Fatalf("chatWithToolsContentStreaming() error = %v", err)
	}

	// Thinking tokens should have been streamed
	if len(thinkingTokens) == 0 {
		t.Fatal("expected thinking tokens to be streamed")
	}
	joined := strings.Join(thinkingTokens, "")
	if !strings.Contains(joined, "step 1") || !strings.Contains(joined, "step 2") {
		t.Fatalf("expected thinking to contain steps, got %q", joined)
	}

	// Content should include both think wrapper and answer
	if !strings.Contains(result.Content, "final answer") {
		t.Fatalf("expected content to contain answer, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "<think>") {
		t.Fatalf("expected content to contain <think> wrapper, got %q", result.Content)
	}
}

func TestContentStreaming_NoThinkTags(t *testing.T) {
	// Regular content without <think> tags
	chunks := []string{
		`{"message":{"content":"Hello "},"done":false}`,
		`{"message":{"content":"world!"},"done":false}`,
		`{"message":{"content":""},"done":true}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		for _, c := range chunks {
			_, _ = w.Write([]byte(c + "\n"))
		}
	}))
	defer server.Close()

	client := NewWithOptions(server.URL, "gen", "embed", Options{ThinkEnabled: true})

	var thinkingTokens []string
	onThinking := func(text string) {
		thinkingTokens = append(thinkingTokens, text)
	}

	result, err := client.chatWithToolsContentStreaming(
		context.Background(), "gen", nil, nil, onThinking,
	)
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	if len(thinkingTokens) != 0 {
		t.Fatalf("expected no thinking tokens for non-think content, got %v", thinkingTokens)
	}
	if !strings.Contains(result.Content, "Hello world!") {
		t.Fatalf("expected content, got %q", result.Content)
	}
}

func TestContentStreaming_ToolCalls(t *testing.T) {
	chunks := []string{
		`{"message":{"content":"","tool_calls":[{"function":{"name":"web_search","arguments":{"query":"test"}}}]},"done":false}`,
		`{"message":{"content":""},"done":true}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		for _, c := range chunks {
			_, _ = w.Write([]byte(c + "\n"))
		}
	}))
	defer server.Close()

	client := NewWithOptions(server.URL, "gen", "embed", Options{ThinkEnabled: true})
	result, err := client.chatWithToolsContentStreaming(
		context.Background(), "gen", nil, nil, func(string) {},
	)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Function.Name != "web_search" {
		t.Fatalf("expected web_search, got %q", result.ToolCalls[0].Function.Name)
	}
}

func TestContentStreaming_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewWithOptions(server.URL, "gen", "embed", Options{ThinkEnabled: true})
	_, err := client.chatWithToolsContentStreaming(
		context.Background(), "gen", nil, nil, func(string) {},
	)
	if err == nil {
		t.Fatal("expected error on server 500")
	}
}

func TestContentStreaming_ThinkTagSplitAcrossChunks(t *testing.T) {
	// <think> tag arrives in parts: "<thi" then "nk>thinking</think>answer"
	// The state machine buffers initial chars to detect <think>
	chunks := []string{
		`{"message":{"content":"<thi"},"done":false}`,
		`{"message":{"content":"nk>"},"done":false}`,
		`{"message":{"content":"deep thought"},"done":false}`,
		`{"message":{"content":"</think>"},"done":false}`,
		`{"message":{"content":"result"},"done":false}`,
		`{"message":{"content":""},"done":true}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		for _, c := range chunks {
			_, _ = w.Write([]byte(c + "\n"))
		}
	}))
	defer server.Close()

	client := NewWithOptions(server.URL, "gen", "embed", Options{ThinkEnabled: true})

	var thinkingTokens []string
	result, err := client.chatWithToolsContentStreaming(
		context.Background(), "gen", nil, nil, func(text string) {
			thinkingTokens = append(thinkingTokens, text)
		},
	)
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	joined := strings.Join(thinkingTokens, "")
	if !strings.Contains(joined, "deep thought") {
		t.Fatalf("expected thinking to contain 'deep thought', got %q", joined)
	}
	if !strings.Contains(result.Content, "result") {
		t.Fatalf("expected content to contain 'result', got %q", result.Content)
	}
}

func TestGenerateJSONFromPromptUsesPlannerModelAndJSONFormat(t *testing.T) {
	var capturedModel string
	var capturedFormat string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		capturedModel, _ = payload["model"].(string)
		capturedFormat, _ = payload["format"].(string)
		_, _ = w.Write([]byte(`{"response":"{\"type\":\"final\",\"answer\":\"ok\"}"}`))
	}))
	defer server.Close()

	client := NewWithOptions(server.URL, "gen-model", "embed-model", Options{PlannerModel: "planner-model"})
	gen := NewGenerator(client)
	out, err := gen.GenerateJSONFromPrompt(context.Background(), "return json")
	if err != nil {
		t.Fatalf("GenerateJSONFromPrompt() error = %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty response")
	}
	if capturedModel != "planner-model" {
		t.Fatalf("expected planner model, got %q", capturedModel)
	}
	if capturedFormat != "json" {
		t.Fatalf("expected format=json, got %q", capturedFormat)
	}
}

func TestSetRuntimeModelConfigAppliesUpdatedModels(t *testing.T) {
	var generateModels []string
	var embedModels []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode generate request: %v", err)
			}
			model, _ := payload["model"].(string)
			generateModels = append(generateModels, model)
			_, _ = w.Write([]byte(`{"response":"ok"}`))
		case "/api/embed":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode embed request: %v", err)
			}
			model, _ := payload["model"].(string)
			embedModels = append(embedModels, model)
			_, _ = w.Write([]byte(`{"embeddings":[[0.1,0.2]]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewWithOptions(server.URL, "old-gen", "old-embed", Options{PlannerModel: "old-plan"})
	if err := client.SetRuntimeModelConfig(ports.RuntimeModelConfig{
		GenerationModel: "new-gen",
		PlannerModel:    "new-plan",
		EmbeddingModel:  "new-embed",
	}); err != nil {
		t.Fatalf("SetRuntimeModelConfig() error = %v", err)
	}

	gen := NewGenerator(client)
	if _, err := gen.GenerateFromPrompt(context.Background(), "hello"); err != nil {
		t.Fatalf("GenerateFromPrompt() error = %v", err)
	}
	if _, err := gen.GenerateJSONFromPrompt(context.Background(), "return json"); err != nil {
		t.Fatalf("GenerateJSONFromPrompt() error = %v", err)
	}
	embedder := NewEmbedder(client)
	if _, err := embedder.Embed(context.Background(), []string{"embed me"}); err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(generateModels) != 2 {
		t.Fatalf("expected 2 generate calls, got %d", len(generateModels))
	}
	if generateModels[0] != "new-gen" {
		t.Fatalf("expected updated generation model, got %q", generateModels[0])
	}
	if generateModels[1] != "new-plan" {
		t.Fatalf("expected updated planner model, got %q", generateModels[1])
	}
	if len(embedModels) != 1 || embedModels[0] != "new-embed" {
		t.Fatalf("expected updated embedding model, got %v", embedModels)
	}
}
