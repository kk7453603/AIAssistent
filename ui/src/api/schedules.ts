import { apiFetch } from "./client";
import type { ScheduledTask } from "./types";

export async function listSchedules(): Promise<ScheduledTask[]> {
  return apiFetch<ScheduledTask[]>("/v1/schedules");
}

export interface CreateScheduleRequest {
  cron_expr: string;
  prompt: string;
  condition?: string;
  webhook_url?: string;
}

export async function createSchedule(data: CreateScheduleRequest): Promise<ScheduledTask> {
  return apiFetch<ScheduledTask>("/v1/schedules", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateSchedule(
  id: string,
  data: Partial<Pick<ScheduledTask, "cron_expr" | "prompt" | "condition" | "webhook_url" | "enabled">>,
): Promise<ScheduledTask> {
  return apiFetch<ScheduledTask>(`/v1/schedules/${id}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export async function deleteSchedule(id: string): Promise<void> {
  await apiFetch<void>(`/v1/schedules/${id}`, {
    method: "DELETE",
  });
}
