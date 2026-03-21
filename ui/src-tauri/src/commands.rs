use serde::Serialize;
use std::fs;
use std::path::Path;
use std::time::UNIX_EPOCH;

#[derive(Serialize)]
pub struct AppConfig {
    pub api_url: String,
    pub vaults_path: String,
}

#[derive(Serialize)]
pub struct VaultEntry {
    pub name: String,
    pub path: String,
    pub is_dir: bool,
    pub size: u64,
    pub modified: String,
}

#[derive(Serialize)]
pub struct SearchResult {
    pub file_path: String,
    pub line_number: usize,
    pub line_content: String,
    pub context: String,
}

const HIDDEN_DIRS: &[&str] = &[".obsidian", ".git", ".trash"];

#[tauri::command]
pub async fn check_api_health(api_url: String) -> Result<bool, String> {
    let resp = reqwest::get(format!("{}/healthz", api_url))
        .await
        .map_err(|e| e.to_string())?;
    Ok(resp.status().is_success())
}

#[tauri::command]
pub fn get_default_config() -> AppConfig {
    AppConfig {
        api_url: "http://localhost:8080".to_string(),
        vaults_path: dirs::home_dir()
            .map(|h| h.join("ObsidianVaults").to_string_lossy().to_string())
            .unwrap_or_default(),
    }
}

#[tauri::command]
pub fn list_vault_entries(path: String) -> Result<Vec<VaultEntry>, String> {
    let dir = Path::new(&path);
    if !dir.is_dir() {
        return Err(format!("Not a directory: {}", path));
    }

    let mut entries = Vec::new();
    let read_dir = fs::read_dir(dir).map_err(|e| e.to_string())?;

    for entry in read_dir {
        let entry = entry.map_err(|e| e.to_string())?;
        let name = entry.file_name().to_string_lossy().to_string();

        if HIDDEN_DIRS.contains(&name.as_str()) || name.starts_with('.') {
            continue;
        }

        let meta = entry.metadata().map_err(|e| e.to_string())?;
        let modified = meta
            .modified()
            .ok()
            .and_then(|t| t.duration_since(UNIX_EPOCH).ok())
            .map(|d| {
                chrono::DateTime::from_timestamp(d.as_secs() as i64, 0)
                    .map(|dt| dt.to_rfc3339())
                    .unwrap_or_default()
            })
            .unwrap_or_default();

        entries.push(VaultEntry {
            name,
            path: entry.path().to_string_lossy().to_string(),
            is_dir: meta.is_dir(),
            size: if meta.is_dir() { 0 } else { meta.len() },
            modified,
        });
    }

    entries.sort_by(|a, b| match (a.is_dir, b.is_dir) {
        (true, false) => std::cmp::Ordering::Less,
        (false, true) => std::cmp::Ordering::Greater,
        _ => a.name.to_lowercase().cmp(&b.name.to_lowercase()),
    });

    Ok(entries)
}

#[tauri::command]
pub fn read_file_content(path: String) -> Result<String, String> {
    fs::read_to_string(&path).map_err(|e| format!("Failed to read {}: {}", path, e))
}

#[tauri::command]
pub fn search_vault(vaults_path: String, query: String) -> Result<Vec<SearchResult>, String> {
    let query_lower = query.to_lowercase();
    let mut results = Vec::new();
    search_recursive(Path::new(&vaults_path), &query_lower, &mut results)?;
    results.truncate(100);
    Ok(results)
}

fn search_recursive(
    dir: &Path,
    query: &str,
    results: &mut Vec<SearchResult>,
) -> Result<(), String> {
    if results.len() >= 100 {
        return Ok(());
    }

    let read_dir = fs::read_dir(dir).map_err(|e| e.to_string())?;

    for entry in read_dir {
        let entry = entry.map_err(|e| e.to_string())?;
        let name = entry.file_name().to_string_lossy().to_string();

        if HIDDEN_DIRS.contains(&name.as_str()) || name.starts_with('.') {
            continue;
        }

        let path = entry.path();
        let meta = entry.metadata().map_err(|e| e.to_string())?;

        if meta.is_dir() {
            search_recursive(&path, query, results)?;
        } else if name.ends_with(".md") || name.ends_with(".txt") {
            if let Ok(content) = fs::read_to_string(&path) {
                let lines: Vec<&str> = content.lines().collect();
                for (i, line) in lines.iter().enumerate() {
                    if results.len() >= 100 {
                        return Ok(());
                    }
                    if line.to_lowercase().contains(query) {
                        let start = i.saturating_sub(2);
                        let end = (i + 3).min(lines.len());
                        let context = lines[start..end].join("\n");
                        results.push(SearchResult {
                            file_path: path.to_string_lossy().to_string(),
                            line_number: i + 1,
                            line_content: line.to_string(),
                            context,
                        });
                    }
                }
            }
        }
    }

    Ok(())
}
