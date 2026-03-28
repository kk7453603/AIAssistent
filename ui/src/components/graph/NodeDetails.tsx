import { ExternalLink, FileText, X } from "lucide-react";
import { useGraphStore } from "../../stores/graphStore";
import { getCategoryColor } from "./GraphLegend";
import type { GraphNode, GraphRelation } from "../../api/types";

interface Props {
  node: GraphNode;
  edges: GraphRelation[];
  allNodes: GraphNode[];
  onNavigateToVault: (path: string) => void;
  onViewDocument: (docId: string) => void;
}

export function NodeDetails({ node, edges, allNodes, onNavigateToVault, onViewDocument }: Props) {
  const selectNode = useGraphStore((s) => s.selectNode);

  const nodeMap = new Map(allNodes.map((n) => [n.id, n]));

  const relatedEdges = edges.filter(
    (e) => e.source_id === node.id || e.target_id === node.id,
  );

  return (
    <div className="flex flex-col h-full border-l border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-gray-200 dark:border-gray-800 px-4 py-3">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate">
          {node.title || node.filename}
        </h3>
        <button
          onClick={() => selectNode(null)}
          className="rounded p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-600 dark:hover:text-gray-200"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {/* Metadata */}
        <div className="space-y-2 text-sm">
          <div className="flex items-center gap-2">
            <span className="text-gray-500 dark:text-gray-400 w-20 shrink-0">Category</span>
            <span className="flex items-center gap-1.5 text-gray-900 dark:text-gray-200">
              <span
                className="inline-block h-2.5 w-2.5 rounded-full"
                style={{ backgroundColor: getCategoryColor(node.category) }}
              />
              {node.category || "—"}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-gray-500 dark:text-gray-400 w-20 shrink-0">Source</span>
            <span className="text-gray-900 dark:text-gray-200">{node.source_type}</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-gray-500 dark:text-gray-400 w-20 shrink-0">Filename</span>
            <span className="text-gray-900 dark:text-gray-200 truncate">{node.filename}</span>
          </div>
          {node.path && (
            <div className="flex items-center gap-2">
              <span className="text-gray-500 dark:text-gray-400 w-20 shrink-0">Path</span>
              <span className="text-gray-900 dark:text-gray-200 truncate text-xs">{node.path}</span>
            </div>
          )}
        </div>

        {/* Relations */}
        {relatedEdges.length > 0 && (
          <div>
            <h4 className="mb-2 text-xs font-semibold text-gray-700 dark:text-gray-300">
              Relations ({relatedEdges.length})
            </h4>
            <ul className="space-y-1">
              {relatedEdges.map((e, i) => {
                const targetId = e.source_id === node.id ? e.target_id : e.source_id;
                const target = nodeMap.get(targetId);
                return (
                  <li key={i}>
                    <button
                      onClick={() => selectNode(targetId)}
                      className="flex w-full items-center gap-2 rounded px-2 py-1 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-800"
                    >
                      <span className="text-xs text-gray-400 dark:text-gray-500 w-20 shrink-0">
                        {e.type}
                      </span>
                      <span className="text-blue-600 dark:text-blue-400 truncate">
                        {target?.title || target?.filename || targetId}
                      </span>
                      {e.weight < 1 && (
                        <span className="ml-auto text-xs text-gray-400">
                          {e.weight.toFixed(2)}
                        </span>
                      )}
                    </button>
                  </li>
                );
              })}
            </ul>
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="border-t border-gray-200 dark:border-gray-800 p-4 space-y-2">
        <button
          onClick={() => onViewDocument(node.id)}
          className="flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          <FileText className="h-4 w-4" />
          View Document
        </button>
        {node.source_type === "obsidian" && node.path && (
          <button
            onClick={() => onNavigateToVault(node.path)}
            className="flex w-full items-center justify-center gap-2 rounded-md border border-gray-300 dark:border-gray-700 px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800"
          >
            <ExternalLink className="h-4 w-4" />
            Open in Vault Browser
          </button>
        )}
      </div>
    </div>
  );
}
