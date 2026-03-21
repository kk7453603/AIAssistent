import { useState } from "react";
import { Check, ChevronDown, ChevronRight, Loader2, X } from "lucide-react";
import { useDashboardStore, type ToolActivity } from "../../stores/dashboardStore";

function ActivityEntry({ activity }: { activity: ToolActivity }) {
  const [expanded, setExpanded] = useState(false);

  const statusColor =
    activity.status === "ok"
      ? "text-green-400"
      : activity.status === "error"
        ? "text-red-400"
        : "text-blue-400";

  const StatusIcon =
    activity.status === "ok"
      ? Check
      : activity.status === "error"
        ? X
        : Loader2;

  const time = new Date(activity.timestamp).toLocaleTimeString();

  return (
    <div className="rounded border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-3 px-3 py-2 text-sm"
      >
        {expanded ? (
          <ChevronDown className="h-3.5 w-3.5 shrink-0 text-gray-400 dark:text-gray-500" />
        ) : (
          <ChevronRight className="h-3.5 w-3.5 shrink-0 text-gray-400 dark:text-gray-500" />
        )}
        <StatusIcon
          className={`h-4 w-4 shrink-0 ${statusColor} ${
            activity.status === "running" ? "animate-spin" : ""
          }`}
        />
        <span className="font-mono text-gray-700 dark:text-gray-300">{activity.tool}</span>
        {activity.duration != null && (
          <span className="text-xs text-gray-500">
            {(activity.duration / 1000).toFixed(1)}s
          </span>
        )}
        <span className="ml-auto text-xs text-gray-400 dark:text-gray-600">{time}</span>
      </button>
      {expanded && (
        <div className="border-t border-gray-200 dark:border-gray-800 px-3 py-2">
          {activity.input && (
            <div className="mb-1">
              <p className="text-xs font-medium text-gray-500">Input</p>
              <pre className="mt-0.5 whitespace-pre-wrap text-xs text-gray-500 dark:text-gray-400">
                {activity.input}
              </pre>
            </div>
          )}
          {activity.output && (
            <div>
              <p className="text-xs font-medium text-gray-500">Output</p>
              <pre className="mt-0.5 whitespace-pre-wrap text-xs text-gray-500 dark:text-gray-400">
                {activity.output}
              </pre>
            </div>
          )}
          {!activity.input && !activity.output && (
            <p className="text-xs text-gray-400 dark:text-gray-600">No details available</p>
          )}
        </div>
      )}
    </div>
  );
}

export function ActivityFeed() {
  const { activities } = useDashboardStore();

  if (activities.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-gray-500">
        <p>No tool activity yet. Start a conversation to see agent tools in action.</p>
      </div>
    );
  }

  return (
    <div className="space-y-1">
      {activities.map((a) => (
        <ActivityEntry key={a.id} activity={a} />
      ))}
    </div>
  );
}
