import { apiFetch } from "./client";
import type { EventSummary, FeedbackSummary, AgentImprovement } from "./types";

export async function fetchEventsSummary(): Promise<EventSummary> {
  return apiFetch<EventSummary>("/v1/events/summary");
}

export async function fetchFeedbackSummary(): Promise<FeedbackSummary> {
  return apiFetch<FeedbackSummary>("/v1/feedback/summary");
}

export async function fetchImprovements(): Promise<AgentImprovement[]> {
  return apiFetch<AgentImprovement[]>("/v1/improvements");
}

export async function patchImprovement(id: string, status: "approved" | "dismissed"): Promise<void> {
  await apiFetch<void>(`/v1/improvements/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ status }),
  });
}
