import { useEffect } from "react";
import { isTauri } from "../utils/isTauri";

export function useQuickAskToggle() {
  useEffect(() => {
    if (!isTauri) return;

    let cleanup: (() => void) | undefined;

    (async () => {
      const { listen } = await import("@tauri-apps/api/event");
      const { getCurrentWebviewWindow } = await import(
        "@tauri-apps/api/webviewWindow"
      );

      const unlisten = await listen("toggle-quick-ask", async () => {
        const current = getCurrentWebviewWindow();
        if (current.label !== "quick-ask") return;

        const visible = await current.isVisible();
        if (visible) {
          await current.hide();
        } else {
          await current.show();
          await current.setFocus();
        }
      });
      cleanup = unlisten;
    })();

    return () => cleanup?.();
  }, []);
}

export function useEscapeToHide() {
  useEffect(() => {
    if (!isTauri) return;

    const handler = async (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        const { getCurrentWebviewWindow } = await import(
          "@tauri-apps/api/webviewWindow"
        );
        const current = getCurrentWebviewWindow();
        if (current.label === "quick-ask") {
          current.hide();
        }
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);
}
