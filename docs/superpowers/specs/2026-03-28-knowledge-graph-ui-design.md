# Knowledge Graph 3D Visualization UI — Design Spec

## Goal

Add a full-screen 3D interactive knowledge graph visualization page to the Tauri desktop app, allowing users to explore document relationships, filter by category/source, search nodes, and navigate to documents.

## Tech Stack

- **react-force-graph-3d** (Three.js) — 3D force-directed graph rendering
- **three** — Three.js peer dependency for custom node objects
- **three-spritetext** — text labels on nodes
- React 19 + Zustand 5 + Tailwind 3 (existing stack)
- Backend: `GET /v1/graph` (already implemented)

## Architecture

Composable component approach matching existing project patterns (chat/, vault/, dashboard/ each split into small components).

### File Structure

**New files:**
```
ui/src/pages/GraphPage.tsx                — layout: sidebar + canvas + details
ui/src/components/graph/GraphCanvas.tsx    — react-force-graph-3d wrapper
ui/src/components/graph/GraphFilters.tsx   — filters + node search
ui/src/components/graph/NodeDetails.tsx    — selected node detail panel
ui/src/components/graph/GraphLegend.tsx    — color/shape/edge legend
ui/src/stores/graphStore.ts               — Zustand: fetch, filters, selection
ui/src/api/graph.ts                       — API client for GET /v1/graph
```

**Modified files:**
```
ui/src/App.tsx          — add "Graph" tab + lazy import GraphPage
ui/src/api/types.ts     — add GraphNode, GraphRelation, Graph types
ui/package.json         — add react-force-graph-3d, three, three-spritetext
```

## Data Types (types.ts)

```typescript
interface GraphNode {
  id: string;
  filename: string;
  source_type: string;  // upload | web | obsidian
  category: string;     // article | note | reference | ...
  title: string;
  path: string;
}

interface GraphRelation {
  source_id: string;
  target_id: string;
  type: string;    // wikilink | markdown_link | similarity
  weight: number;  // 0..1
}

interface Graph {
  nodes: GraphNode[];
  edges: GraphRelation[];
}
```

## Zustand Store (graphStore.ts)

```typescript
interface GraphState {
  graph: Graph | null;
  loading: boolean;
  error: string | null;

  // Filters
  sourceTypes: string[];        // active source_type filters
  categories: string[];         // active category filters
  minScore: number;             // similarity threshold (0.0-1.0)
  searchQuery: string;          // search node by name

  // Selection
  selectedNodeId: string | null;
  hoveredNodeId: string | null;

  // Actions
  fetchGraph: () => Promise<void>;
  toggleSourceType: (st: string) => void;
  toggleCategory: (cat: string) => void;
  setMinScore: (score: number) => void;
  setSearchQuery: (q: string) => void;
  selectNode: (id: string | null) => void;
  setHoveredNode: (id: string | null) => void;
}
```

Derived data computed in components (not stored):
- `filteredNodes` — nodes after sourceTypes/categories/searchQuery filters
- `filteredEdges` — edges where both endpoints in filteredNodes + weight >= minScore
- `uniqueCategories` / `uniqueSourceTypes` — for filter checkboxes

## API Client (api/graph.ts)

```typescript
export async function fetchGraph(filter?: GraphFilter): Promise<Graph> {
  const params = new URLSearchParams();
  if (filter?.source_types?.length) params.set("source_types", filter.source_types.join(","));
  if (filter?.categories?.length) params.set("categories", filter.categories.join(","));
  if (filter?.min_score) params.set("min_score", String(filter.min_score));
  return apiFetch<Graph>(`/v1/graph?${params}`);
}
```

Dual filtering: server-side (reduce payload) + client-side (instant UI filtering without re-fetch).

## Visual Encoding

### Node color by category

```typescript
const CATEGORY_COLORS: Record<string, string> = {
  article:    "#3b82f6",  // blue
  note:       "#8b5cf6",  // violet
  reference:  "#06b6d4",  // cyan
  tutorial:   "#10b981",  // emerald
  code:       "#f59e0b",  // amber
  other:      "#6b7280",  // gray — fallback
};
```

### Node shape/size by source_type

- `upload` — sphere (radius 6)
- `web` — octahedron (radius 5)
- `obsidian` — dodecahedron (radius 7)

Implementation: Three.js custom `nodeThreeObject` — each node gets a `Mesh` with corresponding `Geometry` + `MeshLambertMaterial` in category color. SpriteText label with title next to node.

### Edge color by type

- `wikilink` — solid blue line
- `markdown_link` — solid green line
- `similarity` — orange line, opacity proportional to weight

Edge width scales by weight (1-3px).

### Hover/Selection states

- **Hover** — node scales x1.5, label brightens, connected nodes highlight, others dim (opacity 0.15)
- **Selected** — yellow ring outline, camera smoothly focuses on node

## Layout

```
┌──────────────────────────────────────────────────────┐
│  Header (tabs: Chat  Vaults  Dashboard  Graph  Settings)  │
├──────────┬───────────────────────┬───────────────────┤
│ Filters  │                       │   NodeDetails     │
│          │    3D Graph Canvas    │   (slideout)      │
│ Search   │                       │                   │
│ Legend   │                       │                   │
├──────────┴───────────────────────┴───────────────────┤
```

- **Left sidebar** — `w-60 shrink-0`, collapsible to 0 via toggle button
- **Center canvas** — `flex-1 min-w-0`, adapts to available space via ResizeObserver
- **Right panel** — `w-80 shrink-0`, appears on node selection, hidden by default

```tsx
<div className="flex h-full">
  {showFilters && <GraphFilters className="w-60 shrink-0" />}
  <GraphCanvas className="flex-1 min-w-0" />
  {selectedNode && <NodeDetails className="w-80 shrink-0" />}
</div>
```

Canvas adapts to any window size — react-force-graph-3d takes `width`/`height` props from ResizeObserver on the container. Collapsing sidebar or closing details causes canvas to stretch automatically.

## Interactions

| Action | Result |
|--------|--------|
| Hover node | Tooltip (title), scale up, dim others |
| Click node | selectNode → open NodeDetails right panel, camera focus |
| Click empty space | deselectNode → close NodeDetails |
| Search in sidebar | Filter + auto-focus camera on first result |
| Checkbox source_type | Instant client-side filtering |
| Checkbox category | Instant client-side filtering |
| Slider minScore | Hide similarity edges below threshold |
| "Open in Vault" button in NodeDetails | setPage("vault") + pass document path |

### NodeDetails panel contains:
- Title, filename, path
- Source type, category
- List of relations (type + target title), clickable → focus on target node
- "Open in Vault Browser" button

## Empty State

When graph is empty or GRAPH_ENABLED=false:
- Graph icon + text "Knowledge Graph пуст"
- Instructions: "Установите GRAPH_ENABLED=true и загрузите документы"
- "Go to Settings" button

## Integration with Existing UI

### App.tsx changes
- Add `"graph"` to `Page` type
- New navItem: `{ key: "graph", label: "Graph", icon: Network }` (lucide-react)
- Render: `{page === "graph" && <GraphPage />}`
- Position: after Dashboard, before Settings

### Graph → Vault Browser navigation
- "Open in Vault" button in NodeDetails calls `setPage("vault")`
- Context passing: add `pendingVaultPath` state in App (same pattern as `pendingRef`)

### Chat → Graph navigation
- When agent uses `knowledge_search` — response can include "Show in Graph" link
- Clicking → `setPage("graph")` + focus on the node

### Lazy loading
- `GraphPage` loaded via `React.lazy()` + `Suspense`
- Three.js (~300KB) only loaded when user opens Graph tab
