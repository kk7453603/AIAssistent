import { create } from "zustand";
import { isTauri } from "../utils/isTauri";
import { useSettingsStore } from "./settingsStore";

async function invokeCmd<T>(cmd: string, args?: Record<string, unknown>): Promise<T> {
  if (!isTauri) throw new Error("Tauri IPC not available in browser");
  const { invoke } = await import("@tauri-apps/api/core");
  return invoke<T>(cmd, args);
}

function getApiUrl(): string {
  return useSettingsStore.getState().apiUrl;
}

interface ApiVault {
  id: string;
  name: string;
  path: string;
  enabled: boolean;
}

export interface VaultEntry {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
  modified: string;
}

export interface SearchResult {
  file_path: string;
  line_number: number;
  line_content: string;
  context: string;
}

interface VaultState {
  vaultsPath: string;
  selectedVault: string | null;
  selectedVaultId: string | null;
  vaults: VaultEntry[];
  selectedFilePath: string | null;
  fileContent: string | null;
  searchResults: SearchResult[];
  isSearching: boolean;
  expandedDirs: Record<string, VaultEntry[]>;

  /** Maps vault name -> API vault ID (used in browser mode) */
  _vaultIdMap: Record<string, string>;

  setVaultsPath: (path: string) => void;
  loadVaults: () => Promise<void>;
  selectVault: (name: string) => void;
  loadDir: (path: string) => Promise<void>;
  selectFile: (path: string) => Promise<void>;
  showDocument: (filename: string, content: string) => Promise<void>;
  searchVault: (query: string) => Promise<void>;
  clearSearch: () => void;
  getFileContent: (path: string) => Promise<string>;
}

export const useVaultStore = create<VaultState>()((set, get) => ({
  vaultsPath: "",
  selectedVault: null,
  selectedVaultId: null,
  vaults: [],
  selectedFilePath: null,
  fileContent: null,
  searchResults: [],
  isSearching: false,
  expandedDirs: {},
  _vaultIdMap: {},

  setVaultsPath: (path) => set({ vaultsPath: path }),

  loadVaults: async () => {
    if (isTauri) {
      const { vaultsPath } = get();
      if (!vaultsPath) return;
      try {
        const entries = await invokeCmd<VaultEntry[]>("list_vault_entries", {
          path: vaultsPath,
        });
        set({
          vaults: entries.filter((e) => e.is_dir),
        });
      } catch {
        set({ vaults: [] });
      }
    } else {
      try {
        const resp = await fetch(`${getApiUrl()}/v1/obsidian/vaults`);
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        const data = await resp.json() as { vaults: ApiVault[] };
        const idMap: Record<string, string> = {};
        const entries: VaultEntry[] = data.vaults.map((v) => {
          idMap[v.name] = v.id;
          return {
            name: v.name,
            path: v.id,
            is_dir: true,
            size: 0,
            modified: "",
          };
        });
        set({ vaults: entries, _vaultIdMap: idMap });
      } catch {
        set({ vaults: [] });
      }
    }
  },

  selectVault: (name) => {
    if (isTauri) {
      const { vaults, vaultsPath } = get();
      const vault = vaults.find((v) => v.name === name);
      const path = vault?.path ?? `${vaultsPath}/${name}`;
      set({
        selectedVault: name,
        selectedVaultId: null,
        selectedFilePath: null,
        fileContent: null,
        expandedDirs: {},
        searchResults: [],
      });
      get().loadDir(path);
    } else {
      const { _vaultIdMap } = get();
      const vaultId = _vaultIdMap[name] ?? null;
      set({
        selectedVault: name,
        selectedVaultId: vaultId,
        selectedFilePath: null,
        fileContent: null,
        expandedDirs: {},
        searchResults: [],
      });
      if (vaultId) {
        get().loadDir("");
      }
    }
  },

  loadDir: async (path) => {
    if (isTauri) {
      try {
        const entries = await invokeCmd<VaultEntry[]>("list_vault_entries", {
          path,
        });
        set((s) => ({
          expandedDirs: { ...s.expandedDirs, [path]: entries },
        }));
      } catch {
        // directory not accessible
      }
    } else {
      const { selectedVaultId } = get();
      if (!selectedVaultId) return;
      try {
        const url = `${getApiUrl()}/v1/obsidian/vaults/${encodeURIComponent(selectedVaultId)}/files?path=${encodeURIComponent(path)}`;
        const resp = await fetch(url);
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        const entries = await resp.json() as VaultEntry[];
        set((s) => ({
          expandedDirs: { ...s.expandedDirs, [path]: entries },
        }));
      } catch {
        // directory not accessible
      }
    }
  },

  selectFile: async (path) => {
    if (isTauri) {
      try {
        const content = await invokeCmd<string>("read_file_content", { path });
        set({ selectedFilePath: path, fileContent: content });
      } catch {
        set({ selectedFilePath: path, fileContent: "Failed to read file." });
      }
    } else {
      const { selectedVaultId } = get();
      if (!selectedVaultId) {
        set({ selectedFilePath: path, fileContent: "Failed to read file." });
        return;
      }
      try {
        const url = `${getApiUrl()}/v1/obsidian/vaults/${encodeURIComponent(selectedVaultId)}/files/content?path=${encodeURIComponent(path)}`;
        const resp = await fetch(url);
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        const data = await resp.json() as { content: string; path: string };
        set({ selectedFilePath: path, fileContent: data.content });
      } catch {
        set({ selectedFilePath: path, fileContent: "Failed to read file." });
      }
    }
  },

  showDocument: async (filename, content) => {
    // Set content immediately so preview works
    set({ selectedFilePath: filename, fileContent: content });

    // Try to find this file in a vault via backend search
    if (!isTauri) {
      try {
        // Ensure vaults are loaded
        if (get().vaults.length === 0) {
          await get().loadVaults();
        }

        // Ask backend to find the file across all vaults
        const findResp = await fetch(
          `${getApiUrl()}/v1/obsidian/find?filename=${encodeURIComponent(filename)}`,
        );
        if (findResp.ok) {
          const found = await findResp.json() as { vault_id: string; vault_name: string; path: string };

          // Select the vault and load its root entries
          set({
            selectedVault: found.vault_name,
            selectedVaultId: found.vault_id,
            selectedFilePath: found.path,
            fileContent: content,
            expandedDirs: {},
            searchResults: [],
          });

          // Load root entries for the vault tree
          await get().loadDir("");

          // Expand all parent directories along the path
          const parts = found.path.split("/");
          for (let i = 1; i < parts.length; i++) {
            const dirPath = parts.slice(0, i).join("/");
            await get().loadDir(dirPath);
          }

          // Load content from vault (overrides the passed content with vault version)
          await get().selectFile(found.path);
        }
      } catch {
        // Search failed — content is still shown via the initial set
      }
    }
  },

  searchVault: async (query) => {
    if (!query.trim()) {
      set({ searchResults: [] });
      return;
    }
    if (isTauri) {
      const { vaultsPath, selectedVault } = get();
      const searchPath = selectedVault
        ? `${vaultsPath}/${selectedVault}`
        : vaultsPath;
      set({ isSearching: true });
      try {
        const results = await invokeCmd<SearchResult[]>("search_vault", {
          vaultsPath: searchPath,
          query,
        });
        set({ searchResults: results, isSearching: false });
      } catch {
        set({ searchResults: [], isSearching: false });
      }
    } else {
      // HTTP API search not yet implemented; return empty results
      set({ searchResults: [], isSearching: false });
    }
  },

  clearSearch: () => set({ searchResults: [] }),

  getFileContent: async (path) => {
    if (isTauri) {
      return invokeCmd<string>("read_file_content", { path });
    }
    const { selectedVaultId } = get();
    if (!selectedVaultId) throw new Error("No vault selected");
    const url = `${getApiUrl()}/v1/obsidian/vaults/${encodeURIComponent(selectedVaultId)}/files/content?path=${encodeURIComponent(path)}`;
    const resp = await fetch(url);
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const data = await resp.json() as { content: string; path: string };
    return data.content;
  },
}));
