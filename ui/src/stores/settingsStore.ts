import { create } from "zustand";
import { isTauri } from "../utils/isTauri";

type Store = { get<T>(key: string): Promise<T | null | undefined>; set(key: string, value: unknown): Promise<void> };

export interface MCPServerConfig {
  name: string;
  transport: "stdio" | "sse" | "http";
  url: string;
  enabled: boolean;
}

export interface Settings {
  apiUrl: string;
  ollamaUrl: string;
  vaultsPath: string;
  theme: "light" | "dark" | "system";
  language: "ru" | "en";

  genModel: string;
  plannerModel: string;
  embeddingModel: string;

  mcpServers: MCPServerConfig[];

  agentMaxIterations: number;
  agentPlannerTimeout: number;
  agentToolTimeout: number;
  agentTotalTimeout: number;
  intentRouterEnabled: boolean;
  webSearchEnabled: boolean;
  knowledgeTopK: number;
}

const DEFAULT_SETTINGS: Settings = {
  apiUrl: "http://localhost:8080",
  ollamaUrl: "http://localhost:11434",
  vaultsPath: "",
  theme: "dark",
  language: "ru",
  genModel: "llama3.1:8b",
  plannerModel: "llama3.1:8b",
  embeddingModel: "nomic-embed-text",
  mcpServers: [],
  agentMaxIterations: 10,
  agentPlannerTimeout: 30,
  agentToolTimeout: 30,
  agentTotalTimeout: 120,
  intentRouterEnabled: true,
  webSearchEnabled: true,
  knowledgeTopK: 5,
};

interface SettingsState extends Settings {
  loaded: boolean;
  load: () => Promise<void>;
  update: <K extends keyof Settings>(key: K, value: Settings[K]) => Promise<void>;
  addMCPServer: (server: MCPServerConfig) => Promise<void>;
  removeMCPServer: (name: string) => Promise<void>;
  updateMCPServer: (name: string, server: MCPServerConfig) => Promise<void>;
}

let storeInstance: Store | null = null;

async function getStore(): Promise<Store | null> {
  if (!isTauri) return null;
  if (!storeInstance) {
    const { load } = await import("@tauri-apps/plugin-store");
    storeInstance = await load("settings.json");
  }
  return storeInstance;
}

export const useSettingsStore = create<SettingsState>()((set, get) => ({
  ...DEFAULT_SETTINGS,
  loaded: false,

  load: async () => {
    try {
      const store = await getStore();
      if (!store) { set({ loaded: true }); return; }
      const saved: Partial<Settings> = {};
      for (const key of Object.keys(DEFAULT_SETTINGS) as (keyof Settings)[]) {
        const val = await store.get<Settings[typeof key]>(key);
        if (val !== null && val !== undefined) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          (saved as any)[key] = val;
        }
      }
      set({ ...saved, loaded: true });
    } catch {
      set({ loaded: true });
    }
  },

  update: async (key, value) => {
    set({ [key]: value } as Partial<SettingsState>);
    try {
      const store = await getStore();
      await store?.set(key, value);
    } catch {
      // persist failed
    }
  },

  addMCPServer: async (server) => {
    const servers = [...get().mcpServers, server];
    set({ mcpServers: servers });
    try {
      const store = await getStore();
      await store?.set("mcpServers", servers);
    } catch {
      // persist failed
    }
  },

  removeMCPServer: async (name) => {
    const servers = get().mcpServers.filter((s) => s.name !== name);
    set({ mcpServers: servers });
    try {
      const store = await getStore();
      await store?.set("mcpServers", servers);
    } catch {
      // persist failed
    }
  },

  updateMCPServer: async (name, server) => {
    const servers = get().mcpServers.map((s) => (s.name === name ? server : s));
    set({ mcpServers: servers });
    try {
      const store = await getStore();
      await store?.set("mcpServers", servers);
    } catch {
      // persist failed
    }
  },
}));
