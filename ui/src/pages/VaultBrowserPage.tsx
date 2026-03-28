import { useEffect } from "react";
import { FileText } from "lucide-react";
import { isTauri } from "../utils/isTauri";
import { FileTree } from "../components/vault/FileTree";
import { MarkdownPreview } from "../components/vault/MarkdownPreview";
import { VaultSearch } from "../components/vault/VaultSearch";
import { VaultSelector } from "../components/vault/VaultSelector";
import { useVaultStore } from "../stores/vaultStore";

interface Props {
  onReferenceInChat: (content: string) => void;
  pendingFilePath?: string | null;
  onPendingFileClear?: () => void;
}

export function VaultBrowserPage({ onReferenceInChat, pendingFilePath, onPendingFileClear }: Props) {
  const {
    vaultsPath,
    selectedVault,
    selectedFilePath,
    expandedDirs,
    setVaultsPath,
    loadVaults,
    selectFile,
    getFileContent,
  } = useVaultStore();

  useEffect(() => {
    async function init() {
      if (!isTauri) {
        // In browser mode, loadVaults fetches from HTTP API directly
        // No vaultsPath needed; trigger load immediately
        loadVaults();
        return;
      }
      const { invoke } = await import("@tauri-apps/api/core");
      const config = await invoke<{ vaults_path: string }>(
        "get_default_config",
      );
      setVaultsPath(config.vaults_path);
    }
    init();
  }, [setVaultsPath, loadVaults]);

  useEffect(() => {
    if (vaultsPath) {
      loadVaults();
    }
  }, [vaultsPath, loadVaults]);

  // Handle pending file navigation from Graph page
  useEffect(() => {
    if (!pendingFilePath) return;
    selectFile(pendingFilePath);
    onPendingFileClear?.();
  }, [pendingFilePath, selectFile, onPendingFileClear]);

  const rootPath = selectedVault
    ? isTauri
      ? `${vaultsPath}/${selectedVault}`
      : ""
    : null;

  const rootEntries = rootPath !== null ? expandedDirs[rootPath] ?? [] : [];

  const handleReferenceInChat = async (filePath: string) => {
    try {
      const content = await getFileContent(filePath);
      const relativePath = filePath.replace(vaultsPath + "/", "");
      onReferenceInChat(
        `[Referencing: ${relativePath}]\n\n${content}`,
      );
    } catch {
      // failed to read
    }
  };

  return (
    <div className="flex h-full">
      {/* Left panel: tree + search */}
      <div className="flex w-72 shrink-0 flex-col border-r border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900">
        <div className="space-y-3 border-b border-gray-200 dark:border-gray-800 p-3">
          <VaultSelector />
          <VaultSearch onResultClick={(path) => selectFile(path)} />
        </div>
        <div className="flex-1 overflow-y-auto p-2">
          {selectedVault ? (
            <FileTree
              entries={rootEntries}
              onReferenceInChat={handleReferenceInChat}
            />
          ) : selectedFilePath ? (
            <div className="px-2 py-3">
              <p className="mb-2 text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Viewing Document
              </p>
              <div className="flex items-center gap-2 rounded-md bg-blue-50 dark:bg-blue-900/20 px-3 py-2 text-sm text-blue-700 dark:text-blue-400">
                <FileText className="h-4 w-4 shrink-0" />
                <span className="truncate">{selectedFilePath}</span>
              </div>
              <p className="mt-3 text-xs text-gray-400 dark:text-gray-500">
                Select a vault above to browse vault files.
              </p>
            </div>
          ) : (
            <p className="px-2 py-4 text-sm text-gray-500">
              Select a vault to browse
            </p>
          )}
        </div>
      </div>

      {/* Right panel: preview */}
      <div className="flex-1">
        <MarkdownPreview />
      </div>
    </div>
  );
}
