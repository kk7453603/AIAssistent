import { useCallback, useEffect, useState } from "react";
import { Clock, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import {
  listSchedules,
  createSchedule,
  updateSchedule,
  deleteSchedule,
} from "../../api/schedules";
import type { ScheduledTask } from "../../api/types";

function timeAgo(dateStr: string | null): string {
  if (!dateStr) return "—";
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function statusBadge(status: string) {
  if (status === "ok" || status === "success") {
    return <span className="rounded-full bg-green-100 dark:bg-green-900/30 px-2 py-0.5 text-xs text-green-700 dark:text-green-400">ok</span>;
  }
  if (status === "error" || status === "failed") {
    return <span className="rounded-full bg-red-100 dark:bg-red-900/30 px-2 py-0.5 text-xs text-red-700 dark:text-red-400">error</span>;
  }
  return <span className="text-xs text-gray-400">—</span>;
}

interface FormData {
  prompt: string;
  cron_expr: string;
  condition: string;
  webhook_url: string;
  enabled: boolean;
}

const emptyForm: FormData = {
  prompt: "",
  cron_expr: "",
  condition: "",
  webhook_url: "",
  enabled: true,
};

export function SchedulesWidget() {
  const [tasks, setTasks] = useState<ScheduledTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null); // null = create mode when form open
  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState<FormData>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);

  const fetchTasks = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await listSchedules();
      setTasks(data ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load schedules");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchTasks();
  }, [fetchTasks]);

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm);
    setFormOpen(true);
  };

  const openEdit = (task: ScheduledTask) => {
    setEditingId(task.id);
    setForm({
      prompt: task.prompt,
      cron_expr: task.cron_expr,
      condition: task.condition,
      webhook_url: task.webhook_url,
      enabled: task.enabled,
    });
    setFormOpen(true);
  };

  const closeForm = () => {
    setFormOpen(false);
    setEditingId(null);
    setForm(emptyForm);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      if (editingId) {
        await updateSchedule(editingId, {
          prompt: form.prompt,
          cron_expr: form.cron_expr,
          condition: form.condition || undefined,
          webhook_url: form.webhook_url || undefined,
          enabled: form.enabled,
        });
      } else {
        await createSchedule({
          prompt: form.prompt,
          cron_expr: form.cron_expr,
          condition: form.condition || undefined,
          webhook_url: form.webhook_url || undefined,
        });
      }
      closeForm();
      await fetchTasks();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setSaving(false);
    }
  };

  const handleToggleEnabled = async (task: ScheduledTask) => {
    try {
      await updateSchedule(task.id, { enabled: !task.enabled });
      setTasks((prev) =>
        prev.map((t) => (t.id === task.id ? { ...t, enabled: !t.enabled } : t)),
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "Toggle failed");
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteSchedule(id);
      setDeleteConfirmId(null);
      setTasks((prev) => prev.filter((t) => t.id !== id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Delete failed");
    }
  };

  if (loading && tasks.length === 0) {
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

      {/* Header actions */}
      <div className="mb-3 flex items-center gap-2">
        <button
          onClick={openCreate}
          className="flex items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700"
        >
          <Plus className="h-3.5 w-3.5" />
          Add Schedule
        </button>
        <button
          onClick={fetchTasks}
          className="rounded-md border border-gray-300 dark:border-gray-700 p-1.5 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800"
        >
          <RefreshCw className="h-3.5 w-3.5" />
        </button>
      </div>

      {/* Table */}
      {tasks.length === 0 && !formOpen ? (
        <div className="flex flex-col items-center gap-2 py-8 text-gray-400 dark:text-gray-500">
          <Clock className="h-10 w-10 opacity-30" />
          <p className="text-sm">No scheduled tasks yet.</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 dark:bg-gray-800/50">
              <tr>
                <th className="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">Prompt</th>
                <th className="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">Cron</th>
                <th className="px-3 py-2 text-center font-medium text-gray-600 dark:text-gray-400">Enabled</th>
                <th className="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">Last Run</th>
                <th className="px-3 py-2 text-center font-medium text-gray-600 dark:text-gray-400">Status</th>
                <th className="px-3 py-2 text-right font-medium text-gray-600 dark:text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {tasks.map((task) => (
                <tr key={task.id} className="hover:bg-gray-50 dark:hover:bg-gray-800/30">
                  <td className="px-3 py-2 text-gray-900 dark:text-gray-200 max-w-[200px] truncate">
                    {task.prompt}
                  </td>
                  <td className="px-3 py-2 font-mono text-xs text-gray-600 dark:text-gray-400">
                    {task.cron_expr}
                  </td>
                  <td className="px-3 py-2 text-center">
                    <button
                      onClick={() => handleToggleEnabled(task)}
                      className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                        task.enabled ? "bg-blue-600" : "bg-gray-300 dark:bg-gray-600"
                      }`}
                    >
                      <span
                        className={`inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform ${
                          task.enabled ? "translate-x-4.5" : "translate-x-0.5"
                        }`}
                      />
                    </button>
                  </td>
                  <td className="px-3 py-2 text-xs text-gray-500 dark:text-gray-400">
                    {timeAgo(task.last_run_at)}
                  </td>
                  <td className="px-3 py-2 text-center">
                    {statusBadge(task.last_status)}
                  </td>
                  <td className="px-3 py-2 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <button
                        onClick={() => openEdit(task)}
                        className="rounded p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-600 dark:hover:text-gray-200"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </button>
                      {deleteConfirmId === task.id ? (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => handleDelete(task.id)}
                            className="rounded bg-red-600 px-2 py-0.5 text-xs text-white hover:bg-red-700"
                          >
                            Confirm
                          </button>
                          <button
                            onClick={() => setDeleteConfirmId(null)}
                            className="rounded px-2 py-0.5 text-xs text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
                          >
                            Cancel
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => setDeleteConfirmId(task.id)}
                          className="rounded p-1 text-gray-400 hover:bg-red-50 dark:hover:bg-red-900/20 hover:text-red-600 dark:hover:text-red-400"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Inline form */}
      {formOpen && (
        <div className="mt-3 rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50 p-4 space-y-3">
          <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300">
            {editingId ? "Edit Schedule" : "New Schedule"}
          </h4>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div className="sm:col-span-2">
              <label className="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Prompt *</label>
              <input
                type="text"
                value={form.prompt}
                onChange={(e) => setForm((f) => ({ ...f, prompt: e.target.value }))}
                placeholder="Check for new articles about AI..."
                className="w-full rounded-md border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-1.5 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Cron Expression *</label>
              <input
                type="text"
                value={form.cron_expr}
                onChange={(e) => setForm((f) => ({ ...f, cron_expr: e.target.value }))}
                placeholder="0 9 * * *"
                className="w-full rounded-md border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-1.5 text-sm font-mono text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Condition (optional)</label>
              <input
                type="text"
                value={form.condition}
                onChange={(e) => setForm((f) => ({ ...f, condition: e.target.value }))}
                placeholder="only if new results found"
                className="w-full rounded-md border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-1.5 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
              />
            </div>
            <div className="sm:col-span-2">
              <label className="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Webhook URL (optional)</label>
              <input
                type="text"
                value={form.webhook_url}
                onChange={(e) => setForm((f) => ({ ...f, webhook_url: e.target.value }))}
                placeholder="https://hooks.slack.com/..."
                className="w-full rounded-md border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-1.5 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
              />
            </div>
          </div>
          {editingId && (
            <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => setForm((f) => ({ ...f, enabled: e.target.checked }))}
                className="rounded border-gray-400 text-blue-500"
              />
              Enabled
            </label>
          )}
          <div className="flex items-center gap-2">
            <button
              onClick={handleSave}
              disabled={saving || !form.prompt || !form.cron_expr}
              className="rounded-md bg-blue-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {saving ? "Saving..." : "Save"}
            </button>
            <button
              onClick={closeForm}
              className="rounded-md px-4 py-1.5 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
