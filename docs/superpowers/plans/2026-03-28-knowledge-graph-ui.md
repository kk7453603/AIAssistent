# Knowledge Graph 3D Visualization UI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full-screen 3D interactive knowledge graph page to the Tauri desktop app using react-force-graph-3d.

**Architecture:** New "Graph" tab with composable components: GraphCanvas (3D renderer), GraphFilters (sidebar), NodeDetails (selection panel), GraphLegend. Zustand store for state. API client fetches from existing `GET /v1/graph`. Lazy-loaded to avoid loading Three.js until needed.

**Tech Stack:** react-force-graph-3d, three, three-spritetext, React 19, Zustand 5, Tailwind 3

**Spec:** `docs/superpowers/specs/2026-03-28-knowledge-graph-ui-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `ui/package.json` | Modify | Add dependencies |
| `ui/src/api/types.ts` | Modify | Add Graph types |
| `ui/src/api/graph.ts` | Create | API client for GET /v1/graph |
| `ui/src/stores/graphStore.ts` | Create | Zustand store: fetch, filters, selection |
| `ui/src/components/graph/GraphCanvas.tsx` | Create | react-force-graph-3d wrapper |
| `ui/src/components/graph/GraphFilters.tsx` | Create | Sidebar filters + search |
| `ui/src/components/graph/NodeDetails.tsx` | Create | Selected node detail panel |
| `ui/src/components/graph/GraphLegend.tsx` | Create | Color/shape/edge legend |
| `ui/src/pages/GraphPage.tsx` | Create | Page layout: sidebar + canvas + details |
| `ui/src/App.tsx` | Modify | Add Graph tab + lazy import |

---

### Task 1: Install dependencies and add types

**Files:**
- Modify: `ui/package.json`
- Modify: `ui/src/api/types.ts`

- [ ] **Step 1: Install npm packages**

Run from `ui/` directory:

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npm install react-force-graph-3d three three-spritetext && npm install -D @types/three
```

Expected: packages added to `package.json` dependencies/devDependencies, `node_modules` updated.

- [ ] **Step 2: Add Graph types to types.ts**

Append to end of `ui/src/api/types.ts`:

```typescript
// --- Knowledge Graph ---

export interface GraphNode {
  id: string;
  filename: string;
  source_type: string;
  category: string;
  title: string;
  path: string;
}

export interface GraphRelation {
  source_id: string;
  target_id: string;
  type: string;
  weight: number;
}

export interface Graph {
  nodes: GraphNode[];
  edges: GraphRelation[];
}

export interface GraphFilter {
  source_types?: string[];
  categories?: string[];
  min_score?: number;
  max_depth?: number;
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/package.json ui/package-lock.json ui/src/api/types.ts && git commit -m "feat(ui): add graph visualization dependencies and types"
```

---

### Task 2: API client

**Files:**
- Create: `ui/src/api/graph.ts`

- [ ] **Step 1: Create graph API client**

Create `ui/src/api/graph.ts`:

```typescript
import { apiFetch } from "./client";
import type { Graph, GraphFilter } from "./types";

export async function fetchGraph(filter?: GraphFilter): Promise<Graph> {
  const params = new URLSearchParams();
  if (filter?.source_types?.length) {
    params.set("source_types", filter.source_types.join(","));
  }
  if (filter?.categories?.length) {
    params.set("categories", filter.categories.join(","));
  }
  if (filter?.min_score !== undefined) {
    params.set("min_score", String(filter.min_score));
  }
  if (filter?.max_depth !== undefined) {
    params.set("max_depth", String(filter.max_depth));
  }
  const qs = params.toString();
  return apiFetch<Graph>(`/v1/graph${qs ? `?${qs}` : ""}`);
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/api/graph.ts && git commit -m "feat(ui): add graph API client"
```

---

### Task 3: Zustand store

**Files:**
- Create: `ui/src/stores/graphStore.ts`

- [ ] **Step 1: Create graph store**

Create `ui/src/stores/graphStore.ts`:

```typescript
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
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/stores/graphStore.ts && git commit -m "feat(ui): add Zustand graph store with filters and selectors"
```

---

### Task 4: GraphLegend component

**Files:**
- Create: `ui/src/components/graph/GraphLegend.tsx`

- [ ] **Step 1: Create GraphLegend**

Create `ui/src/components/graph/GraphLegend.tsx`:

```tsx
export const CATEGORY_COLORS: Record<string, string> = {
  article: "#3b82f6",
  note: "#8b5cf6",
  reference: "#06b6d4",
  tutorial: "#10b981",
  code: "#f59e0b",
  other: "#6b7280",
};

export function getCategoryColor(category: string): string {
  return CATEGORY_COLORS[category] ?? CATEGORY_COLORS.other;
}

const SOURCE_SHAPES: { type: string; label: string }[] = [
  { type: "upload", label: "Upload (sphere)" },
  { type: "web", label: "Web (octahedron)" },
  { type: "obsidian", label: "Obsidian (dodecahedron)" },
];

const EDGE_TYPES: { type: string; color: string; label: string }[] = [
  { type: "wikilink", color: "#3b82f6", label: "Wikilink" },
  { type: "markdown_link", color: "#10b981", label: "Markdown link" },
  { type: "similarity", color: "#f97316", label: "Similarity" },
];

export function GraphLegend() {
  return (
    <div className="space-y-4 text-xs text-gray-600 dark:text-gray-400">
      <div>
        <h4 className="mb-1.5 font-semibold text-gray-700 dark:text-gray-300">
          Category (color)
        </h4>
        <ul className="space-y-1">
          {Object.entries(CATEGORY_COLORS).map(([cat, color]) => (
            <li key={cat} className="flex items-center gap-2">
              <span
                className="inline-block h-3 w-3 rounded-full"
                style={{ backgroundColor: color }}
              />
              {cat}
            </li>
          ))}
        </ul>
      </div>

      <div>
        <h4 className="mb-1.5 font-semibold text-gray-700 dark:text-gray-300">
          Source (shape)
        </h4>
        <ul className="space-y-1">
          {SOURCE_SHAPES.map(({ type, label }) => (
            <li key={type}>{label}</li>
          ))}
        </ul>
      </div>

      <div>
        <h4 className="mb-1.5 font-semibold text-gray-700 dark:text-gray-300">
          Edges
        </h4>
        <ul className="space-y-1">
          {EDGE_TYPES.map(({ type, color, label }) => (
            <li key={type} className="flex items-center gap-2">
              <span
                className="inline-block h-0.5 w-4"
                style={{ backgroundColor: color }}
              />
              {label}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/components/graph/GraphLegend.tsx && git commit -m "feat(ui): add GraphLegend component with color/shape/edge encoding"
```

---

### Task 5: GraphFilters component

**Files:**
- Create: `ui/src/components/graph/GraphFilters.tsx`

- [ ] **Step 1: Create GraphFilters**

Create `ui/src/components/graph/GraphFilters.tsx`:

```tsx
import { Search } from "lucide-react";
import {
  useGraphStore,
  selectUniqueSourceTypes,
  selectUniqueCategories,
} from "../../stores/graphStore";
import { getCategoryColor } from "./GraphLegend";

export function GraphFilters() {
  const searchQuery = useGraphStore((s) => s.searchQuery);
  const setSearchQuery = useGraphStore((s) => s.setSearchQuery);
  const activeSourceTypes = useGraphStore((s) => s.sourceTypes);
  const toggleSourceType = useGraphStore((s) => s.toggleSourceType);
  const activeCategories = useGraphStore((s) => s.categories);
  const toggleCategory = useGraphStore((s) => s.toggleCategory);
  const minScore = useGraphStore((s) => s.minScore);
  const setMinScore = useGraphStore((s) => s.setMinScore);

  const allSourceTypes = useGraphStore(selectUniqueSourceTypes);
  const allCategories = useGraphStore(selectUniqueCategories);

  return (
    <div className="flex flex-col gap-5 overflow-y-auto p-4">
      {/* Search */}
      <div>
        <label className="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">
          Search
        </label>
        <div className="relative">
          <Search className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Find node..."
            className="w-full rounded-md border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 py-1.5 pl-8 pr-3 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
          />
        </div>
      </div>

      {/* Source Type */}
      {allSourceTypes.length > 0 && (
        <div>
          <label className="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">
            Source Type
          </label>
          <div className="space-y-1">
            {allSourceTypes.map((st) => (
              <label key={st} className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
                <input
                  type="checkbox"
                  checked={activeSourceTypes.length === 0 || activeSourceTypes.includes(st)}
                  onChange={() => toggleSourceType(st)}
                  className="rounded border-gray-400 text-blue-500"
                />
                {st}
              </label>
            ))}
          </div>
        </div>
      )}

      {/* Category */}
      {allCategories.length > 0 && (
        <div>
          <label className="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">
            Category
          </label>
          <div className="space-y-1">
            {allCategories.map((cat) => (
              <label key={cat} className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
                <input
                  type="checkbox"
                  checked={activeCategories.length === 0 || activeCategories.includes(cat)}
                  onChange={() => toggleCategory(cat)}
                  className="rounded border-gray-400 text-blue-500"
                />
                <span
                  className="inline-block h-2.5 w-2.5 rounded-full"
                  style={{ backgroundColor: getCategoryColor(cat) }}
                />
                {cat}
              </label>
            ))}
          </div>
        </div>
      )}

      {/* Similarity Threshold */}
      <div>
        <label className="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">
          Min Similarity: {minScore.toFixed(2)}
        </label>
        <input
          type="range"
          min={0}
          max={1}
          step={0.05}
          value={minScore}
          onChange={(e) => setMinScore(parseFloat(e.target.value))}
          className="w-full accent-blue-500"
        />
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/components/graph/GraphFilters.tsx && git commit -m "feat(ui): add GraphFilters component with search, checkboxes, and slider"
```

---

### Task 6: NodeDetails component

**Files:**
- Create: `ui/src/components/graph/NodeDetails.tsx`

- [ ] **Step 1: Create NodeDetails**

Create `ui/src/components/graph/NodeDetails.tsx`:

```tsx
import { X, ExternalLink } from "lucide-react";
import { useGraphStore } from "../../stores/graphStore";
import { getCategoryColor } from "./GraphLegend";
import type { GraphNode, GraphRelation } from "../../api/types";

interface Props {
  node: GraphNode;
  edges: GraphRelation[];
  allNodes: GraphNode[];
  onNavigateToVault: (path: string) => void;
}

export function NodeDetails({ node, edges, allNodes, onNavigateToVault }: Props) {
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
      {node.path && (
        <div className="border-t border-gray-200 dark:border-gray-800 p-4">
          <button
            onClick={() => onNavigateToVault(node.path)}
            className="flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-700"
          >
            <ExternalLink className="h-4 w-4" />
            Open in Vault Browser
          </button>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/components/graph/NodeDetails.tsx && git commit -m "feat(ui): add NodeDetails component with metadata, relations, and vault nav"
```

---

### Task 7: GraphCanvas component

**Files:**
- Create: `ui/src/components/graph/GraphCanvas.tsx`

This is the core component — wraps react-force-graph-3d with custom Three.js node rendering.

- [ ] **Step 1: Create GraphCanvas**

Create `ui/src/components/graph/GraphCanvas.tsx`:

```tsx
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ForceGraph3D, { type ForceGraph3DInstance } from "react-force-graph-3d";
import SpriteText from "three-spritetext";
import {
  SphereGeometry,
  OctahedronGeometry,
  DodecahedronGeometry,
  MeshLambertMaterial,
  Mesh,
  Group,
} from "three";
import { useGraphStore, selectFilteredGraph } from "../../stores/graphStore";
import { getCategoryColor } from "./GraphLegend";
import type { GraphNode } from "../../api/types";

const EDGE_COLORS: Record<string, string> = {
  wikilink: "#3b82f6",
  markdown_link: "#10b981",
  similarity: "#f97316",
};

function makeNodeGeometry(sourceType: string) {
  switch (sourceType) {
    case "web":
      return new OctahedronGeometry(5);
    case "obsidian":
      return new DodecahedronGeometry(7);
    default:
      return new SphereGeometry(6, 16, 16);
  }
}

interface GraphLink {
  source: string;
  target: string;
  type: string;
  weight: number;
}

export function GraphCanvas() {
  const containerRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<ForceGraph3DInstance | undefined>(undefined);
  const [dimensions, setDimensions] = useState({ width: 800, height: 600 });

  const { nodes, edges } = useGraphStore(selectFilteredGraph);
  const hoveredNodeId = useGraphStore((s) => s.hoveredNodeId);
  const selectedNodeId = useGraphStore((s) => s.selectedNodeId);
  const selectNode = useGraphStore((s) => s.selectNode);
  const setHoveredNode = useGraphStore((s) => s.setHoveredNode);

  // Convert to force-graph format
  const graphData = useMemo(() => {
    const links: GraphLink[] = edges.map((e) => ({
      source: e.source_id,
      target: e.target_id,
      type: e.type,
      weight: e.weight,
    }));
    return { nodes: [...nodes], links };
  }, [nodes, edges]);

  // ResizeObserver for responsive canvas
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      const { width, height } = entries[0].contentRect;
      setDimensions({ width: Math.floor(width), height: Math.floor(height) });
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Focus camera on selected node
  useEffect(() => {
    if (!selectedNodeId || !fgRef.current) return;
    const node = nodes.find((n) => n.id === selectedNodeId);
    if (!node) return;
    const fg = fgRef.current;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const n = node as any;
    if (n.x !== undefined && n.y !== undefined && n.z !== undefined) {
      fg.cameraPosition(
        { x: n.x + 80, y: n.y + 80, z: n.z + 80 },
        { x: n.x, y: n.y, z: n.z },
        1000,
      );
    }
  }, [selectedNodeId, nodes]);

  // Set of connected node IDs for hover dimming
  const connectedIds = useMemo(() => {
    if (!hoveredNodeId) return null;
    const ids = new Set<string>([hoveredNodeId]);
    for (const e of edges) {
      if (e.source_id === hoveredNodeId) ids.add(e.target_id);
      if (e.target_id === hoveredNodeId) ids.add(e.source_id);
    }
    return ids;
  }, [hoveredNodeId, edges]);

  const nodeThreeObject = useCallback(
    (node: GraphNode) => {
      const color = getCategoryColor(node.category);
      const geometry = makeNodeGeometry(node.source_type);
      const material = new MeshLambertMaterial({ color });

      // Dim if hovering another node
      if (connectedIds && !connectedIds.has(node.id)) {
        material.opacity = 0.15;
        material.transparent = true;
      }

      // Scale up on hover
      const scale = node.id === hoveredNodeId ? 1.5 : 1;

      const mesh = new Mesh(geometry, material);
      mesh.scale.set(scale, scale, scale);

      const label = new SpriteText(node.title || node.filename, 3);
      label.color = connectedIds && !connectedIds.has(node.id) ? "#6b728066" : "#d1d5db";
      label.backgroundColor = "transparent";
      label.position.set(0, 10, 0);

      const group = new Group();
      group.add(mesh);
      group.add(label);

      return group;
    },
    [hoveredNodeId, connectedIds],
  );

  const handleNodeClick = useCallback(
    (node: GraphNode) => {
      selectNode(node.id === selectedNodeId ? null : node.id);
    },
    [selectNode, selectedNodeId],
  );

  const handleNodeHover = useCallback(
    (node: GraphNode | null) => {
      setHoveredNode(node?.id ?? null);
    },
    [setHoveredNode],
  );

  const handleBackgroundClick = useCallback(() => {
    selectNode(null);
  }, [selectNode]);

  const linkColor = useCallback(
    (link: GraphLink) => EDGE_COLORS[link.type] ?? "#6b7280",
    [],
  );

  const linkWidth = useCallback(
    (link: GraphLink) => 1 + link.weight * 2,
    [],
  );

  const linkOpacity = useCallback(
    (link: GraphLink) => (link.type === "similarity" ? link.weight : 0.6),
    [],
  );

  return (
    <div ref={containerRef} className="h-full w-full">
      <ForceGraph3D
        ref={fgRef}
        width={dimensions.width}
        height={dimensions.height}
        graphData={graphData}
        nodeId="id"
        nodeLabel=""
        nodeThreeObject={nodeThreeObject}
        nodeThreeObjectExtend={false}
        onNodeClick={handleNodeClick}
        onNodeHover={handleNodeHover}
        onBackgroundClick={handleBackgroundClick}
        linkSource="source"
        linkTarget="target"
        linkColor={linkColor}
        linkWidth={linkWidth}
        linkOpacity={linkOpacity}
        linkDirectionalParticles={0}
        backgroundColor="#00000000"
        showNavInfo={false}
      />
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

Expected: no errors (there may be type adjustments needed for react-force-graph-3d generics — if so, add `// @ts-expect-error` only where the library types are incomplete, which is common for this package).

- [ ] **Step 3: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/components/graph/GraphCanvas.tsx && git commit -m "feat(ui): add GraphCanvas with 3D force-directed rendering and visual encoding"
```

---

### Task 8: GraphPage layout

**Files:**
- Create: `ui/src/pages/GraphPage.tsx`

- [ ] **Step 1: Create GraphPage**

Create `ui/src/pages/GraphPage.tsx`:

```tsx
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
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/pages/GraphPage.tsx && git commit -m "feat(ui): add GraphPage with sidebar, canvas, details panel, and empty state"
```

---

### Task 9: Wire GraphPage into App

**Files:**
- Modify: `ui/src/App.tsx`

- [ ] **Step 1: Add Graph tab to App.tsx**

In `ui/src/App.tsx`, make the following changes:

1. Add import at the top (after existing imports):

```typescript
import { lazy, Suspense } from "react";
import { Activity, FolderOpen, MessageSquare, Network, Settings } from "lucide-react";
```

Remove the old `import { useCallback, useEffect, useState } from "react";` and `import { Activity, FolderOpen, MessageSquare, Settings } from "lucide-react";` lines.

2. Add lazy import after other imports:

```typescript
const GraphPage = lazy(() =>
  import("./pages/GraphPage").then((m) => ({ default: m.GraphPage })),
);
```

3. Change the `Page` type:

```typescript
type Page = "chat" | "vault" | "dashboard" | "graph" | "settings";
```

4. Add `pendingVaultPath` state alongside `pendingRef`:

```typescript
const [pendingVaultPath, setPendingVaultPath] = useState<string | null>(null);
```

5. Add graph navItem (after dashboard, before settings):

```typescript
{ key: "graph", label: "Graph", icon: Network },
```

6. Add GraphPage render in main (after dashboard, before settings):

```tsx
{page === "graph" && (
  <Suspense fallback={<div className="flex h-full items-center justify-center"><div className="h-8 w-8 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" /></div>}>
    <GraphPage
      onNavigateToVault={(path) => {
        setPendingVaultPath(path);
        setPage("vault");
      }}
    />
  </Suspense>
)}
```

The full updated `App.tsx` should look like:

```tsx
import { lazy, Suspense, useCallback, useEffect, useState } from "react";
import { Activity, FolderOpen, MessageSquare, Network, Settings } from "lucide-react";
import { ConnectionStatus } from "./components/ConnectionStatus";
import { ChatPage } from "./pages/ChatPage";
import { VaultBrowserPage } from "./pages/VaultBrowserPage";
import { DashboardPage } from "./pages/DashboardPage";
import { SettingsPage } from "./pages/SettingsPage";
import { useSettingsStore } from "./stores/settingsStore";
import { isTauri } from "./utils/isTauri";

const GraphPage = lazy(() =>
  import("./pages/GraphPage").then((m) => ({ default: m.GraphPage })),
);

type Page = "chat" | "vault" | "dashboard" | "graph" | "settings";

export default function App() {
  const [page, setPage] = useState<Page>("chat");
  const [pendingRef, setPendingRef] = useState<string | null>(null);
  const [pendingVaultPath, setPendingVaultPath] = useState<string | null>(null);
  const loadSettings = useSettingsStore((s) => s.load);
  const theme = useSettingsStore((s) => s.theme);

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  // Apply theme class to document root
  useEffect(() => {
    const root = document.documentElement;

    const applyTheme = (isDark: boolean) => {
      if (isDark) {
        root.classList.add("dark");
      } else {
        root.classList.remove("dark");
      }
    };

    if (theme === "dark") {
      applyTheme(true);
    } else if (theme === "light") {
      applyTheme(false);
    } else {
      // system
      const mq = window.matchMedia("(prefers-color-scheme: dark)");
      applyTheme(mq.matches);
      const handler = (e: MediaQueryListEvent) => applyTheme(e.matches);
      mq.addEventListener("change", handler);
      return () => mq.removeEventListener("change", handler);
    }
  }, [theme]);

  // Toggle quick-ask window from main window context (Tauri only)
  useEffect(() => {
    if (!isTauri) return;

    let cancelled = false;
    let cleanup: (() => void) | undefined;

    (async () => {
      const { listen } = await import("@tauri-apps/api/event");
      const { WebviewWindow } = await import("@tauri-apps/api/webviewWindow");

      if (cancelled) return;

      const unlisten = await listen("toggle-quick-ask", async () => {
        const quickAsk = await WebviewWindow.getByLabel("quick-ask");
        if (quickAsk) {
          const visible = await quickAsk.isVisible();
          if (visible) {
            await quickAsk.hide();
          } else {
            await quickAsk.show();
            await quickAsk.setFocus();
          }
        }
      });
      cleanup = unlisten;
    })();

    return () => {
      cancelled = true;
      cleanup?.();
    };
  }, []);

  const handleReferenceInChat = useCallback((content: string) => {
    setPendingRef(content);
    setPage("chat");
  }, []);

  const navItems: { key: Page; label: string; icon: typeof MessageSquare }[] = [
    { key: "chat", label: "Chat", icon: MessageSquare },
    { key: "vault", label: "Vaults", icon: FolderOpen },
    { key: "dashboard", label: "Dashboard", icon: Activity },
    { key: "graph", label: "Graph", icon: Network },
    { key: "settings", label: "Settings", icon: Settings },
  ];

  return (
    <div className="flex h-screen flex-col bg-white dark:bg-gray-950">
      <header className="flex items-center justify-between border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-4 py-3">
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
            <span className="hidden md:inline">Personal AI Assistant</span>
            <span className="md:hidden">PAA</span>
          </h1>
          <nav className="flex gap-1 overflow-x-auto">
            {navItems.map(({ key, label, icon: Icon }) => (
              <button
                key={key}
                onClick={() => setPage(key)}
                className={`flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm md:px-3 ${
                  page === key
                    ? "bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100"
                    : "text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800/50 hover:text-gray-700 dark:hover:text-gray-200"
                }`}
              >
                <Icon className="h-4 w-4 shrink-0" />
                <span className="hidden md:inline">{label}</span>
              </button>
            ))}
          </nav>
        </div>
        <ConnectionStatus />
      </header>

      <main className="flex-1 overflow-hidden">
        {page === "chat" && (
          <ChatPage
            pendingReference={pendingRef}
            onReferenceClear={() => setPendingRef(null)}
          />
        )}
        {page === "vault" && (
          <VaultBrowserPage onReferenceInChat={handleReferenceInChat} />
        )}
        {page === "dashboard" && <DashboardPage />}
        {page === "graph" && (
          <Suspense
            fallback={
              <div className="flex h-full items-center justify-center">
                <div className="h-8 w-8 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
              </div>
            }
          >
            <GraphPage
              onNavigateToVault={(path) => {
                setPendingVaultPath(path);
                setPage("vault");
              }}
            />
          </Suspense>
        )}
        {page === "settings" && <SettingsPage />}
      </main>
    </div>
  );
}
```

Note: `pendingVaultPath` is added for future use — when VaultBrowserPage supports opening a file by path, wire it through. For now it's set but not consumed.

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Verify dev server runs**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npx vite build
```

Expected: successful build with no errors. Three.js chunk should appear in output.

- [ ] **Step 4: Commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add ui/src/App.tsx && git commit -m "feat(ui): wire GraphPage into App with lazy loading and Graph tab"
```

---

### Task 10: Manual testing and polish

- [ ] **Step 1: Start dev server and verify Graph tab renders**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent/ui && npm run dev
```

Open browser at `http://localhost:1420` (or Vite's port). Click "Graph" tab.

**If backend is running with GRAPH_ENABLED=true and has data:** expect 3D graph with nodes, edges, colors, shapes.

**If backend is down or GRAPH_ENABLED=false:** expect empty state with "Knowledge Graph пуст" message.

- [ ] **Step 2: Test interactions**

1. Hover a node → verify tooltip, scale up, dimming of unconnected nodes
2. Click a node → verify NodeDetails panel opens on right, camera focuses
3. Click another node in NodeDetails relations → verify camera moves to that node
4. Click background → verify NodeDetails closes
5. Type in Search → verify nodes filter in real-time
6. Toggle source_type checkboxes → verify filtering
7. Toggle category checkboxes → verify filtering
8. Move similarity slider → verify edges appear/disappear
9. Collapse sidebar → verify canvas expands, toggle button appears
10. Click "Open in Vault Browser" → verify navigation to vault page

- [ ] **Step 3: Fix any TypeScript or runtime issues found during testing**

Address any console errors, rendering glitches, or type mismatches.

- [ ] **Step 4: Final commit**

```bash
cd /home/kirillkom/GolangProjects/PersonalAIAssistent && git add -A && git commit -m "fix(ui): polish graph visualization after manual testing"
```

Only commit this step if there were actual fixes. Skip if everything worked on first try.
