import { useEffect, useState } from "react";
import { Wrench } from "lucide-react";
import { listHTTPTools } from "../../api/tools";
import type { HTTPToolDef } from "../../api/types";

export function ToolsTab() {
  const [tools, setTools] = useState<HTTPToolDef[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    listHTTPTools()
      .then((data) => setTools(data ?? []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
      </div>
    );
  }

  if (tools.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 py-8 text-gray-400 dark:text-gray-500">
        <Wrench className="h-10 w-10 opacity-30" />
        <p className="text-sm">No HTTP tools configured.</p>
        <p className="text-xs">Set HTTP_TOOLS or HTTP_TOOLS_FILE env var to add tools.</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {tools.map((tool) => (
        <div
          key={tool.name}
          className="rounded-lg border border-gray-200 dark:border-gray-700 p-4"
        >
          <div className="flex items-center gap-2 mb-2">
            <span className="rounded bg-blue-100 dark:bg-blue-900/30 px-2 py-0.5 text-xs font-mono font-medium text-blue-700 dark:text-blue-400">
              {tool.method || "GET"}
            </span>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
              {tool.name}
            </h3>
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-2">
            {tool.description}
          </p>
          <div className="space-y-1 text-xs text-gray-500 dark:text-gray-400">
            <div>
              <span className="font-medium">URL:</span>{" "}
              <span className="font-mono">{tool.url}</span>
            </div>
            {tool.params && Object.keys(tool.params).length > 0 && (
              <div>
                <span className="font-medium">Params:</span>{" "}
                {Object.keys(tool.params).join(", ")}
              </div>
            )}
            {tool.output_path && (
              <div>
                <span className="font-medium">Output path:</span>{" "}
                <span className="font-mono">{tool.output_path}</span>
              </div>
            )}
            {tool.timeout_seconds > 0 && (
              <div>
                <span className="font-medium">Timeout:</span> {tool.timeout_seconds}s
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
