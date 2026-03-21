# UI Spec 4: Tool Dashboard + Settings

## Goal
Real-time visualization of agent tool execution and a settings panel for managing models, MCP servers, and agent configuration.

## Dependencies
- Spec 1 (scaffold), Spec 2 (chat — for tool status events)

## Tool Dashboard

### DashboardPage (`src/pages/DashboardPage.tsx`)
Shows agent activity in real-time and recent history.

### AgentActivityFeed (`src/components/dashboard/ActivityFeed.tsx`)
- Live feed of tool calls from the current/recent conversations
- Each entry: timestamp, tool name, status (running/ok/error), duration, truncated output
- Color coded: green=ok, red=error, blue=running
- Click entry → expand to see full input/output

### ToolStats (`src/components/dashboard/ToolStats.tsx`)
- Fetches from PAA Prometheus metrics endpoint (`/metrics`)
- Parses `paa_agent_tool_call_total` and `paa_agent_tool_call_duration_seconds`
- Shows: total calls per tool, average duration, error rate
- Simple bar chart (use `recharts` library — lightweight)

### MCPServerStatus (`src/components/dashboard/MCPStatus.tsx`)
- Lists connected MCP servers with status (connected/disconnected)
- Shows tool count per server
- Source: parse PAA API logs or add a `/v1/mcp/status` endpoint (future)
- For MVP: static display from config

## Settings

### SettingsPage (`src/pages/SettingsPage.tsx`)
Tabbed settings panel.

### GeneralSettings (`src/components/settings/GeneralTab.tsx`)
- PAA API URL (default: http://localhost:8080)
- Obsidian vaults path (file picker via Tauri dialog)
- Theme: light/dark/system
- Language: ru/en

### ModelSettings (`src/components/settings/ModelsTab.tsx`)
- List available models from Ollama (`/api/tags` endpoint)
- Select generation model (OLLAMA_GEN_MODEL equivalent)
- Select planner model (OLLAMA_PLANNER_MODEL equivalent)
- Select embedding model
- Pull new model: input model name → calls `ollama pull` (via Tauri command)
- Show model size, quantization, last used

### MCPSettings (`src/components/settings/MCPTab.tsx`)
- List configured MCP servers (from MCP_SERVERS config)
- Add/remove/edit server: name, transport (stdio/sse/http), URL/command
- Test connection button
- Shows discovered tools per server

### AgentSettings (`src/components/settings/AgentTab.tsx`)
- Max iterations slider (1-20)
- Timeout sliders (planner, tool, total)
- Intent router toggle
- Web search toggle
- Knowledge top-k slider

## Settings Storage
Settings stored via Tauri `tauri-plugin-store` (JSON file in app data dir):
```typescript
import { Store } from '@tauri-apps/plugin-store';
const store = new Store('settings.json');
await store.set('api_url', 'http://localhost:8080');
const url = await store.get<string>('api_url');
```

## Verification
1. Dashboard shows tool activity from recent conversation
2. Tool stats display with correct counts
3. Settings persist across app restarts
4. Model list shows available Ollama models
5. MCP server config editable and saveable
