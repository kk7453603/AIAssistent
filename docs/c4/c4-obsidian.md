# Level 3 — Obsidian Integration

## Описание

Управление Obsidian vault: регистрация, синхронизация (ручная/автоматическая), создание заметок через агента, браузер файлов. Vault-конфигурация хранится в JSON-файлах.

## Component Diagram

```mermaid
flowchart TB
    subgraph ObsidianSystem["Obsidian Integration (API Server)"]
        Handlers["ObsidianHandlers\n─────────\nList / Upsert / Remove\nSync / CreateNote\nListFiles / FileContent"]
        Config["VaultConfig\n─────────\nJSON registry\nobsidian_config.json"]
        State["SyncState\n─────────\nPer-vault state\nfile hashes"]
        NoteWriter["ObsidianNoteWriter\n─────────\nCreateNote()\nagent tool interface"]
    end

    subgraph External["External"]
        VaultFS["Obsidian Vault\n(Filesystem)"]
        IngestUC["IngestUseCase\n─────────\nIngestFromSource()"]
    end

    Handlers --> Config
    Handlers --> State
    Handlers --> VaultFS
    Handlers -->|"sync: ingest new files"| IngestUC
    NoteWriter --> VaultFS
    NoteWriter -->|"agent obsidian_write"| Handlers

    UI["Tauri UI\nVaultBrowserPage"] -->|"HTTP"| Handlers
```

## Key Flow: Vault Sync

```mermaid
sequenceDiagram
    participant User
    participant API as ObsidianHandlers
    participant FS as Vault Filesystem
    participant State as SyncState
    participant Ingest as IngestUseCase

    User->>API: POST /v1/obsidian/sync/{id}
    API->>FS: listMarkdownFiles()
    API->>State: loadState(vault_id)
    loop each .md file
        API->>FS: hashFile(path)
        alt hash changed or new
            API->>Ingest: IngestFromSource(obsidian, file)
            API->>State: update hash
        end
    end
    API->>State: saveState(vault_id)
    API-->>User: 200 { synced: N }
```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| ObsidianHandlers | `internal/adapters/http/obsidian_handlers.go` |
| ObsidianAdapter | `internal/infrastructure/source/obsidian/adapter.go` |
| IngestUseCase | `internal/core/usecase/ingest.go` |
