# UI Spec 1: Tauri + React + TypeScript Scaffold

## Goal
Bootstrap the Tauri desktop application with React, TypeScript, Tailwind CSS, and verified connection to the PAA API. This is the foundation all other UI specs build on.

## What gets built
- Tauri v2 project in `ui/` directory
- React 19 + TypeScript + Vite + Tailwind CSS
- Rust backend with basic IPC commands
- PAA API client library (`src/api/`) with TypeScript types
- Health check on startup: verify PAA API is reachable
- Minimal window: header with connection status, empty content area
- Build + dev scripts in package.json

## File structure

```
ui/
├── src-tauri/
│   ├── Cargo.toml              # tauri v2, serde, reqwest
│   ├── tauri.conf.json         # window config, permissions
│   ├── capabilities/
│   │   └── default.json        # fs, http, notification permissions
│   └── src/
│       ├── main.rs             # tauri::Builder, setup, invoke_handler
│       └── commands.rs         # health_check, get_config IPC commands
│
├── src/
│   ├── main.tsx                # ReactDOM.createRoot
│   ├── App.tsx                 # top-level layout + router placeholder
│   ├── api/
│   │   ├── client.ts           # base fetch wrapper with error handling
│   │   ├── types.ts            # TypeScript types matching PAA OpenAPI spec
│   │   └── health.ts           # GET /healthz
│   ├── components/
│   │   ├── Layout.tsx          # sidebar + main area shell
│   │   └── ConnectionStatus.tsx # green/red indicator
│   ├── hooks/
│   │   └── useApiHealth.ts     # polls /healthz every 10s
│   └── styles/
│       └── globals.css         # tailwind directives
│
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
├── tailwind.config.ts
└── postcss.config.js
```

## API types (from PAA OpenAPI spec)

```typescript
// src/api/types.ts
export interface ChatMessage {
  role: 'system' | 'user' | 'assistant' | 'tool';
  content: string;
}

export interface ChatCompletionRequest {
  model: string;
  messages: ChatMessage[];
  stream?: boolean;
  metadata?: {
    user_id?: string;
    conversation_id?: string;
  };
}

export interface ChatCompletionChoice {
  index: number;
  message: ChatMessage;
  finish_reason: string;
}

export interface ChatCompletionResponse {
  id: string;
  choices: ChatCompletionChoice[];
}

export interface HealthResponse {
  status: string;
}

export interface ToolStatusDelta {
  tool: string;
  status: 'running' | 'ok' | 'error';
}
```

## Rust IPC commands

```rust
#[tauri::command]
async fn check_api_health(api_url: String) -> Result<bool, String> {
    let resp = reqwest::get(format!("{}/healthz", api_url))
        .await.map_err(|e| e.to_string())?;
    Ok(resp.status().is_success())
}

#[tauri::command]
fn get_default_config() -> AppConfig {
    AppConfig {
        api_url: "http://localhost:8080".to_string(),
        vaults_path: dirs::home_dir()
            .map(|h| h.join("ObsidanVaults").to_string_lossy().to_string())
            .unwrap_or_default(),
    }
}
```

## Tauri config (tauri.conf.json key parts)

```json
{
  "productName": "PAA - Personal AI Assistant",
  "version": "0.1.0",
  "identifier": "com.paa.desktop",
  "build": {
    "frontendDist": "../dist"
  },
  "app": {
    "windows": [{
      "title": "Personal AI Assistant",
      "width": 1200,
      "height": 800,
      "minWidth": 800,
      "minHeight": 600
    }]
  }
}
```

## Verification
1. `cd ui && npm run tauri dev` — app window opens
2. Connection status shows green when PAA API running on :8080
3. Connection status shows red when API is down
4. `npm run build` + `npm run tauri build` — produces binary
