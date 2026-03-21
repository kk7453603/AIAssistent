# SPEC: Plugin System (YAML/JSON Tool Definitions)

## Goal
Allow adding new tools to the agent without writing Go code. Tools defined via YAML/JSON config files that specify HTTP endpoints, shell commands, or script executions.

## Current State
- MCP tool registry supports external MCP servers (stdio, SSE, HTTP)
- Built-in tools hardcoded in `internal/infrastructure/mcp/server.go`
- Adding a new tool requires Go code changes

## Architecture

### New Package: `internal/infrastructure/plugins/`

```
internal/infrastructure/plugins/
  loader.go           — reads plugin definitions from directory
  executor.go         — executes plugin tools (HTTP, shell, script)
  registry.go         — registers plugins as MCP tools
  validator.go        — validates plugin definitions
  loader_test.go
  executor_test.go
```

### Plugin Definition Format

```yaml
# plugins/weather.yaml
name: weather
version: "1.0"
description: "Get current weather for a location"

tools:
  - name: get_weather
    description: "Get current weather for a city"
    parameters:
      type: object
      properties:
        city:
          type: string
          description: "City name"
      required: ["city"]

    executor:
      type: http                    # http | shell | script
      method: GET
      url: "https://api.weather.com/v1/current?q={{.city}}"
      headers:
        Authorization: "Bearer {{env.WEATHER_API_KEY}}"
      response_path: ".data.temperature"   # jq-like path to extract result
      timeout: 10s

  - name: run_analysis
    description: "Run custom analysis script"
    parameters:
      type: object
      properties:
        input:
          type: string
    executor:
      type: shell
      command: "python3 scripts/analyze.py --input '{{.input}}'"
      timeout: 30s
```

### Plugin Loader

```go
type PluginLoader struct {
    pluginDir string
    plugins   map[string]*PluginDef
}

func (pl *PluginLoader) LoadAll() ([]PluginDef, error)
func (pl *PluginLoader) Watch(ctx context.Context) // hot-reload on file changes
```

### Plugin Executor

```go
type PluginExecutor struct {
    httpClient *http.Client
}

func (pe *PluginExecutor) Execute(ctx context.Context, tool ToolDef, args map[string]any) (string, error)
```

Template rendering with `text/template` for parameter substitution. Environment variables accessible via `{{env.VAR_NAME}}`.

### Security
- Shell commands run in sandbox (configurable: disabled by default)
- HTTP calls: no SSRF protection needed for personal assistant, but log all outgoing requests
- `PLUGINS_ALLOW_SHELL=false` — must be explicitly enabled
- Plugin directory must be owned by current user

### Integration with MCP Registry
Plugins register as additional tools in `MCPToolRegistry`:
```go
// In bootstrap, after loading plugins:
for _, plugin := range plugins {
    for _, tool := range plugin.Tools {
        registry.RegisterTool(tool.Name, tool.Schema, pluginExecutor.Handler(tool))
    }
}
```

### Config
```
PLUGINS_ENABLED=false
PLUGINS_DIR=./plugins               # directory with YAML/JSON definitions
PLUGINS_ALLOW_SHELL=false            # allow shell executor type
PLUGINS_HOT_RELOAD=false             # watch for file changes
```

### Plugin Directory Structure
```
plugins/
  weather.yaml
  github.yaml
  custom_tools.json
```

## Tests
- Unit: YAML/JSON parsing and validation
- Unit: template rendering with parameters
- Unit: HTTP executor with httptest server
- Unit: shell executor (when enabled)
- Integration: load plugin, register tool, execute via agent
