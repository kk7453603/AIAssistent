import { lazy, Suspense, useCallback, useEffect, useState } from "react";
import { Activity, FolderOpen, MessageSquare, Network, Settings } from "lucide-react";
import { ConnectionStatus } from "./components/ConnectionStatus";
import { ChatPage } from "./pages/ChatPage";
import { VaultBrowserPage } from "./pages/VaultBrowserPage";
import { DashboardPage } from "./pages/DashboardPage";
import { SettingsPage } from "./pages/SettingsPage";
import { useSettingsStore } from "./stores/settingsStore";
import { isTauri } from "./utils/isTauri";
import { setApiUrl } from "./api/client";

const GraphPage = lazy(() =>
  import("./pages/GraphPage").then((m) => ({ default: m.GraphPage })),
);

type Page = "chat" | "vault" | "dashboard" | "graph" | "settings";

export default function App() {
  const [page, setPage] = useState<Page>("chat");
  const [pendingRef, setPendingRef] = useState<string | null>(null);
  const [pendingVaultPath, setPendingVaultPath] = useState<string | null>(null);
  const [pendingDocId, setPendingDocId] = useState<string | null>(null);
  const loadSettings = useSettingsStore((s) => s.load);
  const apiUrl = useSettingsStore((s) => s.apiUrl);
  const theme = useSettingsStore((s) => s.theme);

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  useEffect(() => {
    setApiUrl(apiUrl);
  }, [apiUrl]);

  // Apply theme class to document root
  useEffect(() => {
    const root = document.documentElement;

    const applyTheme = (isDark: boolean) => {
      if (isDark) {
        root.classList.add("dark");
      } else {
        root.classList.remove("dark");
      }
    };

    if (theme === "dark") {
      applyTheme(true);
    } else if (theme === "light") {
      applyTheme(false);
    } else {
      // system
      const mq = window.matchMedia("(prefers-color-scheme: dark)");
      applyTheme(mq.matches);
      const handler = (e: MediaQueryListEvent) => applyTheme(e.matches);
      mq.addEventListener("change", handler);
      return () => mq.removeEventListener("change", handler);
    }
  }, [theme]);

  // Toggle quick-ask window from main window context (Tauri only)
  useEffect(() => {
    if (!isTauri) return;

    let cancelled = false;
    let cleanup: (() => void) | undefined;

    (async () => {
      const { listen } = await import("@tauri-apps/api/event");
      const { WebviewWindow } = await import("@tauri-apps/api/webviewWindow");

      if (cancelled) return;

      const unlisten = await listen("toggle-quick-ask", async () => {
        const quickAsk = await WebviewWindow.getByLabel("quick-ask");
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
      cleanup = unlisten;
    })();

    return () => {
      cancelled = true;
      cleanup?.();
    };
  }, []);

  const handleReferenceInChat = useCallback((content: string) => {
    setPendingRef(content);
    setPage("chat");
  }, []);

  const navItems: { key: Page; label: string; icon: typeof MessageSquare }[] = [
    { key: "chat", label: "Chat", icon: MessageSquare },
    { key: "vault", label: "Vaults", icon: FolderOpen },
    { key: "dashboard", label: "Dashboard", icon: Activity },
    { key: "graph", label: "Graph", icon: Network },
    { key: "settings", label: "Settings", icon: Settings },
  ];

  return (
    <div className="flex h-screen flex-col bg-white dark:bg-gray-950">
      <header className="flex items-center justify-between border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-4 py-3">
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
            <span className="hidden md:inline">Personal AI Assistant</span>
            <span className="md:hidden">PAA</span>
          </h1>
          <nav className="flex gap-1 overflow-x-auto">
            {navItems.map(({ key, label, icon: Icon }) => (
              <button
                key={key}
                onClick={() => setPage(key)}
                className={`flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm md:px-3 ${
                  page === key
                    ? "bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100"
                    : "text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800/50 hover:text-gray-700 dark:hover:text-gray-200"
                }`}
              >
                <Icon className="h-4 w-4 shrink-0" />
                <span className="hidden md:inline">{label}</span>
              </button>
            ))}
          </nav>
        </div>
        <ConnectionStatus />
      </header>

      <main className="flex-1 overflow-hidden">
        {page === "chat" && (
          <ChatPage
            pendingReference={pendingRef}
            onReferenceClear={() => setPendingRef(null)}
          />
        )}
        {page === "vault" && (
          <VaultBrowserPage
            onReferenceInChat={handleReferenceInChat}
            pendingFilePath={pendingVaultPath}
            onPendingFileClear={() => setPendingVaultPath(null)}
            pendingDocId={pendingDocId}
            onPendingDocClear={() => setPendingDocId(null)}
          />
        )}
        {page === "dashboard" && <DashboardPage />}
        {page === "graph" && (
          <Suspense
            fallback={
              <div className="flex h-full items-center justify-center">
                <div className="h-8 w-8 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
              </div>
            }
          >
            <GraphPage
              onNavigateToVault={(path) => {
                setPendingVaultPath(path);
                setPage("vault");
              }}
              onViewDocument={(docId) => {
                setPendingDocId(docId);
                setPage("vault");
              }}
            />
          </Suspense>
        )}
        {page === "settings" && <SettingsPage />}
      </main>
    </div>
  );
}
