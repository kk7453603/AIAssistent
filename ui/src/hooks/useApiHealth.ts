import { useEffect, useState } from "react";
import { checkHealth } from "../api/health";

const POLL_INTERVAL = 10_000;

export function useApiHealth() {
  const [connected, setConnected] = useState(false);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    let cancelled = false;

    async function poll() {
      const ok = await checkHealth();
      if (!cancelled) {
        setConnected(ok);
        setChecking(false);
      }
    }

    poll();
    const id = setInterval(poll, POLL_INTERVAL);

    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  return { connected, checking };
}
