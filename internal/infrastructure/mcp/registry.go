package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// builtinTools is the fixed set of agent-native tool names.
var builtinTools = map[string]bool{
	"knowledge_search": true,
	"web_search":       true,
	"obsidian_write":   true,
	"task_tool":        true,
}

// ToolRegistry implements ports.MCPToolRegistry, combining built-in agent tools
// with dynamically discovered tools from external MCP servers.
type ToolRegistry struct {
	mu            sync.RWMutex
	clientManager *ClientManager
	builtinDefs   []ports.ToolDefinition
	httpExecutors map[string]HTTPToolDef
}

// NewToolRegistry creates a registry backed by the given client manager.
func NewToolRegistry(cm *ClientManager) *ToolRegistry {
	return &ToolRegistry{
		clientManager: cm,
		builtinDefs:   defaultBuiltinDefs(),
	}
}

// ListTools returns all available tools: built-in first, then external.
func (r *ToolRegistry) ListTools() []ports.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]ports.ToolDefinition, 0, len(r.builtinDefs)+8)
	all = append(all, r.builtinDefs...)
	if r.clientManager != nil {
		all = append(all, r.clientManager.ListExternalTools()...)
	}
	return all
}

// IsBuiltIn returns true for built-in agent tools.
func (r *ToolRegistry) IsBuiltIn(name string) bool {
	return builtinTools[strings.ToLower(strings.TrimSpace(name))]
}

// CallMCPTool routes a call to an HTTP tool or external MCP server.
func (r *ToolRegistry) CallMCPTool(ctx context.Context, name string, args map[string]any) (string, error) {
	// Check HTTP tools first.
	if r.IsHTTPTool(name) {
		return r.CallHTTPTool(ctx, name, args)
	}
	if r.clientManager == nil {
		return "", fmt.Errorf("no MCP clients configured")
	}
	return r.clientManager.CallTool(ctx, name, args)
}

// Close releases resources held by the underlying client manager.
func (r *ToolRegistry) Close() {
	if r.clientManager != nil {
		r.clientManager.Close()
	}
}

func defaultBuiltinDefs() []ports.ToolDefinition {
	return []ports.ToolDefinition{
		{
			Name:        "knowledge_search",
			Description: "Search the knowledge base (Obsidian vaults and uploaded documents)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{"type": "string", "description": "search query"},
					"limit":    map[string]any{"type": "integer", "description": "max results"},
				},
				"required": []string{"question"},
			},
			Source: "builtin",
		},
		{
			Name:        "web_search",
			Description: "Search the internet via SearXNG",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "search query"},
					"limit": map[string]any{"type": "integer", "description": "max results"},
				},
				"required": []string{"query"},
			},
			Source: "builtin",
		},
		{
			Name:        "obsidian_write",
			Description: "Create a note in an Obsidian vault",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vault":   map[string]any{"type": "string", "description": "vault id"},
					"title":   map[string]any{"type": "string", "description": "note title"},
					"content": map[string]any{"type": "string", "description": "note content in markdown"},
					"folder":  map[string]any{"type": "string", "description": "folder inside vault"},
				},
				"required": []string{"title", "content"},
			},
			Source: "builtin",
		},
		{
			Name:        "task_tool",
			Description: "Manage user tasks (create, list, get, update, delete, complete)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{"type": "string", "enum": []string{"create", "list", "get", "update", "delete", "complete"}},
					"title":  map[string]any{"type": "string"},
					"id":     map[string]any{"type": "string"},
				},
				"required": []string{"action"},
			},
			Source: "builtin",
		},
	}
}

// Ensure ToolRegistry satisfies the interface.
var _ ports.MCPToolRegistry = (*ToolRegistry)(nil)
