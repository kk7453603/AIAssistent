const DEFAULT_API_URL = "http://localhost:8080";

let apiUrl = DEFAULT_API_URL;

export function setApiUrl(url: string) {
  apiUrl = url;
}

export function getApiUrl(): string {
  return apiUrl;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const url = `${apiUrl}${path}`;
  const resp = await fetch(url, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });

  if (!resp.ok) {
    const body = await resp.text().catch(() => "");
    throw new ApiError(resp.status, body || resp.statusText);
  }

  return resp.json() as Promise<T>;
}
