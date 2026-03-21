import { useState } from "react";
import { Plus, Server, Trash2 } from "lucide-react";
import {
  useSettingsStore,
  type MCPServerConfig,
} from "../../stores/settingsStore";

function MCPServerRow({ server }: { server: MCPServerConfig }) {
  const { removeMCPServer, updateMCPServer } = useSettingsStore();

  return (
    <div className="flex items-center gap-3 rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 px-4 py-3">
      <Server className="h-5 w-5 shrink-0 text-gray-400 dark:text-gray-500" />
      <div className="flex-1">
        <p className="text-sm font-medium text-gray-800 dark:text-gray-200">{server.name}</p>
        <p className="text-xs text-gray-500">
          {server.transport} &middot; {server.url}
        </p>
      </div>
      <label className="flex cursor-pointer items-center gap-2">
        <input
          type="checkbox"
          checked={server.enabled}
          onChange={(e) =>
            updateMCPServer(server.name, {
              ...server,
              enabled: e.target.checked,
            })
          }
          className="h-4 w-4 rounded border-gray-300 dark:border-gray-600 bg-gray-100 dark:bg-gray-700 text-blue-500 focus:ring-0"
        />
        <span className="text-xs text-gray-500 dark:text-gray-400">Enabled</span>
      </label>
      <button
        onClick={() => removeMCPServer(server.name)}
        className="text-gray-400 dark:text-gray-600 hover:text-red-400"
      >
        <Trash2 className="h-4 w-4" />
      </button>
    </div>
  );
}

export function MCPTab() {
  const { mcpServers, addMCPServer } = useSettingsStore();
  const [adding, setAdding] = useState(false);
  const [form, setForm] = useState<MCPServerConfig>({
    name: "",
    transport: "sse",
    url: "",
    enabled: true,
  });

  const handleAdd = () => {
    if (!form.name.trim() || !form.url.trim()) return;
    addMCPServer({ ...form });
    setForm({ name: "", transport: "sse", url: "", enabled: true });
    setAdding(false);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-gray-600 dark:text-gray-300">MCP Servers</h3>
        <button
          onClick={() => setAdding(!adding)}
          className="flex items-center gap-1 text-xs text-blue-400 hover:text-blue-300"
        >
          <Plus className="h-3.5 w-3.5" />
          Add server
        </button>
      </div>

      {adding && (
        <div className="space-y-3 rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-100/50 dark:bg-gray-800/50 p-4">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="mb-1 block text-xs text-gray-500 dark:text-gray-400">Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                className="w-full rounded border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-1.5 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs text-gray-500 dark:text-gray-400">Transport</label>
              <select
                value={form.transport}
                onChange={(e) =>
                  setForm({
                    ...form,
                    transport: e.target.value as "stdio" | "sse" | "http",
                  })
                }
                className="w-full rounded border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-1.5 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
              >
                <option value="stdio">stdio</option>
                <option value="sse">SSE</option>
                <option value="http">HTTP</option>
              </select>
            </div>
          </div>
          <div>
            <label className="mb-1 block text-xs text-gray-500 dark:text-gray-400">URL / Command</label>
            <input
              type="text"
              value={form.url}
              onChange={(e) => setForm({ ...form, url: e.target.value })}
              className="w-full rounded border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-1.5 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
            />
          </div>
          <div className="flex justify-end gap-2">
            <button
              onClick={() => setAdding(false)}
              className="rounded px-3 py-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
            >
              Cancel
            </button>
            <button
              onClick={handleAdd}
              disabled={!form.name.trim() || !form.url.trim()}
              className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-500 disabled:opacity-40"
            >
              Add
            </button>
          </div>
        </div>
      )}

      <div className="space-y-2">
        {mcpServers.length === 0 && !adding ? (
          <p className="py-8 text-center text-sm text-gray-500">
            No MCP servers configured
          </p>
        ) : (
          mcpServers.map((s) => <MCPServerRow key={s.name} server={s} />)
        )}
      </div>
    </div>
  );
}
