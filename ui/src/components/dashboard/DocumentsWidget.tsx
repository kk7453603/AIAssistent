import { useCallback, useEffect, useState } from "react";
import { FileText, RefreshCw } from "lucide-react";
import { listDocuments } from "../../api/documents";
import type { DocumentInfo } from "../../api/types";

function statusColor(status: string): string {
  switch (status) {
    case "ready":
      return "text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/30";
    case "processing":
      return "text-blue-600 dark:text-blue-400 bg-blue-100 dark:bg-blue-900/30";
    case "failed":
      return "text-red-600 dark:text-red-400 bg-red-100 dark:bg-red-900/30";
    default:
      return "text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-800";
  }
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function DocumentsWidget() {
  const [docs, setDocs] = useState<DocumentInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchDocs = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await listDocuments(30);
      setDocs(data ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load documents");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchDocs();
  }, [fetchDocs]);

  if (loading && docs.length === 0) {
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
          onClick={fetchDocs}
          className="rounded-md border border-gray-300 dark:border-gray-700 p-1.5 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800"
        >
          <RefreshCw className="h-3.5 w-3.5" />
        </button>
      </div>

      {docs.length === 0 ? (
        <div className="flex flex-col items-center gap-2 py-8 text-gray-400 dark:text-gray-500">
          <FileText className="h-10 w-10 opacity-30" />
          <p className="text-sm">No documents ingested yet.</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 dark:bg-gray-800/50">
              <tr>
                <th className="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">
                  Title / Filename
                </th>
                <th className="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">
                  Source
                </th>
                <th className="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">
                  Category
                </th>
                <th className="px-3 py-2 text-center font-medium text-gray-600 dark:text-gray-400">
                  Status
                </th>
                <th className="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">
                  Added
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {docs.map((doc) => (
                <tr
                  key={doc.id}
                  className="hover:bg-gray-50 dark:hover:bg-gray-800/30"
                >
                  <td className="px-3 py-2 max-w-[250px]">
                    <div className="text-gray-900 dark:text-gray-200 truncate">
                      {doc.title || doc.filename}
                    </div>
                    {doc.title && doc.filename && (
                      <div className="text-xs text-gray-400 truncate">
                        {doc.filename}
                      </div>
                    )}
                  </td>
                  <td className="px-3 py-2 text-xs text-gray-600 dark:text-gray-400">
                    {doc.source_type || "—"}
                  </td>
                  <td className="px-3 py-2 text-xs text-gray-600 dark:text-gray-400">
                    {doc.category || "—"}
                  </td>
                  <td className="px-3 py-2 text-center">
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs ${statusColor(doc.status)}`}
                    >
                      {doc.status}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-xs text-gray-500 dark:text-gray-400">
                    {timeAgo(doc.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
