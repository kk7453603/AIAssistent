import { useCallback, useEffect, useState } from "react";
import { Check, ThumbsDown, ThumbsUp, X, Zap, RefreshCw } from "lucide-react";
import {
  fetchEventsSummary,
  fetchFeedbackSummary,
  fetchImprovements,
  patchImprovement,
} from "../../api/selfImprove";
import type {
  EventSummary,
  FeedbackSummary,
  AgentImprovement,
} from "../../api/types";

const EVENT_COLORS: Record<string, string> = {
  empty_retrieval: "#f59e0b",
  tool_error: "#ef4444",
  fallback: "#8b5cf6",
  low_confidence: "#f97316",
};

function EventsCard({ data }: { data: EventSummary | null }) {
  if (!data || Object.keys(data).length === 0) {
    return (
      <div className="text-sm text-gray-400 dark:text-gray-500 text-center py-4">
        No events recorded
      </div>
    );
  }
  const maxCount = Math.max(...Object.values(data), 1);
  return (
    <div className="space-y-2">
      {Object.entries(data)
        .sort(([, a], [, b]) => b - a)
        .map(([type, count]) => (
          <div key={type} className="flex items-center gap-2 text-sm">
            <span className="w-32 shrink-0 truncate text-gray-600 dark:text-gray-400">
              {type}
            </span>
            <div className="flex-1 h-4 rounded bg-gray-100 dark:bg-gray-800 overflow-hidden">
              <div
                className="h-full rounded"
                style={{
                  width: `${(count / maxCount) * 100}%`,
                  backgroundColor: EVENT_COLORS[type] ?? "#6b7280",
                }}
              />
            </div>
            <span className="w-8 text-right text-xs text-gray-500">{count}</span>
          </div>
        ))}
    </div>
  );
}

function FeedbackCard({ data }: { data: FeedbackSummary | null }) {
  if (!data) {
    return (
      <div className="text-sm text-gray-400 dark:text-gray-500 text-center py-4">
        No feedback yet
      </div>
    );
  }
  const up = data.counts["positive"] ?? data.counts["thumbs_up"] ?? 0;
  const down = data.counts["negative"] ?? data.counts["thumbs_down"] ?? 0;
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-1.5 text-green-600 dark:text-green-400">
          <ThumbsUp className="h-4 w-4" />
          <span className="text-lg font-semibold">{up}</span>
        </div>
        <div className="flex items-center gap-1.5 text-red-500 dark:text-red-400">
          <ThumbsDown className="h-4 w-4" />
          <span className="text-lg font-semibold">{down}</span>
        </div>
      </div>
      {data.recent.length > 0 && (
        <div>
          <h4 className="mb-1 text-xs font-semibold text-gray-600 dark:text-gray-400">
            Recent
          </h4>
          <ul className="space-y-1">
            {data.recent.slice(0, 5).map((fb) => (
              <li
                key={fb.id}
                className="flex items-start gap-2 text-xs text-gray-600 dark:text-gray-400"
              >
                {fb.rating === "positive" || fb.rating === "thumbs_up" ? (
                  <ThumbsUp className="h-3 w-3 shrink-0 mt-0.5 text-green-500" />
                ) : (
                  <ThumbsDown className="h-3 w-3 shrink-0 mt-0.5 text-red-500" />
                )}
                <span className="truncate">{fb.comment || "No comment"}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function ImprovementsCard({
  items,
  onAction,
}: {
  items: AgentImprovement[];
  onAction: (id: string, status: "approved" | "dismissed") => void;
}) {
  if (items.length === 0) {
    return (
      <div className="text-sm text-gray-400 dark:text-gray-500 text-center py-4">
        No pending improvements
      </div>
    );
  }
  return (
    <ul className="space-y-2">
      {items.map((imp) => (
        <li
          key={imp.id}
          className="rounded-md border border-gray-200 dark:border-gray-700 p-2"
        >
          <div className="flex items-start justify-between gap-2">
            <div className="flex-1 min-w-0">
              <span className="rounded-full bg-blue-100 dark:bg-blue-900/30 px-2 py-0.5 text-xs text-blue-700 dark:text-blue-400">
                {imp.category}
              </span>
              <p className="mt-1 text-sm text-gray-700 dark:text-gray-300">
                {imp.description}
              </p>
            </div>
            <div className="flex shrink-0 items-center gap-1">
              <button
                onClick={() => onAction(imp.id, "approved")}
                className="rounded p-1 text-green-600 hover:bg-green-50 dark:hover:bg-green-900/20"
                title="Approve"
              >
                <Check className="h-4 w-4" />
              </button>
              <button
                onClick={() => onAction(imp.id, "dismissed")}
                className="rounded p-1 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20"
                title="Dismiss"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}

export function SelfImproveWidget() {
  const [events, setEvents] = useState<EventSummary | null>(null);
  const [feedback, setFeedback] = useState<FeedbackSummary | null>(null);
  const [improvements, setImprovements] = useState<AgentImprovement[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [ev, fb, imp] = await Promise.allSettled([
        fetchEventsSummary(),
        fetchFeedbackSummary(),
        fetchImprovements(),
      ]);
      if (ev.status === "fulfilled") setEvents(ev.value);
      if (fb.status === "fulfilled") setFeedback(fb.value);
      if (imp.status === "fulfilled") setImprovements(imp.value ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAll();
  }, [fetchAll]);

  const handleImprovementAction = async (id: string, status: "approved" | "dismissed") => {
    try {
      await patchImprovement(id, status);
      setImprovements((prev) => prev.filter((i) => i.id !== id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Action failed");
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
      </div>
    );
  }

  return (
    <div>
      {error && (
        <div className="mb-3 rounded-md bg-red-50 dark:bg-red-900/20 px-3 py-2 text-sm text-red-700 dark:text-red-400">
          {error}
        </div>
      )}

      <div className="mb-3 flex justify-end">
        <button
          onClick={fetchAll}
          className="rounded-md border border-gray-300 dark:border-gray-700 p-1.5 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800"
        >
          <RefreshCw className="h-3.5 w-3.5" />
        </button>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div className="rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-300">
            <Zap className="h-4 w-4 text-amber-500" />
            Agent Events (7d)
          </h4>
          <EventsCard data={events} />
        </div>

        <div className="rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-300">
            <ThumbsUp className="h-4 w-4 text-green-500" />
            User Feedback (7d)
          </h4>
          <FeedbackCard data={feedback} />
        </div>

        <div className="rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-300">
            <Check className="h-4 w-4 text-blue-500" />
            Pending Improvements
          </h4>
          <ImprovementsCard items={improvements} onAction={handleImprovementAction} />
        </div>
      </div>
    </div>
  );
}
