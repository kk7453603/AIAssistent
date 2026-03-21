# UI Spec 5: System Tray + Hotkeys + Mini-Window

## Goal
Quick access to the AI assistant from anywhere: system tray icon, global hotkey (Ctrl+Space), and a floating mini-window for quick questions.

## Dependencies
- Spec 1 (scaffold), Spec 2 (chat — reuses useChat hook)

## System Tray

### Rust Implementation (`src-tauri/src/tray.rs`)

```rust
use tauri::{
    menu::{Menu, MenuItem},
    tray::{TrayIcon, TrayIconBuilder},
    Manager,
};

pub fn setup_tray(app: &tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    let open = MenuItem::with_id(app, "open", "Open PAA", true, None::<&str>)?;
    let quick_ask = MenuItem::with_id(app, "quick_ask", "Quick Ask (Ctrl+Space)", true, None::<&str>)?;
    let quit = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&open, &quick_ask, &quit])?;

    TrayIconBuilder::new()
        .icon(app.default_window_icon().unwrap().clone())
        .menu(&menu)
        .tooltip("Personal AI Assistant")
        .on_menu_event(|app, event| {
            match event.id.as_ref() {
                "open" => { /* show main window */ }
                "quick_ask" => { /* show mini window */ }
                "quit" => app.exit(0),
                _ => {}
            }
        })
        .build(app)?;
    Ok(())
}
```

Tray behavior:
- Left click → show/hide main window
- Right click → context menu (Open, Quick Ask, Quit)
- App close → minimize to tray (not quit)

## Global Hotkeys

### Rust Implementation (`src-tauri/src/hotkeys.rs`)

```rust
use tauri_plugin_global_shortcut::ShortcutState;

pub fn setup_hotkeys(app: &tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    app.global_shortcut().on_shortcut("Ctrl+Space", |app, shortcut, event| {
        if event.state == ShortcutState::Pressed {
            app.emit("toggle-quick-ask", ()).unwrap();
        }
    })?;
    Ok(())
}
```

Required Tauri plugins:
- `tauri-plugin-global-shortcut` — global keyboard shortcuts

## Mini-Window (Quick Ask)

### QuickAskWindow
A small, always-on-top floating window:
- Size: 600x400px
- Position: centered on screen
- Always on top
- Transparent/blurred background (if platform supports)
- Single input field + streaming response area
- Escape → close
- Uses the same `useChat` hook as main chat

### Tauri Window Config
```json
{
  "label": "quick-ask",
  "title": "Quick Ask",
  "width": 600,
  "height": 400,
  "center": true,
  "alwaysOnTop": true,
  "decorations": false,
  "transparent": true,
  "visible": false,
  "skipTaskbar": true
}
```

### React Component (`src/pages/QuickAskPage.tsx`)
```typescript
function QuickAskPage() {
  const { messages, isStreaming, sendMessage, stopStreaming } = useChat(apiUrl);

  // Escape to close
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        getCurrentWindow().hide();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  return (
    <div className="h-screen flex flex-col bg-black/80 backdrop-blur rounded-xl p-4">
      <div className="flex-1 overflow-auto">
        {messages.map(msg => <CompactMessage key={msg.id} {...msg} />)}
      </div>
      <QuickInput onSend={sendMessage} disabled={isStreaming} />
    </div>
  );
}
```

### Window Management (`src/hooks/useQuickAsk.ts`)
```typescript
import { listen } from '@tauri-apps/api/event';
import { WebviewWindow } from '@tauri-apps/api/webviewWindow';

// Listen for hotkey event from Rust
listen('toggle-quick-ask', async () => {
  const quickAsk = await WebviewWindow.getByLabel('quick-ask');
  if (quickAsk) {
    const visible = await quickAsk.isVisible();
    if (visible) {
      await quickAsk.hide();
    } else {
      await quickAsk.show();
      await quickAsk.setFocus();
    }
  }
});
```

## App Lifecycle
- App start → show main window + setup tray + register hotkeys
- Close main window → hide to tray (not quit)
- Tray "Quit" or Ctrl+Q → actually quit
- Ctrl+Space → toggle quick ask mini-window

## Verification
1. App shows tray icon after launch
2. Closing main window → app stays in tray
3. Tray click → shows main window
4. Ctrl+Space → mini-window appears centered, always on top
5. Type question in mini-window → streaming response
6. Escape → mini-window hides
7. Ctrl+Space again → mini-window reappears with previous conversation
