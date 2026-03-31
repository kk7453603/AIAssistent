import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, RefreshCw } from "lucide-react";
import { useSettingsStore } from "../../stores/settingsStore";
import { apiFetch } from "../../api/client";
import {
  ensureSelectedModel,
  isLikelyEmbeddingModel,
  modelKindLabel,
  type OllamaModel,
  sortModelsBySize,
} from "../../utils/ollamaModels";

function formatSize(bytes: number): string {
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(1)} GB`;
  const mb = bytes / (1024 * 1024);
  return `${mb.toFixed(0)} MB`;
}

interface RuntimeModelsResponse {
  gen_model: string;
  planner_model: string;
  embedding_model: string;
  provider: string;
  runtime_apply_supported: boolean;
}

function formatRuntimeError(error: unknown): string {
  if (!(error instanceof Error) || !error.message) {
    return "Failed to apply runtime model settings.";
  }

  try {
    const parsed = JSON.parse(error.message) as { error?: string };
    if (parsed.error) {
      return parsed.error;
    }
  } catch {
    // keep raw error message
  }

  return error.message;
}

export function ModelsTab() {
  const { ollamaUrl, genModel, plannerModel, embeddingModel, update } =
    useSettingsStore();
  const [models, setModels] = useState<OllamaModel[]>([]);
  const [loading, setLoading] = useState(false);
  const [pullName, setPullName] = useState("");
  const [pulling, setPulling] = useState(false);
  const [syncingRuntime, setSyncingRuntime] = useState(false);
  const [applyingRuntime, setApplyingRuntime] = useState(false);
  const [runtimeMessage, setRuntimeMessage] = useState("");
  const [runtimeError, setRuntimeError] = useState("");

  const fetchModels = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await fetch(`${ollamaUrl}/api/tags`);
      if (resp.ok) {
        const data = await resp.json();
        setModels(sortModelsBySize(data.models ?? []));
      }
    } catch {
      // Ollama not reachable
    } finally {
      setLoading(false);
    }
  }, [ollamaUrl]);

  const syncRuntimeModelsToStore = useCallback(async (runtime: RuntimeModelsResponse) => {
    const current = useSettingsStore.getState();
    if (current.genModel !== runtime.gen_model) {
      await current.update("genModel", runtime.gen_model);
    }
    if (current.plannerModel !== runtime.planner_model) {
      await current.update("plannerModel", runtime.planner_model);
    }
    if (current.embeddingModel !== runtime.embedding_model) {
      await current.update("embeddingModel", runtime.embedding_model);
    }
  }, []);

  const loadRuntimeModels = useCallback(async () => {
    setSyncingRuntime(true);
    setRuntimeError("");
    try {
      const runtime = await apiFetch<RuntimeModelsResponse>("/v1/settings/models");
      await syncRuntimeModelsToStore(runtime);
      setRuntimeMessage(`Runtime apply is active via ${runtime.provider}.`);
    } catch (error) {
      setRuntimeMessage("");
      setRuntimeError(formatRuntimeError(error));
    } finally {
      setSyncingRuntime(false);
    }
  }, [syncRuntimeModelsToStore]);

  useEffect(() => {
    void fetchModels();
  }, [fetchModels]);

  useEffect(() => {
    void loadRuntimeModels();
  }, [loadRuntimeModels]);

  const handlePull = async () => {
    if (!pullName.trim()) return;
    setPulling(true);
    try {
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

  const generationOptions = useMemo(
    () => ensureSelectedModel(models.filter((model) => !isLikelyEmbeddingModel(model)), genModel),
    [genModel, models],
  );
  const plannerOptions = useMemo(
    () => ensureSelectedModel(models.filter((model) => !isLikelyEmbeddingModel(model)), plannerModel),
    [models, plannerModel],
  );
  const embeddingOptions = useMemo(
    () => ensureSelectedModel(models.filter(isLikelyEmbeddingModel), embeddingModel),
    [embeddingModel, models],
  );

  const handleModelChange = useCallback(
    async (key: "genModel" | "plannerModel" | "embeddingModel", value: string) => {
      const previous = { genModel, plannerModel, embeddingModel };
      const next = { ...previous, [key]: value };

      setRuntimeMessage("");
      setRuntimeError("");
      await update(key, value);
      setApplyingRuntime(true);

      try {
        const runtime = await apiFetch<RuntimeModelsResponse>("/v1/settings/models", {
          method: "PUT",
          body: JSON.stringify({
            gen_model: next.genModel,
            planner_model: next.plannerModel,
            embedding_model: next.embeddingModel,
          }),
        });
        await syncRuntimeModelsToStore(runtime);
        setRuntimeMessage("Runtime model settings applied.");
      } catch (error) {
        await update("genModel", previous.genModel);
        await update("plannerModel", previous.plannerModel);
        await update("embeddingModel", previous.embeddingModel);
        setRuntimeError(formatRuntimeError(error));
      } finally {
        setApplyingRuntime(false);
      }
    },
    [embeddingModel, genModel, plannerModel, syncRuntimeModelsToStore, update],
  );

  return (
    <div className="space-y-6">
      {(syncingRuntime || applyingRuntime || runtimeMessage || runtimeError) && (
        <div
          className={`rounded-lg border px-3 py-2 text-sm ${
            runtimeError
              ? "border-red-300 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-300"
              : "border-blue-300 bg-blue-50 text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/40 dark:text-blue-300"
          }`}
        >
          {syncingRuntime
            ? "Syncing runtime model settings from backend..."
            : applyingRuntime
              ? "Applying runtime model settings..."
              : runtimeError || runtimeMessage}
        </div>
      )}

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
                <div className="min-w-0">
                  <div className="truncate font-mono text-sm text-gray-700 dark:text-gray-300">{m.name}</div>
                  <div className="mt-1 flex items-center gap-2 text-xs text-gray-500">
                    <span className="rounded-full border border-gray-300 px-1.5 py-0.5 dark:border-gray-700">
                      {modelKindLabel(m)}
                    </span>
                    {m.details?.parameter_size && <span>{m.details.parameter_size}</span>}
                    {m.details?.quantization_level && <span>{m.details.quantization_level}</span>}
                  </div>
                </div>
                <span className="ml-3 shrink-0 text-xs text-gray-500">
                  {m.size > 0 ? formatSize(m.size) : "Not installed"}
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
        {
          label: "Generation Model",
          key: "genModel" as const,
          value: genModel,
          options: generationOptions,
          hint: "Only chat/generation models are shown here.",
        },
        {
          label: "Planner Model",
          key: "plannerModel" as const,
          value: plannerModel,
          options: plannerOptions,
          hint: "Use a non-embedding model for planning and tool calls.",
        },
        {
          label: "Embedding Model",
          key: "embeddingModel" as const,
          value: embeddingModel,
          options: embeddingOptions,
          hint: "Only embedding models are shown here.",
        },
      ].map(({ label, key, value, options, hint }) => (
        <div key={key}>
          <label className="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-300">
            {label}
          </label>
          <select
            value={value}
            onChange={(e) => void handleModelChange(key, e.target.value)}
            disabled={syncingRuntime || applyingRuntime}
            className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
          >
            <option value="">-- select --</option>
            {options.map((model) => (
              <option key={model.name} value={model.name}>
                {model.name}
              </option>
            ))}
          </select>
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {hint}
          </p>
        </div>
      ))}
    </div>
  );
}
