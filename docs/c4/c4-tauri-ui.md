# Level 3 — Tauri Desktop UI

## Описание

Десктопное приложение на Tauri (Rust + React). Chat с SSE streaming, Obsidian browser, settings, dashboard, 3D граф знаний. Zustand для state management, typed API client.

## Component Diagram

```mermaid
flowchart TB
    subgraph TauriApp["Tauri Desktop App"]
        subgraph Pages["Pages"]
            Chat["ChatPage\n─────────\nSSE streaming\nthink blocks\ntool status"]
            Vault["VaultBrowserPage\n─────────\nvault management\nfile browser"]
            Settings["SettingsPage\n─────────\nmodel, MCP, agent\ntheme, language"]
            Dashboard["DashboardPage\n─────────\ntool usage stats"]
            Graph["GraphPage\n─────────\n3D knowledge graph\nreact-force-graph-3d"]
            QuickAsk["QuickAskPage\n─────────\nquick question"]
        end

        subgraph Stores["Zustand Stores"]
            ChatStore["chatStore\n─────────\nmessages, streaming,\nmodel selection"]
            ConvStore["conversationStore\n─────────\nhistory, search"]
            VaultSt["vaultStore\n─────────\nvault CRUD, files"]
            SettingsSt["settingsStore\n─────────\ntheme, models, MCP"]
            DashSt["dashboardStore\n─────────\ntool usage data"]
            GraphSt["graphStore\n─────────\nnodes, edges, filters"]
        end

        subgraph APIClient["API Client"]
            Client["client.ts\n─────────\napiFetch()\nSSE streaming"]
            GraphAPI["graph.ts\n─────────\nfetchGraph()"]
            HealthAPI["health.ts\n─────────\nhealthCheck()"]
            Types["types.ts\n─────────\nall API types"]
        end
    end

    Chat --> ChatStore --> Client
    Vault --> VaultSt --> Client
    Settings --> SettingsSt --> Client
    Dashboard --> DashSt --> Client
    Graph --> GraphSt --> GraphAPI

    Client -->|"HTTP/SSE"| API["PAA API Server"]
```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| App + routing | `ui/src/App.tsx` |
| ChatPage | `ui/src/pages/ChatPage.tsx` |
| VaultBrowserPage | `ui/src/pages/VaultBrowserPage.tsx` |
| SettingsPage | `ui/src/pages/SettingsPage.tsx` |
| DashboardPage | `ui/src/pages/DashboardPage.tsx` |
| GraphPage | `ui/src/pages/GraphPage.tsx` |
| API client | `ui/src/api/client.ts` |
| Types | `ui/src/api/types.ts` |
| Stores | `ui/src/stores/` |
