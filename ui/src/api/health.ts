import { apiFetch } from "./client";
import type { HealthResponse } from "./types";

export async function checkHealth(): Promise<boolean> {
  try {
    const resp = await apiFetch<HealthResponse>("/healthz");
    return resp.status === "ok";
  } catch {
    return false;
  }
}
