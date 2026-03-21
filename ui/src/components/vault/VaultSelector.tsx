import { ChevronDown } from "lucide-react";
import { useVaultStore } from "../../stores/vaultStore";

export function VaultSelector() {
  const { vaults, selectedVault, selectVault } = useVaultStore();

  return (
    <div className="relative">
      <select
        value={selectedVault ?? ""}
        onChange={(e) => selectVault(e.target.value)}
        className="w-full appearance-none rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 pr-8 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
      >
        <option value="" disabled>
          Select vault...
        </option>
        {vaults.map((v) => (
          <option key={v.name} value={v.name}>
            {v.name}
          </option>
        ))}
      </select>
      <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400 dark:text-gray-500" />
    </div>
  );
}
