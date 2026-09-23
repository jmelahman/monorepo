export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export type Item = {
  id: number;
  title: string;
  created_at: string;
};

export type Health = {
  status: string;
  version: string;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (body && typeof body.error === "string") message = body.error;
    } catch {
      // Non-JSON error body; keep the status line.
    }
    throw new ApiError(res.status, message);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  health: () => request<Health>("/api/health"),
  listItems: () => request<Item[]>("/api/items"),
  createItem: (title: string) =>
    request<Item>("/api/items", { method: "POST", body: JSON.stringify({ title }) }),
  deleteItem: (id: number) => request<void>(`/api/items/${id}`, { method: "DELETE" }),
};
