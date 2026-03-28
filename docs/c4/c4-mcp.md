# Level 3 — MCP Integration

## Описание

Двусторонняя MCP-интеграция: PAA выступает как MCP-сервер (для Claude Code, Cursor, OpenWebUI) и как MCP-клиент (подключается к внешним серверам: filesystem, code-runner, GitHub). Дополнительно: JSON-defined HTTP API tools.

## Component Diagram

```mermaid
flowchart TB
    subgraph MCPSystem["MCP Integration"]
        MCPServer["MCPServer\n─────────\nStreamable HTTP on /mcp\ntools: knowledge_search,\nweb_search, obsidian_write,\ntask_create/list/get/\nupdate/delete/complete"]
        MCPRegistry["MCPToolRegistry\n─────────\nListTools() — built-in + MCP\nIsBuiltIn()\nCallMCPTool()"]
        MCPClient["MCPClient\n─────────\nConnect to external\nMCP servers\n(stdio/SSE/HTTP)"]
        HTTPTools["HTTPToolsPlugin\n─────────\nJSON-defined HTTP tools\nenv var expansion\nJSONPath output"]
    end

    subgraph ExternalMCP["External MCP Servers"]
        FSServer["filesystem\nread/write files"]
        CodeRunner["code-runner\nexecute python/bash"]
        GitHub["github\nrepo operations"]
    end

    subgraph Clients["MCP Clients"]
        ClaudeCode["Claude Code"]
        Cursor["Cursor"]
        OWUI["OpenWebUI"]
    end

    Clients -->|"MCP protocol"| MCPServer
    MCPServer --> MCPRegistry
    MCPRegistry --> MCPClient
    MCPClient --> FSServer & CodeRunner & GitHub
    MCPRegistry --> HTTPTools

    AgentChat["AgentChatUseCase"] -->|"CallMCPTool()"| MCPRegistry
```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| MCPServer | `internal/infrastructure/mcp/server.go` |
| MCPClient | `internal/infrastructure/mcp/client.go` |
| MCPToolRegistry | `internal/infrastructure/mcp/registry.go` |
| HTTPToolsPlugin | `internal/infrastructure/mcp/http_tools.go` |
| MCP ports | `internal/core/ports/mcp.go` |
