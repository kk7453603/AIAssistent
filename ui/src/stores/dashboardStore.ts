import { create } from "zustand";
import type { ToolStatusDelta } from "../api/types";

export interface ToolActivity {
  id: string;
  timestamp: number;
  tool: string;
  status: "running" | "ok" | "error";
  duration?: number;
  input?: string;
  output?: string;
}

export interface ToolStat {
  tool: string;
  totalCalls: number;
  avgDuration: number;
  errorRate: number;
}

interface DashboardState {
  activities: ToolActivity[];
  toolStats: ToolStat[];

  addActivity: (toolStatus: ToolStatusDelta) => void;
  updateActivity: (tool: string, status: "ok" | "error") => void;
  fetchToolStats: (apiUrl: string) => Promise<void>;
  clearActivities: () => void;
}

export const useDashboardStore = create<DashboardState>()((set) => ({
  activities: [],
  toolStats: [],

  addActivity: (ts) => {
    set((s) => {
      // If this is a terminal status, update the existing running entry
      if (ts.status === "ok" || ts.status === "error") {
        const idx = s.activities.findIndex(
          (a) => a.tool === ts.tool && a.status === "running",
        );
        if (idx >= 0) {
          const activities = [...s.activities];
          activities[idx] = {
            ...activities[idx],
            status: ts.status,
            duration: Date.now() - activities[idx].timestamp,
          };
          return { activities };
        }
      }
      // Otherwise add a new entry
      const activity: ToolActivity = {
        id: `${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
        timestamp: Date.now(),
        tool: ts.tool,
        status: ts.status,
      };
      return {
        activities: [activity, ...s.activities].slice(0, 200),
      };
    });
  },

  updateActivity: (tool, status) => {
    set((s) => {
      const activities = [...s.activities];
      const idx = activities.findIndex(
        (a) => a.tool === tool && a.status === "running",
      );
      if (idx >= 0) {
        activities[idx] = {
          ...activities[idx],
          status,
          duration: Date.now() - activities[idx].timestamp,
        };
      }
      return { activities };
    });
  },

  fetchToolStats: async (apiUrl) => {
    try {
      const resp = await fetch(`${apiUrl}/metrics`);
      if (!resp.ok) return;
      const text = await resp.text();

      const stats = new Map<string, { total: number; errors: number; durationSum: number }>();

      for (const line of text.split("\n")) {
        // paa_agent_tool_calls_total{service="api",status="ok",tool="web_search"} 2
        // Labels may appear in any order, so extract them individually.
        const totalMatch = line.match(
          /paa_agent_tool_calls_total\{([^}]+)\}\s+(\d+)/,
        );
        if (totalMatch) {
          const labels = totalMatch[1];
          const toolName = labels.match(/tool="([^"]+)"/)?.[1];
          const status = labels.match(/status="([^"]+)"/)?.[1];
          const count = parseInt(totalMatch[2], 10);
          if (toolName) {
            const entry = stats.get(toolName) ?? { total: 0, errors: 0, durationSum: 0 };
            entry.total += count;
            if (status === "error") entry.errors += count;
            stats.set(toolName, entry);
          }
        }

        // paa_agent_tool_call_duration_seconds_sum{...tool="knowledge_search"...} 1.234
        const durMatch = line.match(
          /paa_agent_tool_call_duration_seconds_sum\{([^}]+)\}\s+([\d.]+)/,
        );
        if (durMatch) {
          const toolName = durMatch[1].match(/tool="([^"]+)"/)?.[1];
          if (toolName) {
            const entry = stats.get(toolName) ?? { total: 0, errors: 0, durationSum: 0 };
            entry.durationSum = parseFloat(durMatch[2]);
            stats.set(toolName, entry);
          }
        }
      }

      const toolStats: ToolStat[] = Array.from(stats.entries()).map(
        ([tool, s]) => ({
          tool,
          totalCalls: s.total,
          avgDuration: s.total > 0 ? s.durationSum / s.total : 0,
          errorRate: s.total > 0 ? s.errors / s.total : 0,
        }),
      );

      set({ toolStats });
    } catch {
      // metrics endpoint unavailable
    }
  },

  clearActivities: () => set({ activities: [] }),
}));
