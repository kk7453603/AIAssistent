import { apiFetch } from "./client";
import type { HTTPToolDef } from "./types";

export async function listHTTPTools(): Promise<HTTPToolDef[]> {
  return apiFetch<HTTPToolDef[]>("/v1/tools");
}
