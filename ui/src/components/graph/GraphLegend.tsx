// Palette for known categories + dynamically generated colors for others
const PALETTE = [
  "#3b82f6", "#8b5cf6", "#06b6d4", "#10b981", "#f59e0b",
  "#ef4444", "#ec4899", "#14b8a6", "#f97316", "#6366f1",
  "#84cc16", "#0ea5e9", "#d946ef", "#a855f7", "#22d3ee",
];

const dynamicColorCache = new Map<string, string>();

function hashString(s: string): number {
  let hash = 0;
  for (let i = 0; i < s.length; i++) {
    hash = ((hash << 5) - hash + s.charCodeAt(i)) | 0;
  }
  return Math.abs(hash);
}

export function getCategoryColor(category: string): string {
  if (!category) return "#6b7280";
  const cached = dynamicColorCache.get(category);
  if (cached) return cached;
  const color = PALETTE[hashString(category) % PALETTE.length];
  dynamicColorCache.set(category, color);
  return color;
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

export function GraphLegend({ categories }: { categories?: string[] }) {
  const cats = (categories ?? []).filter(Boolean).slice(0, 10);

  return (
    <div className="space-y-4 text-xs text-gray-600 dark:text-gray-400">
      {cats.length > 0 && (
        <div>
          <h4 className="mb-1.5 font-semibold text-gray-700 dark:text-gray-300">
            Category (color)
          </h4>
          <ul className="space-y-1">
            {cats.map((cat) => (
              <li key={cat} className="flex items-center gap-2">
                <span
                  className="inline-block h-3 w-3 rounded-full"
                  style={{ backgroundColor: getCategoryColor(cat) }}
                />
                {cat}
              </li>
            ))}
          </ul>
        </div>
      )}

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
