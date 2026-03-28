import { apiFetch } from "./client";
import type { Graph, GraphFilter } from "./types";

export async function fetchGraph(filter?: GraphFilter): Promise<Graph> {
  const params = new URLSearchParams();
  if (filter?.source_types?.length) {
    params.set("source_types", filter.source_types.join(","));
  }
  if (filter?.categories?.length) {
    params.set("categories", filter.categories.join(","));
  }
  if (filter?.min_score !== undefined) {
    params.set("min_score", String(filter.min_score));
  }
  if (filter?.max_depth !== undefined) {
    params.set("max_depth", String(filter.max_depth));
  }
  const qs = params.toString();
  return apiFetch<Graph>(`/v1/graph${qs ? `?${qs}` : ""}`);
}
