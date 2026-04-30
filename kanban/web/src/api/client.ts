export type Board = {
  id: number;
  name: string;
  slug: string;
  source_repo_path: string;
  worktree_root: string;
  base_branch: string;
  created_at: number;
};

export type Column = { id: number; board_id: number; name: string; position: number };

export type Ticket = {
  id: number;
  board_id: number;
  column_id: number;
  title: string;
  slug: string;
  body: string;
  position: number;
  created_at: number;
  archived_at?: number;
};

export type Session = {
  id: number;
  ticket_id: number;
  worktree_path: string;
  branch_name: string;
  container_id?: string;
  container_name?: string;
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

export type DiscoveredTask = {
  label: string;
  command: string;
  args: string[];
  cwd: string;
  env: Record<string, string>;
  container_port?: number;
  has_port: boolean;
};

export type TaskRun = {
  id: number;
  session_id: number;
  task_label: string;
  command: string;
  status: string;
  exit_code?: number;
  started_at: number;
  stopped_at?: number;
};

export type PortAllocation = {
  id: number;
  session_id: number;
  label: string;
  container_port: number;
  host_port: number;
  proxy_active: boolean;
};

export class ApiError extends Error {
  status: number;
  body: string;
  constructor(status: number, message: string, body: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.body = body;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    ...init,
  });
  if (!res.ok) {
    const text = await res.text();
    let message = text;
    try {
      const parsed = JSON.parse(text);
      if (parsed && typeof parsed.error === "string") message = parsed.error;
    } catch {
      // not JSON; use raw body
    }
    throw new ApiError(res.status, message || `HTTP ${res.status}`, text);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  listBoards: () => request<Board[]>("/api/boards"),
  createBoard: (input: { name: string; source_repo_path: string; worktree_root?: string; base_branch?: string }) =>
    request<Board>("/api/boards", { method: "POST", body: JSON.stringify(input) }),
  boardState: (id: number) => request<BoardState>(`/api/boards/${id}/state`),

  createTicket: (boardId: number, input: { column_id: number; title: string; body?: string }) =>
    request<Ticket>(`/api/boards/${boardId}/tickets`, { method: "POST", body: JSON.stringify(input) }),
  moveTicket: (id: number, input: { column_id: number; position: number }) =>
    request<void>(`/api/tickets/${id}/move`, { method: "PATCH", body: JSON.stringify(input) }),
  archiveTicket: (id: number) => request<void>(`/api/tickets/${id}/archive`, { method: "POST" }),
  listArchivedTickets: (boardId: number) => request<Ticket[]>(`/api/boards/${boardId}/archived`),
  deleteTicket: (id: number) => request<void>(`/api/tickets/${id}`, { method: "DELETE" }),

  ensureSession: (ticketId: number) => request<Session>(`/api/tickets/${ticketId}/session`, { method: "POST" }),
  startSession: (id: number) => request<Session>(`/api/sessions/${id}/start`, { method: "POST" }),
  stopSession: (id: number) => request<void>(`/api/sessions/${id}/stop`, { method: "POST" }),

  discoverTasks: (sessionId: number) => request<DiscoveredTask[]>(`/api/sessions/${sessionId}/discover-tasks`),
  listTaskRuns: (sessionId: number) => request<TaskRun[]>(`/api/sessions/${sessionId}/task-runs`),
  startTaskRun: (sessionId: number, label: string) =>
    request<TaskRun>(`/api/sessions/${sessionId}/task-runs`, { method: "POST", body: JSON.stringify({ label }) }),
  stopTaskRun: (id: number) => request<void>(`/api/task-runs/${id}`, { method: "DELETE" }),

  listPorts: (sessionId: number) => request<PortAllocation[]>(`/api/sessions/${sessionId}/ports`),
  createPort: (sessionId: number, input: { label: string; container_port: number }) =>
    request<PortAllocation[]>(`/api/sessions/${sessionId}/ports`, { method: "POST", body: JSON.stringify(input) }),
  deletePort: (id: number) => request<void>(`/api/ports/${id}`, { method: "DELETE" }),
};

export type SubscribeOptions = {
  onEvent: (type: string, data: unknown) => void;
  onStatus?: (status: "open" | "error" | "closed") => void;
};

export function subscribeBoard(boardId: number, opts: SubscribeOptions): () => void {
  const es = new EventSource(`/api/boards/${boardId}/events`);
  const handler = (e: MessageEvent) => {
    try {
      opts.onEvent(e.type, JSON.parse(e.data));
    } catch {
      opts.onEvent(e.type, null);
    }
  };
  for (const t of ["ticket_created", "ticket_moved", "ticket_archived", "ticket_deleted", "session_updated", "ready"]) {
    es.addEventListener(t, handler as EventListener);
  }
  es.onopen = () => opts.onStatus?.("open");
  es.onerror = () => opts.onStatus?.("error");
  return () => {
    es.close();
    opts.onStatus?.("closed");
  };
}
