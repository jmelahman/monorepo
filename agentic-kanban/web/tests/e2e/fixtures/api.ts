// Thin typed wrappers over the kanban HTTP API so specs don't reach into
// `web/src/api/client.ts` (which runs in a browser context). Each call
// targets the backend directly so seeding stays out of the UI's render
// cycle.

const BACKEND = process.env.KANBAN_E2E_BACKEND ?? "http://localhost:7474";

export type Board = {
  id: number;
  name: string;
  slug: string;
  mount_path: string;
  base_branch: string;
};

export type Column = { id: number; board_id: number; name: string; position: number };

export type Ticket = { id: number; board_id: number; column_id: number; title: string };

export type Session = {
  id: number;
  ticket_id: number;
  status: string;
  started_at?: number;
  stopped_at?: number;
};

export type BoardState = {
  board: Board;
  columns: Column[];
  tickets: Ticket[];
  sessions: Session[];
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BACKEND}${path}`, {
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    ...init,
  });
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`${init?.method ?? "GET"} ${path} → ${res.status}: ${body}`);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  createBoard: (input: { name: string; mount_path: string }) =>
    request<Board>("/api/boards", { method: "POST", body: JSON.stringify(input) }),
  deleteBoard: (id: number) =>
    request<void>(`/api/boards/${id}`, { method: "DELETE" }),
  listBoards: () => request<Board[]>("/api/boards"),
  createTicket: (boardId: number, input: { title: string; body?: string }) =>
    request<Ticket>(`/api/boards/${boardId}/tickets`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  ensureSession: (ticketId: number) =>
    request<Session>(`/api/tickets/${ticketId}/session`, { method: "POST" }),
  startSession: (id: number) =>
    request<Session>(`/api/sessions/${id}/start`, { method: "POST" }),
  stopSession: (id: number) =>
    request<void>(`/api/sessions/${id}/stop`, { method: "POST" }),
  restartSession: (id: number) =>
    request<Session>(`/api/sessions/${id}/restart`, { method: "POST" }),
  boardState: (id: number) => request<BoardState>(`/api/boards/${id}/state`),
};

// Polls `boardState` until the named session matches `predicate`. Used to
// gate UI assertions on a server-side status transition without racing the
// SSE bus, which drops events when subscribers fall behind.
export async function waitForSession(
  boardId: number,
  sessionId: number,
  predicate: (s: Session) => boolean,
  opts: { timeoutMs?: number; intervalMs?: number } = {},
): Promise<Session> {
  const timeoutMs = opts.timeoutMs ?? 60_000;
  const intervalMs = opts.intervalMs ?? 250;
  const deadline = Date.now() + timeoutMs;
  let last: Session | undefined;
  while (Date.now() < deadline) {
    const state = await api.boardState(boardId);
    last = state.sessions.find((s) => s.id === sessionId);
    if (last && predicate(last)) return last;
    await new Promise((r) => setTimeout(r, intervalMs));
  }
  throw new Error(
    `waitForSession(${sessionId}) timeout after ${timeoutMs}ms; last status=${last?.status ?? "<missing>"}`,
  );
}

export const isAttachable = (s: Session): boolean =>
  s.status === "idle" || s.status === "working" || s.status === "awaiting_perm";
