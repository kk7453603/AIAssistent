use tauri::Emitter;
use tauri_plugin_global_shortcut::{GlobalShortcutExt, ShortcutState};

pub fn setup_hotkeys(app: &tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    app.global_shortcut().on_shortcut("Ctrl+Space", |app, _shortcut, event| {
        if event.state == ShortcutState::Pressed {
            let _ = app.emit("toggle-quick-ask", ());
        }
    })?;

    Ok(())
}
