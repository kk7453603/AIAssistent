import { useCallback, useEffect, useState } from "react";
import { Loader2, RefreshCw } from "lucide-react";
import { useSettingsStore } from "../../stores/settingsStore";

interface OllamaModel {
  name: string;
  size: number;
  details?: {
    quantization_level?: string;
  };
}

function formatSize(bytes: number): string {
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(1)} GB`;
  const mb = bytes / (1024 * 1024);
  return `${mb.toFixed(0)} MB`;
}

export function ModelsTab() {
  const { apiUrl, genModel, plannerModel, embeddingModel, update } =
    useSettingsStore();
  const [models, setModels] = useState<OllamaModel[]>([]);
  const [loading, setLoading] = useState(false);
  const [pullName, setPullName] = useState("");
  const [pulling, setPulling] = useState(false);

  const fetchModels = useCallback(async () => {
    setLoading(true);
    try {
      const ollamaUrl = apiUrl.replace(/:8080$/, ":11434");
      const resp = await fetch(`${ollamaUrl}/api/tags`);
      if (resp.ok) {
        const data = await resp.json();
        setModels(data.models ?? []);
      }
    } catch {
      // Ollama not reachable
    } finally {
      setLoading(false);
    }
  }, [apiUrl]);

  useEffect(() => {
    fetchModels();
  }, [fetchModels]);

  const handlePull = async () => {
    if (!pullName.trim()) return;
    setPulling(true);
    try {
      const ollamaUrl = apiUrl.replace(/:8080$/, ":11434");
      await fetch(`${ollamaUrl}/api/pull`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: pullName }),
      });
      setPullName("");
      await fetchModels();
    } catch {
      // pull failed
    } finally {
      setPulling(false);
    }
  };

  const modelNames = models.map((m) => m.name);

  return (
    <div className="space-y-6">
      {/* Available models */}
      <div>
        <div className="mb-2 flex items-center justify-between">
          <h3 className="text-sm font-medium text-gray-600 dark:text-gray-300">Available Models</h3>
          <button
            onClick={fetchModels}
            disabled={loading}
            className="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </button>
        </div>
        <div className="max-h-48 overflow-y-auto rounded-lg border border-gray-200 dark:border-gray-800">
          {models.length === 0 ? (
            <p className="px-3 py-4 text-sm text-gray-500">
              {loading ? "Loading..." : "No models found. Is Ollama running?"}
            </p>
          ) : (
            models.map((m) => (
              <div
                key={m.name}
                className="flex items-center justify-between border-b border-gray-200 dark:border-gray-800 px-3 py-2 last:border-b-0"
              >
                <span className="font-mono text-sm text-gray-700 dark:text-gray-300">{m.name}</span>
                <span className="text-xs text-gray-500">
                  {formatSize(m.size)}
                  {m.details?.quantization_level && ` \u00b7 ${m.details.quantization_level}`}
                </span>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Pull new model */}
      <div>
        <label className="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-300">
          Pull Model
        </label>
        <div className="flex gap-2">
          <input
            type="text"
            value={pullName}
            onChange={(e) => setPullName(e.target.value)}
            placeholder="e.g. llama3.1:8b"
            className="flex-1 rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500 outline-none focus:border-blue-500"
          />
          <button
            onClick={handlePull}
            disabled={pulling || !pullName.trim()}
            className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-4 py-2 text-sm text-white hover:bg-blue-500 disabled:opacity-40"
          >
            {pulling && <Loader2 className="h-4 w-4 animate-spin" />}
            Pull
          </button>
        </div>
      </div>

      {/* Model selection */}
      {[
        { label: "Generation Model", key: "genModel" as const, value: genModel },
        { label: "Planner Model", key: "plannerModel" as const, value: plannerModel },
        { label: "Embedding Model", key: "embeddingModel" as const, value: embeddingModel },
      ].map(({ label, key, value }) => (
        <div key={key}>
          <label className="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-300">
            {label}
          </label>
          <select
            value={value}
            onChange={(e) => update(key, e.target.value)}
            className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
          >
            <option value="">-- select --</option>
            {modelNames.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </div>
      ))}
    </div>
  );
}
