import { useEffect } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useDashboardStore } from "../../stores/dashboardStore";
import { useSettingsStore } from "../../stores/settingsStore";

const COLORS = ["#3b82f6", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6", "#ec4899"];

export function ToolStats() {
  const { toolStats, fetchToolStats } = useDashboardStore();
  const apiUrl = useSettingsStore((s) => s.apiUrl);

  useEffect(() => {
    fetchToolStats(apiUrl);
    const id = setInterval(() => fetchToolStats(apiUrl), 30_000);
    return () => clearInterval(id);
  }, [apiUrl, fetchToolStats]);

  if (toolStats.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-gray-500">
        <p>No tool metrics available. Ensure PAA API is running with /metrics endpoint.</p>
      </div>
    );
  }

  const chartData = toolStats.map((s) => ({
    name: s.tool,
    calls: s.totalCalls,
    avgMs: Math.round(s.avgDuration * 1000),
    errorPct: Math.round(s.errorRate * 100),
  }));

  return (
    <div className="space-y-6">
      {/* Calls per tool */}
      <div>
        <h3 className="mb-3 text-sm font-medium text-gray-600 dark:text-gray-300">
          Total Calls by Tool
        </h3>
        <ResponsiveContainer width="100%" height={200}>
          <BarChart data={chartData}>
            <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
            <XAxis dataKey="name" tick={{ fill: "#9ca3af", fontSize: 12 }} />
            <YAxis tick={{ fill: "#9ca3af", fontSize: 12 }} />
            <Tooltip
              contentStyle={{
                backgroundColor: "#1f2937",
                border: "1px solid #374151",
                borderRadius: 8,
                color: "#e5e7eb",
              }}
            />
            <Bar dataKey="calls" radius={[4, 4, 0, 0]}>
              {chartData.map((_, i) => (
                <Cell key={i} fill={COLORS[i % COLORS.length]} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>

      {/* Stats table */}
      <div className="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-800">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 dark:bg-gray-900 text-gray-500 dark:text-gray-400">
            <tr>
              <th className="px-4 py-2 text-left font-medium">Tool</th>
              <th className="px-4 py-2 text-right font-medium">Calls</th>
              <th className="px-4 py-2 text-right font-medium">Avg (ms)</th>
              <th className="px-4 py-2 text-right font-medium">Error %</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200 dark:divide-gray-800">
            {chartData.map((row) => (
              <tr key={row.name} className="text-gray-700 dark:text-gray-300">
                <td className="px-4 py-2 font-mono">{row.name}</td>
                <td className="px-4 py-2 text-right">{row.calls}</td>
                <td className="px-4 py-2 text-right">{row.avgMs}</td>
                <td className="px-4 py-2 text-right">
                  <span
                    className={
                      row.errorPct > 10 ? "text-red-400" : "text-gray-500 dark:text-gray-400"
                    }
                  >
                    {row.errorPct}%
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
