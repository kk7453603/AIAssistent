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
