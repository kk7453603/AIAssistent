import { useState } from "react";
import { Bot, Globe, Server, Settings } from "lucide-react";
import { GeneralTab } from "../components/settings/GeneralTab";
import { ModelsTab } from "../components/settings/ModelsTab";
import { MCPTab } from "../components/settings/MCPTab";
import { AgentTab } from "../components/settings/AgentTab";

type Tab = "general" | "models" | "mcp" | "agent";

const TABS: { key: Tab; label: string; icon: typeof Settings }[] = [
  { key: "general", label: "General", icon: Settings },
  { key: "models", label: "Models", icon: Globe },
  { key: "mcp", label: "MCP", icon: Server },
  { key: "agent", label: "Agent", icon: Bot },
];

export function SettingsPage() {
  const [tab, setTab] = useState<Tab>("general");

  return (
    <div className="flex h-full">
      <div className="w-48 shrink-0 border-r border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 p-3">
        <nav className="space-y-1">
          {TABS.map(({ key, label, icon: Icon }) => (
            <button
              key={key}
              onClick={() => setTab(key)}
              className={`flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm ${
                tab === key
                  ? "bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100"
                  : "text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800/50 hover:text-gray-700 dark:hover:text-gray-200"
              }`}
            >
              <Icon className="h-4 w-4" />
              {label}
            </button>
          ))}
        </nav>
      </div>
      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto max-w-2xl">
          {tab === "general" && <GeneralTab />}
          {tab === "models" && <ModelsTab />}
          {tab === "mcp" && <MCPTab />}
          {tab === "agent" && <AgentTab />}
        </div>
      </div>
    </div>
  );
}
