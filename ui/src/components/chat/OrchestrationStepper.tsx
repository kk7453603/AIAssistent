import { useState } from "react";
import { Check, ChevronDown, ChevronRight, Loader2, X } from "lucide-react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { OrchestrationStepEvent } from "../../api/types";

const AGENT_COLORS: Record<string, string> = {
  researcher: "#3b82f6",
  coder: "#10b981",
  writer: "#8b5cf6",
  critic: "#f97316",
};

function agentColor(name: string): string {
  return AGENT_COLORS[name] ?? "#6b7280";
}

function StatusIcon({ status }: { status: string }) {
  if (status === "started") {
    return <Loader2 className="h-4 w-4 animate-spin text-blue-400" />;
  }
  if (status === "completed") {
    return <Check className="h-4 w-4 text-green-400" />;
  }
  return <X className="h-4 w-4 text-red-400" />;
}

function formatDuration(ms: number): string {
  if (ms <= 0) return "";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

interface StepRowProps {
  step: OrchestrationStepEvent;
  isLast: boolean;
  isLastCompleted: boolean;
}

function StepRow({ step, isLast, isLastCompleted }: StepRowProps) {
  const [expanded, setExpanded] = useState(isLastCompleted);
  const hasResult = step.result && step.status !== "started";

  return (
    <div className="relative flex gap-3">
      {/* Timeline line */}
      <div className="flex flex-col items-center">
        <div
          className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-2"
          style={{ borderColor: agentColor(step.agent_name) }}
        >
          <StatusIcon status={step.status} />
        </div>
        {!isLast && (
          <div className="w-0.5 flex-1 bg-gray-300 dark:bg-gray-700" />
        )}
      </div>

      {/* Content */}
      <div className="flex-1 pb-4">
        <div className="flex items-center gap-2">
          <span
            className="rounded-full px-2 py-0.5 text-xs font-medium text-white"
            style={{ backgroundColor: agentColor(step.agent_name) }}
          >
            {step.agent_name}
          </span>
          <span className="text-sm text-gray-700 dark:text-gray-300 truncate">
            {step.task}
          </span>
          {step.duration_ms > 0 && (
            <span className="ml-auto shrink-0 text-xs text-gray-400">
              {formatDuration(step.duration_ms)}
            </span>
          )}
          {step.status === "started" && (
            <span className="ml-auto shrink-0 text-xs text-blue-400 animate-pulse">
              running...
            </span>
          )}
        </div>

        {/* Collapsible result */}
        {hasResult && (
          <div className="mt-1">
            <button
              onClick={() => setExpanded(!expanded)}
              className="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
            >
              {expanded ? (
                <ChevronDown className="h-3 w-3" />
              ) : (
                <ChevronRight className="h-3 w-3" />
              )}
              {expanded ? "Hide result" : "Show result"}
            </button>
            {expanded && (
              <div className="mt-1 rounded border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50 p-2 prose dark:prose-invert prose-xs max-w-none">
                <Markdown remarkPlugins={[remarkGfm]}>
                  {step.result}
                </Markdown>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

interface Props {
  steps: OrchestrationStepEvent[];
}

export function OrchestrationStepper({ steps }: Props) {
  if (steps.length === 0) return null;

  const sorted = [...steps].sort((a, b) => a.step_index - b.step_index);
  const completed = sorted.filter((s) => s.status === "completed");
  const lastCompletedIndex =
    completed.length > 0
      ? completed[completed.length - 1].step_index
      : undefined;

  return (
    <div className="space-y-0">
      {sorted.map((step, i) => (
        <StepRow
          key={`${step.orchestration_id}-${step.step_index}`}
          step={step}
          isLast={i === sorted.length - 1}
          isLastCompleted={step.step_index === lastCompletedIndex}
        />
      ))}
    </div>
  );
}
