import { useApiHealth } from "../hooks/useApiHealth";

export function ConnectionStatus() {
  const { connected, checking } = useApiHealth();

  return (
    <div className="flex items-center gap-2 text-sm">
      <span
        className={`inline-block h-2.5 w-2.5 rounded-full ${
          checking
            ? "bg-yellow-400 animate-pulse"
            : connected
              ? "bg-green-400"
              : "bg-red-500"
        }`}
      />
      <span className="text-gray-500 dark:text-gray-400">
        {checking ? "Connecting..." : connected ? "API Connected" : "API Offline"}
      </span>
    </div>
  );
}
