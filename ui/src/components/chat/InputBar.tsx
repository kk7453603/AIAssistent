import {
  type KeyboardEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { ChevronDown, Send, Square } from "lucide-react";
import { useSettingsStore } from "../../stores/settingsStore";
import { isLikelyEmbeddingModel, type OllamaModel } from "../../utils/ollamaModels";

interface Props {
  onSend: (content: string, model: string) => void;
  onStop: () => void;
  isStreaming: boolean;
}

const PAA_MODEL = "paa-agent";

export function InputBar({ onSend, onStop, isStreaming }: Props) {
  const [text, setText] = useState("");
  const [models, setModels] = useState<string[]>([PAA_MODEL]);
  const [selectedModel, setSelectedModel] = useState(PAA_MODEL);
  const [modelsLoading, setModelsLoading] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const ollamaUrl = useSettingsStore((s) => s.ollamaUrl);

  // Fetch available models from Ollama
  useEffect(() => {
    let cancelled = false;
    async function fetchModels() {
      setModelsLoading(true);
      try {
        const resp = await fetch(`${ollamaUrl}/api/tags`);
        if (!resp.ok) throw new Error("Failed to fetch models");
        const data = await resp.json();
        const names: string[] = (data.models ?? [])
          .filter((m: OllamaModel) => !isLikelyEmbeddingModel(m))
          .map((m: OllamaModel) => m.name);
        if (!cancelled) {
          // Always include paa-agent at the top
          const all = [PAA_MODEL, ...names.filter((n) => n !== PAA_MODEL)];
          setModels(all);
        }
      } catch {
        // Keep default list if fetch fails
        if (!cancelled) {
          setModels([PAA_MODEL]);
        }
      } finally {
        if (!cancelled) setModelsLoading(false);
      }
    }
    fetchModels();
    return () => {
      cancelled = true;
    };
  }, [ollamaUrl]);

  const resizeTextarea = useCallback(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
  }, []);

  useEffect(resizeTextarea, [text, resizeTextarea]);

  const handleSend = () => {
    const trimmed = text.trim();
    if (!trimmed || isStreaming) return;
    onSend(trimmed, selectedModel);
    setText("");
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="border-t border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 p-2 md:p-3">
      <div className="mb-2 flex items-center gap-2">
        <div className="relative">
          <select
            value={selectedModel}
            onChange={(e) => setSelectedModel(e.target.value)}
            disabled={modelsLoading}
            className="appearance-none rounded border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 py-1 pl-2.5 pr-7 text-xs text-gray-600 dark:text-gray-300 outline-none focus:border-blue-500"
          >
            {models.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
          <ChevronDown className="pointer-events-none absolute right-1.5 top-1/2 h-3 w-3 -translate-y-1/2 text-gray-400 dark:text-gray-500" />
        </div>
        {modelsLoading && (
          <span className="text-xs text-gray-500">Loading models...</span>
        )}
      </div>
      <div className="flex w-full items-end gap-2">
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Type a message..."
          rows={1}
          className="flex-1 resize-none rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-4 py-2.5 text-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 outline-none focus:border-blue-500"
        />
        {isStreaming ? (
          <button
            onClick={onStop}
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-red-600 text-white hover:bg-red-500"
          >
            <Square className="h-4 w-4" />
          </button>
        ) : (
          <button
            onClick={handleSend}
            disabled={!text.trim()}
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-40 disabled:hover:bg-blue-600"
          >
            <Send className="h-4 w-4" />
          </button>
        )}
      </div>
    </div>
  );
}
