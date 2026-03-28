import { create } from "zustand";
import { fetchGraph } from "../api/graph";
import type { Graph, GraphNode, GraphRelation } from "../api/types";

interface GraphState {
  graph: Graph | null;
  loading: boolean;
  error: string | null;

  // Filters
  sourceTypes: string[];
  categories: string[];
  minScore: number;
  searchQuery: string;

  // Selection
  selectedNodeId: string | null;
  hoveredNodeId: string | null;

  // Actions
  fetch: () => Promise<void>;
  toggleSourceType: (st: string) => void;
  toggleCategory: (cat: string) => void;
  setMinScore: (score: number) => void;
  setSearchQuery: (q: string) => void;
  selectNode: (id: string | null) => void;
  setHoveredNode: (id: string | null) => void;
}

export const useGraphStore = create<GraphState>()((set, get) => ({
  graph: null,
  loading: false,
  error: null,

  sourceTypes: [],
  categories: [],
  minScore: 0.5,
  searchQuery: "",

  selectedNodeId: null,
  hoveredNodeId: null,

  fetch: async () => {
    set({ loading: true, error: null });
    try {
      const graph = await fetchGraph();
      set({ graph, loading: false });
    } catch (e) {
      set({ error: e instanceof Error ? e.message : "Failed to fetch graph", loading: false });
    }
  },

  toggleSourceType: (st) => {
    const current = get().sourceTypes;
    set({
      sourceTypes: current.includes(st)
        ? current.filter((s) => s !== st)
        : [...current, st],
    });
  },

  toggleCategory: (cat) => {
    const current = get().categories;
    set({
      categories: current.includes(cat)
        ? current.filter((c) => c !== cat)
        : [...current, cat],
    });
  },

  setMinScore: (score) => set({ minScore: score }),
  setSearchQuery: (q) => set({ searchQuery: q }),
  selectNode: (id) => set({ selectedNodeId: id }),
  setHoveredNode: (id) => set({ hoveredNodeId: id }),
}));

// --- Derived selectors ---

export function selectFilteredGraph(state: GraphState): {
  nodes: GraphNode[];
  edges: GraphRelation[];
} {
  const { graph, sourceTypes, categories, searchQuery, minScore } = state;
  if (!graph) return { nodes: [], edges: [] };

  let nodes = graph.nodes;

  if (sourceTypes.length > 0) {
    nodes = nodes.filter((n) => sourceTypes.includes(n.source_type));
  }
  if (categories.length > 0) {
    nodes = nodes.filter((n) => categories.includes(n.category));
  }
  if (searchQuery.trim()) {
    const q = searchQuery.toLowerCase();
    nodes = nodes.filter(
      (n) =>
        n.title.toLowerCase().includes(q) ||
        n.filename.toLowerCase().includes(q),
    );
  }

  const nodeIds = new Set(nodes.map((n) => n.id));
  const edges = graph.edges.filter(
    (e) =>
      nodeIds.has(e.source_id) &&
      nodeIds.has(e.target_id) &&
      e.weight >= minScore,
  );

  return { nodes, edges };
}

export function selectUniqueSourceTypes(state: GraphState): string[] {
  if (!state.graph) return [];
  return [...new Set(state.graph.nodes.map((n) => n.source_type))].sort();
}

export function selectUniqueCategories(state: GraphState): string[] {
  if (!state.graph) return [];
  return [...new Set(state.graph.nodes.map((n) => n.category))].sort();
}
