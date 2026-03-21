import { useCallback, useEffect, useRef, useState } from "react";
import { Loader2, Search, X } from "lucide-react";
import { useVaultStore } from "../../stores/vaultStore";

interface Props {
  onResultClick: (filePath: string) => void;
}

export function VaultSearch({ onResultClick }: Props) {
  const { searchResults, isSearching, searchVault, clearSearch } =
    useVaultStore();
  const [query, setQuery] = useState("");
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const handleChange = useCallback(
    (value: string) => {
      setQuery(value);
      clearTimeout(debounceRef.current);
      if (!value.trim()) {
        clearSearch();
        return;
      }
      debounceRef.current = setTimeout(() => {
        searchVault(value);
      }, 300);
    },
    [searchVault, clearSearch],
  );

  useEffect(() => {
    return () => clearTimeout(debounceRef.current);
  }, []);

  return (
    <div className="flex flex-col">
      <div className="relative">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400 dark:text-gray-500" />
        <input
          type="text"
          value={query}
          onChange={(e) => handleChange(e.target.value)}
          placeholder="Search in vault..."
          className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 py-2 pl-9 pr-8 text-sm text-gray-900 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500 outline-none focus:border-blue-500"
        />
        {query && (
          <button
            onClick={() => {
              setQuery("");
              clearSearch();
            }}
            className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <X className="h-4 w-4" />
          </button>
        )}
      </div>

      {isSearching && (
        <div className="flex items-center gap-2 px-2 py-3 text-sm text-gray-500 dark:text-gray-400">
          <Loader2 className="h-4 w-4 animate-spin" />
          Searching...
        </div>
      )}

      {searchResults.length > 0 && (
        <div className="mt-2 max-h-64 space-y-1 overflow-y-auto">
          {searchResults.map((r, i) => {
            const fileName = r.file_path.split("/").pop() ?? r.file_path;
            return (
              <button
                key={i}
                onClick={() => onResultClick(r.file_path)}
                className="w-full rounded px-2 py-1.5 text-left hover:bg-gray-100 dark:hover:bg-gray-800"
              >
                <p className="truncate text-xs text-gray-500 dark:text-gray-400">{fileName}:{r.line_number}</p>
                <p className="truncate text-sm text-gray-700 dark:text-gray-300">
                  {r.line_content}
                </p>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
