# UI Spec 3: Vault Browser

## Goal
File explorer for Obsidian vaults with markdown preview, full-text search, and the ability to reference notes in chat.

## Dependencies
- Spec 1 (scaffold) must be complete

## Architecture
Two data paths for vault access:
1. **Tauri IPC** (primary): Rust reads vaults directly from disk — fast, no Docker needed
2. **MCP fallback**: if Tauri can't access path, use PAA API's filesystem MCP tools

## Rust IPC Commands

```rust
#[tauri::command]
fn list_vault_entries(path: String) -> Result<Vec<VaultEntry>, String>

#[tauri::command]
fn read_file_content(path: String) -> Result<String, String>

#[tauri::command]
fn search_vault(vaults_path: String, query: String) -> Result<Vec<SearchResult>, String>
// Uses simple substring match + file path. For full-text, grep-like.
```

```rust
#[derive(Serialize)]
struct VaultEntry {
    name: String,
    path: String,
    is_dir: bool,
    size: u64,        // bytes, 0 for dirs
    modified: String,  // ISO 8601
}

#[derive(Serialize)]
struct SearchResult {
    file_path: String,
    line_number: usize,
    line_content: String,
    context: String, // ±2 lines around match
}
```

## React Components

### VaultBrowser (`src/pages/VaultBrowserPage.tsx`)
Split pane: file tree on left (resizable), content preview on right.

### FileTree (`src/components/vault/FileTree.tsx`)
- Recursive tree with expand/collapse
- Icons: folder (📁), markdown file (📝), other files (📄)
- Filter: hide `.obsidian/`, `.git/`, `.trash/`
- Click file → show preview
- Right-click context menu: "Reference in chat", "Copy path"
- Lazy loading: only load children when folder expanded

### MarkdownPreview (`src/components/vault/MarkdownPreview.tsx`)
- Renders markdown via `react-markdown` (shared with chat)
- Shows file path as breadcrumb at top
- Read-only (editing stays in Obsidian)
- Internal links (`[[note name]]`) rendered as clickable → navigate in tree
- Front matter (YAML) rendered as metadata badge

### VaultSearch (`src/components/vault/VaultSearch.tsx`)
- Search input with debounce (300ms)
- Calls Tauri `search_vault` command
- Results show file path + matching line with highlighted query
- Click result → opens file in preview

### VaultSelector (`src/components/vault/VaultSelector.tsx`)
- Dropdown showing available vaults (from config vaults_path)
- Switch between ML, Rust, architector, etc.

## "Reference in Chat" Feature
When user right-clicks a file and selects "Reference in chat":
1. Read file content via Tauri IPC
2. Insert into chat input as: `[Referencing: ML/Progress.md]\n\n{content}`
3. Switch to chat page

## Verification
1. Open vault browser → see file tree of ML vault
2. Click Progress.md → markdown renders with formatting
3. Search "transformer" → finds matches across vault files
4. Right-click file → "Reference in chat" → switches to chat with content
5. Works offline (Tauri IPC, no API needed for browsing)
