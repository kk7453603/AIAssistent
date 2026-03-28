import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ForceGraph3D from "react-force-graph-3d";
import type { ForceGraphMethods, NodeObject, LinkObject } from "react-force-graph-3d";
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

type FGNode = NodeObject<GraphNode>;
type FGLink = LinkObject<GraphNode, GraphLink>;

export function GraphCanvas() {
  const containerRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<ForceGraphMethods<FGNode, FGLink> | undefined>(undefined);
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
    return { nodes: [...nodes] as FGNode[], links: links as FGLink[] };
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
    (node: FGNode) => {
      const gn = node as GraphNode;
      const color = getCategoryColor(gn.category);
      const geometry = makeNodeGeometry(gn.source_type);
      const material = new MeshLambertMaterial({ color });

      // Dim if hovering another node
      if (connectedIds && !connectedIds.has(gn.id)) {
        material.opacity = 0.15;
        material.transparent = true;
      }

      // Scale up on hover
      const scale = gn.id === hoveredNodeId ? 1.5 : 1;

      const mesh = new Mesh(geometry, material);
      mesh.scale.set(scale, scale, scale);

      const label = new SpriteText(gn.title || gn.filename, 3);
      label.color = connectedIds && !connectedIds.has(gn.id) ? "#6b728066" : "#d1d5db";
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
        linkOpacity={0.6}
        linkDirectionalParticles={0}
        backgroundColor="#00000000"
        showNavInfo={false}
      />
    </div>
  );
}
