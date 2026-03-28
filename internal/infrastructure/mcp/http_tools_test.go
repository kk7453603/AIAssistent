package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseHTTPTools(t *testing.T) {
	raw := `[{"name":"weather","description":"Get weather","url":"https://api.example.com","method":"GET","params":{"city":"string"}}]`
	tools, err := ParseHTTPTools(raw)
	if err != nil {
		t.Fatalf("ParseHTTPTools() error = %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "weather" {
		t.Errorf("name = %q", tools[0].Name)
	}
}

func TestParseHTTPTools_Empty(t *testing.T) {
	tools, err := ParseHTTPTools("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tools != nil {
		t.Errorf("expected nil, got %v", tools)
	}
}

func TestExecuteHTTPTool_GET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("city") != "Moscow" {
			t.Errorf("expected city=Moscow, got %q", r.URL.Query().Get("city"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"temperature":25}}`))
	}))
	defer server.Close()

	tool := HTTPToolDef{
		Name:       "weather",
		URL:        server.URL,
		Method:     "GET",
		OutputPath: "$.data.temperature",
	}

	result, err := executeHTTPTool(context.Background(), tool, map[string]any{"city": "Moscow"})
	if err != nil {
		t.Fatalf("executeHTTPTool() error = %v", err)
	}
	if result != "25" {
		t.Errorf("result = %q, want %q", result, "25")
	}
}

func TestExecuteHTTPTool_POST(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["text"] != "Hello" {
			t.Errorf("expected text=Hello, got %v", body["text"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"translation":"Привет"}`))
	}))
	defer server.Close()

	tool := HTTPToolDef{
		Name:         "translate",
		URL:          server.URL,
		Method:       "POST",
		BodyTemplate: map[string]any{"text": "{{text}}", "target": "{{target_lang}}"},
		OutputPath:   "$.translation",
	}

	result, err := executeHTTPTool(context.Background(), tool, map[string]any{
		"text":        "Hello",
		"target_lang": "ru",
	})
	if err != nil {
		t.Fatalf("executeHTTPTool() error = %v", err)
	}
	if result != "Привет" {
		t.Errorf("result = %q, want %q", result, "Привет")
	}
}

func TestExecuteHTTPTool_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	tool := HTTPToolDef{Name: "broken", URL: server.URL, Method: "GET"}
	_, err := executeHTTPTool(context.Background(), tool, nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestExtractJSONPath(t *testing.T) {
	tests := []struct {
		data string
		path string
		want string
	}{
		{`{"data":{"temp":25}}`, "$.data.temp", "25"},
		{`{"result":"ok"}`, "$.result", "ok"},
		{`{"a":{"b":{"c":"deep"}}}`, "$.a.b.c", "deep"},
		{`{"x":1}`, "$.missing", ""},
		{`invalid`, "$.x", ""},
	}
	for _, tt := range tests {
		got := extractJSONPath([]byte(tt.data), tt.path)
		if got != tt.want {
			t.Errorf("extractJSONPath(%q, %q) = %q, want %q", tt.data, tt.path, got, tt.want)
		}
	}
}

func TestExpandEnvVar(t *testing.T) {
	t.Setenv("TEST_API_KEY", "secret123")
	if got := expandEnvVar("$TEST_API_KEY"); got != "secret123" {
		t.Errorf("got %q, want %q", got, "secret123")
	}
	if got := expandEnvVar("plain-value"); got != "plain-value" {
		t.Errorf("got %q, want %q", got, "plain-value")
	}
}

func TestBuildRequestBody(t *testing.T) {
	tmpl := map[string]any{"text": "{{text}}", "target": "{{lang}}"}
	args := map[string]any{"text": "Hello", "lang": "ru"}
	result := buildRequestBody(tmpl, args)
	if result["text"] != "Hello" || result["target"] != "ru" {
		t.Errorf("result = %v", result)
	}
}

func TestRegisterHTTPTools(t *testing.T) {
	reg := NewToolRegistry(nil)
	initialCount := len(reg.ListTools())

	tools := []HTTPToolDef{
		{Name: "test_tool", Description: "A test", URL: "http://localhost", Method: "GET"},
	}
	RegisterHTTPTools(reg, tools)

	if len(reg.ListTools()) != initialCount+1 {
		t.Errorf("expected %d tools, got %d", initialCount+1, len(reg.ListTools()))
	}
	if !reg.IsHTTPTool("test_tool") {
		t.Error("expected test_tool to be HTTP tool")
	}
}
