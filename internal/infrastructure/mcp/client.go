package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// ServerConfig describes an external MCP server to connect to.
type ServerConfig struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"` // "stdio" or "sse" or "streamable-http"
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	URL       string   `json:"url,omitempty"`
}

// connectedServer holds a live MCP client and the tools it exposes.
type connectedServer struct {
	name   string
	client mcpclient.MCPClient
	tools  []mcpgo.Tool
}

// ClientManager manages connections to external MCP servers.
type ClientManager struct {
	mu      sync.RWMutex
	servers []connectedServer
}

// NewClientManager creates a ClientManager and connects to each configured
// MCP server.  Failures to connect are logged but do not prevent startup.
func NewClientManager(ctx context.Context, configs []ServerConfig) *ClientManager {
	cm := &ClientManager{}
	for _, cfg := range configs {
		if err := cm.addServer(ctx, cfg); err != nil {
			slog.Error("mcp_client_connect_failed", "server", cfg.Name, "err", err)
		}
	}
	return cm
}

func (cm *ClientManager) addServer(ctx context.Context, cfg ServerConfig) error {
	var c mcpclient.MCPClient
	var err error

	switch cfg.Transport {
	case "stdio":
		if cfg.Command == "" {
			return fmt.Errorf("stdio transport requires command")
		}
		c, err = mcpclient.NewStdioMCPClient(cfg.Command, nil, cfg.Args...)
		if err != nil {
			return fmt.Errorf("create stdio client %s: %w", cfg.Name, err)
		}
	case "sse":
		if cfg.URL == "" {
			return fmt.Errorf("sse transport requires url")
		}
		c, err = mcpclient.NewSSEMCPClient(cfg.URL)
		if err != nil {
			return fmt.Errorf("create sse client %s: %w", cfg.Name, err)
		}
	case "streamable-http", "http", "":
		if cfg.URL == "" {
			return fmt.Errorf("streamable-http transport requires url")
		}
		httpTransport, tErr := transport.NewStreamableHTTP(cfg.URL)
		if tErr != nil {
			return fmt.Errorf("create http transport %s: %w", cfg.Name, tErr)
		}
		c = mcpclient.NewClient(httpTransport)
	default:
		return fmt.Errorf("unsupported transport %q for server %s", cfg.Transport, cfg.Name)
	}

	initReq := mcpgo.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcpgo.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcpgo.Implementation{
		Name:    "paa-mcp-client",
		Version: "1.0.0",
	}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		_ = c.Close()
		return fmt.Errorf("initialize %s: %w", cfg.Name, err)
	}

	toolsResult, err := c.ListTools(ctx, mcpgo.ListToolsRequest{})
	if err != nil {
		_ = c.Close()
		return fmt.Errorf("list tools %s: %w", cfg.Name, err)
	}

	slog.Info("mcp_client_connected",
		"server", cfg.Name,
		"tools", len(toolsResult.Tools),
	)

	cm.mu.Lock()
	cm.servers = append(cm.servers, connectedServer{
		name:   cfg.Name,
		client: c,
		tools:  toolsResult.Tools,
	})
	cm.mu.Unlock()

	return nil
}

// ListExternalTools returns tool definitions from all connected MCP servers.
func (cm *ClientManager) ListExternalTools() []ports.ToolDefinition {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var defs []ports.ToolDefinition
	for _, srv := range cm.servers {
		for _, t := range srv.tools {
			schema := make(map[string]any)
			if t.InputSchema.Properties != nil {
				raw, _ := json.Marshal(t.InputSchema)
				_ = json.Unmarshal(raw, &schema)
			}
			defs = append(defs, ports.ToolDefinition{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: schema,
				Source:       srv.name,
			})
		}
	}
	return defs
}

// CallTool routes a tool call to the MCP server that owns it.
func (cm *ClientManager) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, srv := range cm.servers {
		for _, t := range srv.tools {
			if t.Name != name {
				continue
			}
			result, err := srv.client.CallTool(ctx, mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      name,
					Arguments: args,
				},
			})
			if err != nil {
				return "", fmt.Errorf("mcp call %s/%s: %w", srv.name, name, err)
			}
			if result.IsError {
				return "", fmt.Errorf("mcp tool %s/%s returned error: %s", srv.name, name, contentToText(result.Content))
			}
			return contentToText(result.Content), nil
		}
	}
	return "", fmt.Errorf("mcp tool %q not found on any connected server", name)
}

// Close disconnects from all external MCP servers.
func (cm *ClientManager) Close() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for _, srv := range cm.servers {
		_ = srv.client.Close()
	}
	cm.servers = nil
}

func contentToText(content []mcpgo.Content) string {
	var out string
	for _, c := range content {
		if tc, ok := c.(mcpgo.TextContent); ok {
			if out != "" {
				out += "\n"
			}
			out += tc.Text
		}
	}
	return out
}
