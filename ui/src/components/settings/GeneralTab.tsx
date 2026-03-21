import { useSettingsStore } from "../../stores/settingsStore";

export function GeneralTab() {
  const { apiUrl, vaultsPath, theme, language, update } = useSettingsStore();

  return (
    <div className="space-y-6">
      <div>
        <label className="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-300">
          PAA API URL
        </label>
        <input
          type="text"
          value={apiUrl}
          onChange={(e) => update("apiUrl", e.target.value)}
          className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
        />
      </div>

      <div>
        <label className="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-300">
          Obsidian Vaults Path
        </label>
        <input
          type="text"
          value={vaultsPath}
          onChange={(e) => update("vaultsPath", e.target.value)}
          className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
        />
      </div>

      <div>
        <label className="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-300">
          Theme
        </label>
        <select
          value={theme}
          onChange={(e) =>
            update("theme", e.target.value as "light" | "dark" | "system")
          }
          className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
        >
          <option value="dark">Dark</option>
          <option value="light">Light</option>
          <option value="system">System</option>
        </select>
      </div>

      <div>
        <label className="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-300">
          Language
        </label>
        <select
          value={language}
          onChange={(e) =>
            update("language", e.target.value as "ru" | "en")
          }
          className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-gray-200 outline-none focus:border-blue-500"
        >
          <option value="ru">Русский</option>
          <option value="en">English</option>
        </select>
      </div>
    </div>
  );
}
