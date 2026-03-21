import { create } from "zustand";
import { isTauri } from "../utils/isTauri";

async function invokeCmd<T>(cmd: string, args?: Record<string, unknown>): Promise<T> {
  if (!isTauri) throw new Error("Tauri IPC not available in browser");
  const { invoke } = await import("@tauri-apps/api/core");
  return invoke<T>(cmd, args);
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
  vaults: VaultEntry[];
  selectedFilePath: string | null;
  fileContent: string | null;
  searchResults: SearchResult[];
  isSearching: boolean;
  expandedDirs: Record<string, VaultEntry[]>;

  setVaultsPath: (path: string) => void;
  loadVaults: () => Promise<void>;
  selectVault: (name: string) => void;
  loadDir: (path: string) => Promise<void>;
  selectFile: (path: string) => Promise<void>;
  searchVault: (query: string) => Promise<void>;
  clearSearch: () => void;
  getFileContent: (path: string) => Promise<string>;
}

export const useVaultStore = create<VaultState>()((set, get) => ({
  vaultsPath: "",
  selectedVault: null,
  vaults: [],
  selectedFilePath: null,
  fileContent: null,
  searchResults: [],
  isSearching: false,
  expandedDirs: {},

  setVaultsPath: (path) => set({ vaultsPath: path }),

  loadVaults: async () => {
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
  },

  selectVault: (name) => {
    const { vaults, vaultsPath } = get();
    const vault = vaults.find((v) => v.name === name);
    const path = vault?.path ?? `${vaultsPath}/${name}`;
    set({
      selectedVault: name,
      selectedFilePath: null,
      fileContent: null,
      expandedDirs: {},
      searchResults: [],
    });
    get().loadDir(path);
  },

  loadDir: async (path) => {
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
  },

  selectFile: async (path) => {
    try {
      const content = await invokeCmd<string>("read_file_content", { path });
      set({ selectedFilePath: path, fileContent: content });
    } catch {
      set({ selectedFilePath: path, fileContent: "Failed to read file." });
    }
  },

  searchVault: async (query) => {
    const { vaultsPath, selectedVault } = get();
    if (!query.trim()) {
      set({ searchResults: [] });
      return;
    }
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
  },

  clearSearch: () => set({ searchResults: [] }),

  getFileContent: async (path) => {
    return invokeCmd<string>("read_file_content", { path });
  },
}));
