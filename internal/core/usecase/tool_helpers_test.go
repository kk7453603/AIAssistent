package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// mockFSRegistry implements ports.MCPToolRegistry for filesystem probe tests.
type mockFSRegistry struct {
	tools   []ports.ToolDefinition
	results map[string]string // tool name -> result
	errors  map[string]error
}

func (m *mockFSRegistry) ListTools() []ports.ToolDefinition {
	return m.tools
}

func (m *mockFSRegistry) IsBuiltIn(name string) bool {
	return false
}

func (m *mockFSRegistry) CallMCPTool(_ context.Context, name string, args map[string]any) (string, error) {
	if m.errors != nil {
		if err, ok := m.errors[name]; ok {
			return "", err
		}
	}
	if m.results != nil {
		if result, ok := m.results[name]; ok {
			return result, nil
		}
	}
	return "", fmt.Errorf("tool %s not found", name)
}

func TestProbeFilesystemContext_NoTool(t *testing.T) {
	// Registry without list_allowed_directories — should return "".
	registry := &mockFSRegistry{
		tools: []ports.ToolDefinition{
			{Name: "some_other_tool"},
		},
	}
	// Reset cache to avoid interference from other tests.
	resetFSContextCache()

	result := probeFilesystemContext(context.Background(), registry)
	if result != "" {
		t.Fatalf("expected empty result when tool is absent, got %q", result)
	}
}

func TestProbeFilesystemContext_WithPaths(t *testing.T) {
	registry := &mockFSRegistry{
		tools: []ports.ToolDefinition{
			{Name: "list_allowed_directories"},
			{Name: "list_directory"},
		},
		results: map[string]string{
			"list_allowed_directories": "/allowed/vaults\n/allowed/code\n",
			"list_directory":           "[DIR] ML\n[DIR] Rust\n[DIR] neovim\n",
		},
	}
	resetFSContextCache()

	result := probeFilesystemContext(context.Background(), registry)

	if result == "" {
		t.Fatal("expected non-empty filesystem context")
	}
	if !strings.Contains(result, "Available filesystem paths:") {
		t.Errorf("expected header, got: %q", result)
	}
	if !strings.Contains(result, "/allowed/vaults") {
		t.Errorf("expected /allowed/vaults in result, got: %q", result)
	}
	if !strings.Contains(result, "[DIR] ML") {
		t.Errorf("expected [DIR] ML in result, got: %q", result)
	}
	if !strings.Contains(result, "[DIR] Rust") {
		t.Errorf("expected [DIR] Rust in result, got: %q", result)
	}
}

func TestProbeFilesystemContext_CachesResult(t *testing.T) {
	callCount := 0
	registry := &mockFSRegistry{
		tools: []ports.ToolDefinition{
			{Name: "list_allowed_directories"},
			{Name: "list_directory"},
		},
	}
	// Override CallMCPTool via a counting wrapper.
	countingRegistry := &countingMockFSRegistry{
		inner:     registry,
		callCount: &callCount,
		results: map[string]string{
			"list_allowed_directories": "/allowed/vaults\n",
			"list_directory":           "[DIR] ML\n",
		},
	}
	resetFSContextCache()

	// First call — should probe.
	probeFilesystemContext(context.Background(), countingRegistry)
	firstCount := callCount

	// Second call immediately — should hit cache, no additional calls.
	probeFilesystemContext(context.Background(), countingRegistry)
	if callCount != firstCount {
		t.Fatalf("expected cached result on second call (no extra CallMCPTool), but call count went from %d to %d", firstCount, callCount)
	}
}

func TestProbeFilesystemContext_CacheExpires(t *testing.T) {
	registry := &mockFSRegistry{
		tools: []ports.ToolDefinition{
			{Name: "list_allowed_directories"},
			{Name: "list_directory"},
		},
		results: map[string]string{
			"list_allowed_directories": "/allowed/vaults\n",
			"list_directory":           "[DIR] ML\n",
		},
	}

	// Manually set cache with an already-expired entry.
	fsContextMu.Lock()
	fsContextCached = "stale"
	fsContextExpiresAt = time.Now().Add(-1 * time.Second)
	fsContextMu.Unlock()

	result := probeFilesystemContext(context.Background(), registry)
	if result == "stale" {
		t.Fatal("expected cache to be refreshed after expiry, but got stale value")
	}
	if !strings.Contains(result, "/allowed/vaults") {
		t.Errorf("expected fresh result with /allowed/vaults, got: %q", result)
	}
}

func TestMaybeSummarize_Short(t *testing.T) {
	result := maybeSummarize("short text", 2048)
	if result != "short text" {
		t.Error("short text should pass through unchanged")
	}
}

func TestMaybeSummarize_Long(t *testing.T) {
	long := strings.Repeat("word ", 1000)
	result := maybeSummarize(long, 2048)
	if len(result) > 2200 {
		t.Errorf("should be truncated, got len=%d", len(result))
	}
	if !strings.Contains(result, "[...truncated") {
		t.Error("should contain truncation marker")
	}
}

func TestIsRecoverableToolError(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`{"error":"Access denied - path outside allowed directories"}`, true},
		{`{"error":"file not found: /foo/bar"}`, true},
		{`{"error":"connection refused"}`, false},
		{`{"answer":"some result"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isRecoverableToolError(tt.input); got != tt.expected {
				t.Errorf("isRecoverableToolError(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAddFSHintToError(t *testing.T) {
	result := addFSHintToError("path error", "/allowed/vaults")
	if !strings.Contains(result, "Hint") || !strings.Contains(result, "/allowed/vaults") {
		t.Error("should contain hint with paths")
	}
	// Empty context should not modify
	result2 := addFSHintToError("error", "")
	if result2 != "error" {
		t.Error("empty context should pass through")
	}
}

// resetFSContextCache clears the package-level cache for test isolation.
func resetFSContextCache() {
	fsContextMu.Lock()
	fsContextCached = ""
	fsContextExpiresAt = time.Time{}
	fsContextMu.Unlock()
}

// countingMockFSRegistry wraps mockFSRegistry and counts CallMCPTool invocations.
type countingMockFSRegistry struct {
	inner     *mockFSRegistry
	callCount *int
	results   map[string]string
}

func (c *countingMockFSRegistry) ListTools() []ports.ToolDefinition {
	return c.inner.ListTools()
}

func (c *countingMockFSRegistry) IsBuiltIn(name string) bool {
	return false
}

func (c *countingMockFSRegistry) CallMCPTool(ctx context.Context, name string, args map[string]any) (string, error) {
	*c.callCount++
	if c.results != nil {
		if result, ok := c.results[name]; ok {
			return result, nil
		}
	}
	return "", fmt.Errorf("tool %s not found", name)
}
