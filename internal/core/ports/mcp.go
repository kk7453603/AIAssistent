package ports

import "context"

// ToolDefinition describes an available tool (built-in or from an MCP server).
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
	Source      string                 `json:"source"` // "builtin" or MCP server name
}

// MCPToolRegistry aggregates built-in agent tools and dynamically discovered
// tools from connected MCP servers.
type MCPToolRegistry interface {
	// ListTools returns all available tools (built-in + MCP).
	ListTools() []ToolDefinition
	// IsBuiltIn checks whether the named tool is a built-in agent tool.
	IsBuiltIn(name string) bool
	// CallMCPTool invokes a tool on an external MCP server.
	CallMCPTool(ctx context.Context, name string, args map[string]any) (string, error)
}
