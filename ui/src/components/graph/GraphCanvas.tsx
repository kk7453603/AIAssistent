import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ForceGraph3D from "react-force-graph-3d";
import type { ForceGraphMethods, NodeObject, LinkObject } from "react-force-graph-3d";
import SpriteText from "three-spritetext";
import {
  SphereGeometry,
  OctahedronGeometry,
  DodecahedronGeometry,
  MeshBasicMaterial,
  Mesh,
  Group,
} from "three";
import { useGraphStore, useFilteredGraph } from "../../stores/graphStore";
import { getCategoryColor } from "./GraphLegend";
import type { GraphNode } from "../../api/types";

const EDGE_COLORS: Record<string, string> = {
  wikilink: "#3b82f6",
  markdown_link: "#10b981",
  similarity: "#f97316",
};

// Shared geometries — created once, reused for all nodes.
const GEOM_SPHERE = new SphereGeometry(6, 12, 12);
const GEOM_OCTAHEDRON = new OctahedronGeometry(5);
const GEOM_DODECAHEDRON = new DodecahedronGeometry(7);

function getGeometry(sourceType: string) {
  switch (sourceType) {
    case "web":
      return GEOM_OCTAHEDRON;
    case "obsidian":
      return GEOM_DODECAHEDRON;
    default:
      return GEOM_SPHERE;
  }
}

interface GraphLink {
  source: string;
  target: string;
  type: string;
  weight: number;
}

type FGNode = NodeObject<GraphNode>;
type FGLink = LinkObject<GraphNode, GraphLink>;

export function GraphCanvas() {
  const containerRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<ForceGraphMethods<FGNode, FGLink> | undefined>(undefined);
  const [dimensions, setDimensions] = useState({ width: 800, height: 600 });

  const { nodes, edges } = useFilteredGraph();
  const hoveredNodeId = useGraphStore((s) => s.hoveredNodeId);
  const selectedNodeId = useGraphStore((s) => s.selectedNodeId);
  const selectNode = useGraphStore((s) => s.selectNode);
  const setHoveredNode = useGraphStore((s) => s.setHoveredNode);

  // Track hovered node in a ref to avoid re-creating nodeThreeObject on hover.
  const hoveredRef = useRef<string | null>(null);
  hoveredRef.current = hoveredNodeId;

  // Cache Three.js objects per node ID to avoid re-creation.
  const nodeObjectCache = useRef(new Map<string, Group>());

  // Convert to force-graph format
  const graphData = useMemo(() => {
    const links: GraphLink[] = edges.map((e) => ({
      source: e.source_id,
      target: e.target_id,
      type: e.type,
      weight: e.weight,
    }));
    return { nodes: [...nodes] as FGNode[], links: links as FGLink[] };
  }, [nodes, edges]);

  // Clear cache when graph data changes
  useEffect(() => {
    nodeObjectCache.current.clear();
  }, [nodes]);

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

  // Build node Three.js objects — cached, NOT dependent on hover state.
  const nodeThreeObject = useCallback(
    (node: FGNode) => {
      const gn = node as GraphNode;
      const cached = nodeObjectCache.current.get(gn.id);
      if (cached) return cached;

      const color = getCategoryColor(gn.category);
      const geometry = getGeometry(gn.source_type);
      const material = new MeshBasicMaterial({ color, transparent: true, opacity: 0.9 });
      const mesh = new Mesh(geometry, material);

      const showLabel = nodes.length <= 80;
      const group = new Group();
      group.add(mesh);

      if (showLabel) {
        const label = new SpriteText(gn.title || gn.filename, 3);
        label.color = "#d1d5db";
        label.backgroundColor = "transparent";
        label.position.set(0, 10, 0);
        group.add(label);
      }

      nodeObjectCache.current.set(gn.id, group);
      return group;
    },
    // Only recreate when nodes array reference changes (filter/fetch).
    // Hover does NOT trigger recreation.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [nodes],
  );

  // Use nodeColor for hover dimming — cheap, no object recreation.
  const nodeColor = useCallback(
    (node: FGNode) => {
      const gn = node as GraphNode;
      const hovered = hoveredRef.current;
      if (!hovered) return getCategoryColor(gn.category);
      if (gn.id === hovered) return getCategoryColor(gn.category);
      // Check if connected
      for (const e of edges) {
        if (
          (e.source_id === hovered && e.target_id === gn.id) ||
          (e.target_id === hovered && e.source_id === gn.id)
        ) {
          return getCategoryColor(gn.category);
        }
      }
      return "#33333344"; // dimmed
    },
    [edges],
  );

  const handleNodeClick = useCallback(
    (node: FGNode) => {
      const gn = node as GraphNode;
      selectNode(gn.id === selectedNodeId ? null : gn.id);
    },
    [selectNode, selectedNodeId],
  );

  const handleNodeHover = useCallback(
    (node: FGNode | null) => {
      const gn = node as GraphNode | null;
      setHoveredNode(gn?.id ?? null);
      // Update container cursor
      if (containerRef.current) {
        containerRef.current.style.cursor = gn ? "pointer" : "default";
      }
    },
    [setHoveredNode],
  );

  const handleBackgroundClick = useCallback(() => {
    selectNode(null);
  }, [selectNode]);

  const linkColor = useCallback(
    (link: FGLink) => {
      const gl = link as unknown as GraphLink;
      return EDGE_COLORS[gl.type] ?? "#6b7280";
    },
    [],
  );

  const linkWidth = useCallback(
    (link: FGLink) => {
      const gl = link as unknown as GraphLink;
      return 1 + gl.weight * 2;
    },
    [],
  );

  // Tooltip on hover (lightweight, no object recreation)
  const nodeLabel = useCallback(
    (node: FGNode) => {
      const gn = node as GraphNode;
      return `<div style="background:#1f2937;color:#f3f4f6;padding:4px 8px;border-radius:6px;font-size:12px;max-width:200px">
        <b>${gn.title || gn.filename}</b>
        ${gn.category ? `<br/><span style="color:#9ca3af">${gn.category}</span>` : ""}
      </div>`;
    },
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
        nodeLabel={nodeLabel}
        nodeColor={nodeColor}
        nodeThreeObject={nodeThreeObject}
        nodeThreeObjectExtend={false}
        onNodeClick={handleNodeClick}
        onNodeHover={handleNodeHover}
        onBackgroundClick={handleBackgroundClick}
        linkSource="source"
        linkTarget="target"
        linkColor={linkColor}
        linkWidth={linkWidth}
        linkOpacity={0.6}
        linkDirectionalParticles={0}
        backgroundColor="#00000000"
        showNavInfo={false}
        cooldownTicks={150}
        warmupTicks={50}
        d3AlphaDecay={0.05}
        d3VelocityDecay={0.3}
      />
    </div>
  );
}
