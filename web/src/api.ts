import type { CreateLinkInput, Link, Stats } from "./types";
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
    },
  });
  if (res.status === 204) {
    return undefined as T;
  }
  const body: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    const msg =
      body &&
      typeof body === "object" &&
      "error" in body &&
      typeof body.error === "string"
        ? body.error
        : res.statusText || `HTTP ${res.status}`;
    throw new ApiError(res.status, msg);
  }
  return body as T;
}
export const api = {
  listLinks: () => request<Link[]>("/api/links"),
  createLink: (input: CreateLinkInput) =>
    request<Link>("/api/links", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  getStats: (code: string) =>
    request<Stats>(`/api/links/${encodeURIComponent(code)}/stats`),
  deleteLink: (code: string) =>
    request<void>(`/api/links/${encodeURIComponent(code)}`, {
      method: "DELETE",
    }),
};
export type Api = typeof api;
