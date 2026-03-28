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
