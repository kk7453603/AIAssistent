# JSON HTTP Tools Plugin System

**Дата:** 2026-03-28
**Этап:** 6 — Уровень 3.4: Автоматизация и агенты
**Статус:** approved

## Проблема

Подключение новых tools к агенту требует написания MCP сервера или Go кода. Для простых HTTP API (weather, translate, webhooks) это overkill. Нужен способ определять tools через JSON config.

## Решение

JSON-defined HTTP tools: пользователь описывает endpoint (URL, method, params, headers) в JSON config. System автоматически регистрирует tools в существующем ToolRegistry. Агент вызывает их как обычные tools.

## Tool Definition Format

```json
[
  {
    "name": "weather",
    "description": "Get current weather for a city",
    "url": "https://api.weather.com/current",
    "method": "GET",
    "params": {"city": "string", "units": "string"},
    "headers": {"X-API-Key": "$WEATHER_API_KEY"},
    "output_path": "$.data.temperature",
    "timeout_seconds": 10
  },
  {
    "name": "translate",
    "description": "Translate text between languages",
    "url": "https://api.translate.com/v1/translate",
    "method": "POST",
    "body_template": {"text": "{{text}}", "source": "{{source_lang}}", "target": "{{target_lang}}"},
    "headers": {"Authorization": "Bearer $TRANSLATE_KEY"},
    "output_path": "$.translation"
  }
]
```

### Поля

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Tool name for agent (must be unique) |
| `description` | Yes | Human-readable description shown to agent |
| `url` | Yes | HTTP endpoint URL |
| `method` | Yes | GET or POST |
| `params` | No | Query params for GET: `{name: type_hint}` |
| `body_template` | No | JSON body for POST with `{{placeholder}}` substitution |
| `headers` | No | Static headers. `$ENV_VAR` expanded from environment |
| `output_path` | No | JSONPath to extract result (default: full response body) |
| `timeout_seconds` | No | Request timeout (default: 30) |

### Environment Variable Expansion

Header values starting with `$` are expanded from env vars at startup:
- `"X-API-Key": "$WEATHER_API_KEY"` → `"X-API-Key": "abc123"` (from env)
- If env var not set → empty string (tool still registers, call may fail with auth error)

### Body Template Substitution

`{{placeholder}}` in `body_template` are replaced with values from agent's tool call arguments:
- Template: `{"text": "{{text}}", "target": "{{target_lang}}"}`
- Agent calls with: `{"text": "Hello", "target_lang": "ru"}`
- Result: `{"text": "Hello", "target": "ru"}`

### Output Path (simple JSONPath)

Dot-notation path to extract value from JSON response:
- `"$.data.temperature"` → extracts `response["data"]["temperature"]`
- `"$.results[0].text"` → extracts first element's text
- Empty/omitted → return full response body as string

## Конфигурация

```env
# Inline JSON
HTTP_TOOLS=[{"name":"weather","description":"Get weather","url":"https://api.weather.com/current","method":"GET","params":{"city":"string"}}]

# Or from file (takes precedence if both set)
HTTP_TOOLS_FILE=/path/to/tools.json
```

## Интеграция с ToolRegistry

`HTTPToolLoader` parses config and registers each tool:

1. Parse JSON → `[]HTTPToolDef`
2. For each tool:
   - Build `domain.ToolSchema` with name, description, and params as JSON schema
   - Register executor function in `ToolRegistry`
3. Executor: receives tool call args → builds HTTP request → call → parse response → apply output_path → return result string

### HTTPToolDef domain type

```go
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
```

## Файловая структура

```
internal/infrastructure/mcp/http_tools.go       # HTTPToolLoader + HTTPToolExecutor
internal/infrastructure/mcp/http_tools_test.go
internal/config/config.go                        # HTTP_TOOLS, HTTP_TOOLS_FILE fields
internal/bootstrap/bootstrap.go                  # wire loader
```

## Что НЕ входит

- UI для создания/редактирования HTTP tools
- OAuth2 / complex auth flows (только static headers + env vars)
- Response transformation beyond simple JSONPath (no scripting)
- Rate limiting per tool
- Tool-level error retries (uses global resilience)
