import { useSettingsStore } from "../../stores/settingsStore";

function SliderField({
  label,
  value,
  min,
  max,
  step,
  unit,
  onChange,
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  unit?: string;
  onChange: (v: number) => void;
}) {
  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <label className="text-sm font-medium text-gray-600 dark:text-gray-300">{label}</label>
        <span className="text-sm text-gray-500 dark:text-gray-400">
          {value}
          {unit && ` ${unit}`}
        </span>
      </div>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-full accent-blue-500"
      />
    </div>
  );
}

function ToggleField({
  label,
  description,
  value,
  onChange,
}: {
  label: string;
  description?: string;
  value: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label className="flex cursor-pointer items-center justify-between rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 px-4 py-3">
      <div>
        <p className="text-sm font-medium text-gray-800 dark:text-gray-200">{label}</p>
        {description && (
          <p className="text-xs text-gray-500">{description}</p>
        )}
      </div>
      <div
        className={`relative h-6 w-11 rounded-full transition-colors ${
          value ? "bg-blue-600" : "bg-gray-300 dark:bg-gray-700"
        }`}
        onClick={() => onChange(!value)}
      >
        <div
          className={`absolute top-0.5 h-5 w-5 rounded-full bg-white transition-transform ${
            value ? "translate-x-5" : "translate-x-0.5"
          }`}
        />
      </div>
    </label>
  );
}

export function AgentTab() {
  const {
    agentMaxIterations,
    agentPlannerTimeout,
    agentToolTimeout,
    agentTotalTimeout,
    intentRouterEnabled,
    webSearchEnabled,
    knowledgeTopK,
    update,
  } = useSettingsStore();

  return (
    <div className="space-y-6">
      <SliderField
        label="Max Iterations"
        value={agentMaxIterations}
        min={1}
        max={20}
        step={1}
        onChange={(v) => update("agentMaxIterations", v)}
      />

      <SliderField
        label="Planner Timeout"
        value={agentPlannerTimeout}
        min={5}
        max={120}
        step={5}
        unit="s"
        onChange={(v) => update("agentPlannerTimeout", v)}
      />

      <SliderField
        label="Tool Timeout"
        value={agentToolTimeout}
        min={5}
        max={120}
        step={5}
        unit="s"
        onChange={(v) => update("agentToolTimeout", v)}
      />

      <SliderField
        label="Total Timeout"
        value={agentTotalTimeout}
        min={30}
        max={600}
        step={30}
        unit="s"
        onChange={(v) => update("agentTotalTimeout", v)}
      />

      <SliderField
        label="Knowledge Top-K"
        value={knowledgeTopK}
        min={1}
        max={20}
        step={1}
        onChange={(v) => update("knowledgeTopK", v)}
      />

      <div className="space-y-3">
        <ToggleField
          label="Intent Router"
          description="Route queries to appropriate tools before LLM"
          value={intentRouterEnabled}
          onChange={(v) => update("intentRouterEnabled", v)}
        />
        <ToggleField
          label="Web Search"
          description="Enable SearXNG web search as fallback tool"
          value={webSearchEnabled}
          onChange={(v) => update("webSearchEnabled", v)}
        />
      </div>
    </div>
  );
}
