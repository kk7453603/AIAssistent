package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// HTTPToolDef defines an HTTP-based tool from JSON config.
type HTTPToolDef struct {
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Params         map[string]string `json:"params,omitempty"`
	BodyTemplate   map[string]any    `json:"body_template,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	OutputPath     string            `json:"output_path,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

// ParseHTTPTools parses JSON array of HTTP tool definitions.
func ParseHTTPTools(raw string) ([]HTTPToolDef, error) {
	if raw == "" {
		return nil, nil
	}
	var tools []HTTPToolDef
	if err := json.Unmarshal([]byte(raw), &tools); err != nil {
		return nil, fmt.Errorf("parse HTTP_TOOLS: %w", err)
	}
	return tools, nil
}

// LoadHTTPToolsFromFile reads and parses HTTP tool definitions from a JSON file.
func LoadHTTPToolsFromFile(path string) ([]HTTPToolDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read HTTP_TOOLS_FILE: %w", err)
	}
	return ParseHTTPTools(string(data))
}

// RegisterHTTPTools adds HTTP tool definitions and executors to the registry.
func RegisterHTTPTools(registry *ToolRegistry, tools []HTTPToolDef) {
	for _, tool := range tools {
		def := toolDefFromHTTP(tool)
		registry.addHTTPTool(def, tool)
	}
}

func toolDefFromHTTP(t HTTPToolDef) ports.ToolDefinition {
	properties := make(map[string]any)
	var required []string

	if t.Method == "GET" && len(t.Params) > 0 {
		for name, typeHint := range t.Params {
			properties[name] = map[string]any{"type": typeHint}
			required = append(required, name)
		}
	}

	if t.Method == "POST" && len(t.BodyTemplate) > 0 {
		for key := range t.BodyTemplate {
			// Extract placeholder names from template values.
			if s, ok := t.BodyTemplate[key].(string); ok && strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") {
				paramName := strings.Trim(s, "{} ")
				properties[paramName] = map[string]any{"type": "string"}
				required = append(required, paramName)
			}
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return ports.ToolDefinition{
		Name:        t.Name,
		Description: t.Description,
		InputSchema: schema,
		Source:       "http",
	}
}

// addHTTPTool registers an HTTP tool definition and its executor.
func (r *ToolRegistry) addHTTPTool(def ports.ToolDefinition, tool HTTPToolDef) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.builtinDefs = append(r.builtinDefs, def)

	if r.httpExecutors == nil {
		r.httpExecutors = make(map[string]HTTPToolDef)
	}
	r.httpExecutors[def.Name] = tool
}

// CallHTTPTool executes an HTTP tool by name.
func (r *ToolRegistry) CallHTTPTool(ctx context.Context, name string, args map[string]any) (string, error) {
	r.mu.RLock()
	tool, ok := r.httpExecutors[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("http tool %q not found", name)
	}
	return executeHTTPTool(ctx, tool, args)
}

// IsHTTPTool checks if the named tool is an HTTP tool.
func (r *ToolRegistry) IsHTTPTool(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.httpExecutors[name]
	return ok
}

func executeHTTPTool(ctx context.Context, tool HTTPToolDef, args map[string]any) (string, error) {
	timeout := time.Duration(tool.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	client := &http.Client{Timeout: timeout}

	url := tool.URL
	var body io.Reader

	switch strings.ToUpper(tool.Method) {
	case "GET":
		if len(args) > 0 {
			params := make([]string, 0, len(args))
			for k, v := range args {
				params = append(params, fmt.Sprintf("%s=%v", k, v))
			}
			sep := "?"
			if strings.Contains(url, "?") {
				sep = "&"
			}
			url += sep + strings.Join(params, "&")
		}
	case "POST":
		bodyMap := buildRequestBody(tool.BodyTemplate, args)
		bodyJSON, err := json.Marshal(bodyMap)
		if err != nil {
			return "", fmt.Errorf("marshal body: %w", err)
		}
		body = bytes.NewReader(bodyJSON)
	default:
		return "", fmt.Errorf("unsupported method: %s", tool.Method)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(tool.Method), url, body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply headers with env var expansion.
	for k, v := range tool.Headers {
		req.Header.Set(k, expandEnvVar(v))
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http tool %q: %w", tool.Name, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("http tool %q returned status %d: %s", tool.Name, resp.StatusCode, string(respBody))
	}

	result := string(respBody)

	// Apply output_path if specified.
	if tool.OutputPath != "" {
		if extracted := extractJSONPath(respBody, tool.OutputPath); extracted != "" {
			result = extracted
		}
	}

	return result, nil
}

func buildRequestBody(template map[string]any, args map[string]any) map[string]any {
	if len(template) == 0 {
		return args
	}

	result := make(map[string]any, len(template))
	for k, v := range template {
		s, ok := v.(string)
		if ok && strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}") {
			paramName := strings.Trim(s, "{} ")
			if val, exists := args[paramName]; exists {
				result[k] = val
				continue
			}
		}
		result[k] = v
	}
	return result
}

func expandEnvVar(val string) string {
	if strings.HasPrefix(val, "$") {
		envName := strings.TrimPrefix(val, "$")
		return os.Getenv(envName)
	}
	return val
}

// extractJSONPath extracts a value using simple dot-notation path (e.g., "$.data.temperature").
func extractJSONPath(data []byte, path string) string {
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")

	var obj any
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}

	parts := strings.Split(path, ".")
	current := obj

	for _, part := range parts {
		if part == "" {
			continue
		}

		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = m[part]
		if !ok {
			return ""
		}
	}

	switch v := current.(type) {
	case string:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(b)
	}
}
