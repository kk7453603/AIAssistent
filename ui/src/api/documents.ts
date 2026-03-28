import { apiFetch } from "./client";
import type { DocumentInfo } from "./types";

export async function listDocuments(limit = 50): Promise<DocumentInfo[]> {
  return apiFetch<DocumentInfo[]>(`/v1/documents?limit=${limit}`);
}
