import { useEffect, useMemo } from "react";
import { Network, PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { useState } from "react";
import { useGraphStore, selectFilteredGraph } from "../stores/graphStore";
import { GraphCanvas } from "../components/graph/GraphCanvas";
import { GraphFilters } from "../components/graph/GraphFilters";
import { GraphLegend } from "../components/graph/GraphLegend";
import { NodeDetails } from "../components/graph/NodeDetails";

interface Props {
  onNavigateToVault?: (path: string) => void;
}

export function GraphPage({ onNavigateToVault }: Props) {
  const fetch = useGraphStore((s) => s.fetch);
  const loading = useGraphStore((s) => s.loading);
  const error = useGraphStore((s) => s.error);
  const graph = useGraphStore((s) => s.graph);
  const selectedNodeId = useGraphStore((s) => s.selectedNodeId);
  const { nodes, edges } = useGraphStore(selectFilteredGraph);

  const [showFilters, setShowFilters] = useState(true);

  useEffect(() => {
    fetch();
  }, [fetch]);

  const selectedNode = useMemo(
    () => (selectedNodeId ? nodes.find((n) => n.id === selectedNodeId) ?? null : null),
    [selectedNodeId, nodes],
  );

  // Empty state
  if (!loading && !error && graph && graph.nodes.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4 text-gray-500 dark:text-gray-400">
        <Network className="h-16 w-16 opacity-30" />
        <h2 className="text-lg font-semibold text-gray-700 dark:text-gray-300">
          Knowledge Graph пуст
        </h2>
        <p className="max-w-sm text-center text-sm">
          Установите <code className="rounded bg-gray-100 dark:bg-gray-800 px-1 py-0.5">GRAPH_ENABLED=true</code> в
          настройках и загрузите документы для построения графа связей.
        </p>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4 text-gray-500 dark:text-gray-400">
        <Network className="h-16 w-16 opacity-30" />
        <h2 className="text-lg font-semibold text-red-500">Error loading graph</h2>
        <p className="text-sm">{error}</p>
        <button
          onClick={fetch}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm text-white hover:bg-blue-700"
        >
          Retry
        </button>
      </div>
    );
  }

  // Loading state
  if (loading && !graph) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="flex h-full">
      {/* Left sidebar */}
      {showFilters && (
        <div className="w-60 shrink-0 border-r border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 flex flex-col">
          <div className="flex items-center justify-between border-b border-gray-200 dark:border-gray-800 px-4 py-3">
            <span className="text-sm font-semibold text-gray-700 dark:text-gray-300">
              Filters
            </span>
            <button
              onClick={() => setShowFilters(false)}
              className="rounded p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800"
            >
              <PanelLeftClose className="h-4 w-4" />
            </button>
          </div>
          <div className="flex-1 overflow-y-auto">
            <GraphFilters />
          </div>
          <div className="border-t border-gray-200 dark:border-gray-800 p-4">
            <GraphLegend />
          </div>
        </div>
      )}

      {/* Canvas */}
      <div className="relative flex-1 min-w-0 bg-gray-950">
        {!showFilters && (
          <button
            onClick={() => setShowFilters(true)}
            className="absolute left-3 top-3 z-10 rounded-md bg-gray-800/70 p-2 text-gray-300 hover:bg-gray-700/80 hover:text-white"
          >
            <PanelLeftOpen className="h-4 w-4" />
          </button>
        )}
        {/* Node count badge */}
        <div className="absolute right-3 top-3 z-10 rounded-md bg-gray-800/70 px-2 py-1 text-xs text-gray-300">
          {nodes.length} nodes &middot; {edges.length} edges
        </div>
        <GraphCanvas />
      </div>

      {/* Right panel */}
      {selectedNode && (
        <div className="w-80 shrink-0">
          <NodeDetails
            node={selectedNode}
            edges={edges}
            allNodes={nodes}
            onNavigateToVault={onNavigateToVault ?? (() => {})}
          />
        </div>
      )}
    </div>
  );
}
