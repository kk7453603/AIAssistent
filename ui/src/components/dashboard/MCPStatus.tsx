import { Circle, Server } from "lucide-react";
import { useSettingsStore } from "../../stores/settingsStore";

export function MCPStatus() {
  const mcpServers = useSettingsStore((s) => s.mcpServers);

  if (mcpServers.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-gray-500">
        <p>No MCP servers configured. Add servers in Settings &rarr; MCP.</p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {mcpServers.map((server) => (
        <div
          key={server.name}
          className="flex items-center gap-3 rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 px-4 py-3"
        >
          <Server className="h-5 w-5 text-gray-400 dark:text-gray-500" />
          <div className="flex-1">
            <p className="text-sm font-medium text-gray-800 dark:text-gray-200">{server.name}</p>
            <p className="text-xs text-gray-500">
              {server.transport} &middot; {server.url}
            </p>
          </div>
          <div className="flex items-center gap-1.5">
            <Circle
              className={`h-2.5 w-2.5 ${
                server.enabled ? "fill-green-400 text-green-400" : "fill-gray-400 dark:fill-gray-600 text-gray-400 dark:text-gray-600"
              }`}
            />
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {server.enabled ? "Enabled" : "Disabled"}
            </span>
          </div>
        </div>
      ))}
    </div>
  );
}
