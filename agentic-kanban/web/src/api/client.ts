import { sseStatus, trackRequestEnd, trackRequestStart } from "@/api/runtimeSignals";

export type Board = {
  id: number;
  name: string;
  slug: string;
  repo_path: string;
  mount_path: string;
  project_dir: string;
  worktree_root: string;
  base_branch: string;
  branch_prefix: string;
  git_author_name: string;
  git_author_email: string;
  created_at: number;
  position: number;
};

// BoardEnv lists env var key names for a board. Values are write-only
// secrets and never leave the server.
export type BoardEnv = { keys: string[] };

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

export type PRState = "draft" | "open" | "merged" | "closed";

export const PR_STATE_COLOR: Record<PRState, string> = {
  draft: "text-zinc-400",
  open: "text-emerald-400",
  merged: "text-purple-400",
  closed: "text-red-400",
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
  pr_state?: PRState | "";
  pr_number?: number;
  pr_url?: string;
  pr_title?: string;
  claude_session_id?: string;
  harness?: string;
};

// SessionSummary is an instance-wide count of running containers, broken down
// by status. `running` is the sum of the four active states (working, awaiting,
// idle, starting); stopped and error sessions are excluded.
export type SessionSummary = {
  running: number;
  working: number;
  awaiting_perm: number;
  idle: number;
  starting: number;
};

export type PRReviewDecision = "approved" | "changes_requested" | "review_required" | "";

export type PRCheckEntry = { name: string; url?: string };

export type PRChecks = {
  total: number;
  success: number;
  failure: number;
  pending: number;
  failing: PRCheckEntry[];
};

export type PRDetail = {
  additions: number;
  deletions: number;
  review_decision: PRReviewDecision;
  checks: PRChecks;
};

// PullProgress is an aggregate snapshot of an in-flight image pull, summed
// across layers. `total` is 0 until the daemon reports the first byte counter.
export type PullProgress = {
  session_id: number;
  image: string;
  current: number;
  total: number;
  layers: number;
  status: string;
  done: boolean;
};

export type MergeConfig = {
  allow_merge_commit: boolean;
  allow_squash: boolean;
  allow_rebase: boolean;
  /** Strategy used when a merge omits one; "" when unset. */
  default_strategy: string;
};

export type SyncConfig = {
  allow_rebase: boolean;
  allow_merge: boolean;
};

export type BoardState = {
  board: Board;
  columns: Column[];
  tickets: Ticket[];
  sessions: Session[];
  merge_config: MergeConfig;
  sync_config: SyncConfig;
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

export type DiscoverTasksResult = {
  tasks: DiscoveredTask[];
  warnings: string[];
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

/** One downloadable file of a deploy's [artifacts.<name>] section. */
export type PreviewArtifactFile = {
  name: string;
  size: number;
};

/** A named set of per-commit build outputs published for download. */
export type PreviewArtifact = {
  name: string;
  hash: string;
  files: PreviewArtifactFile[];
};

export type Preview = {
  id: number;
  repo: string;
  sha: string;
  short_sha: string;
  ref?: string;
  branch?: string;
  author_name?: string;
  author_email?: string;
  fe_hash?: string;
  be_hash?: string;
  status: "queued" | "building" | "ready" | "failed" | "evicted";
  error?: string;
  attempt_count: number;
  preview_url?: string;
  /** Live backend state once ready: running, starting, or idle. */
  process?: string;
  /** Same, for a process-mode frontend; absent for static bundles. */
  fe_process?: string;
  artifacts?: PreviewArtifact[];
  created_at: string;
  updated_at: string;
};

/**
 * A deploy from the cross-board dashboard feed, joined to the board that owns
 * it. The board fields are absent when a deploy outlives its board (renamed
 * or deleted) — the orchestrator only ever knows the repo name.
 */
export type DashboardPreview = Preview & {
  board_id?: number;
  board_name?: string;
  board_slug?: string;
};

/** One board's share of the preview orchestrator's disk usage. */
export type PreviewStorageRepo = {
  repo: string;
  artifacts_bytes: number;
  state_bytes: number;
  logs_bytes: number;
  mirror_bytes: number;
  total_bytes: number;
  deploys: number;
  evicted_deploys: number;
  board_id?: number;
  board_name?: string;
  board_slug?: string;
};

/** What the preview orchestrator occupies on disk, by category and board. */
export type PreviewStorage = {
  total_bytes: number;
  artifacts_bytes: number;
  state_bytes: number;
  logs_bytes: number;
  mirror_bytes: number;
  tmp_bytes: number;
  db_bytes: number;
  repos: PreviewStorageRepo[];
};

/** Limits on how many preview deploys are kept. 0 means unlimited. */
export type RetentionPolicy = {
  max_deploys_per_repo: number;
  max_age_days: number;
};

/** The outcome of one retention sweep. */
export type GCResult = {
  policy: RetentionPolicy;
  evicted: { id: number; repo: string; short_sha: string; branch?: string }[];
  freed_bytes: number;
};

export type AppSettings = {
  harness: string;
  worktrees_root: string;
  worktrees_root_resolved: string;
  worktrees_root_locked: boolean;
  sign_commits: boolean;
};

export type Harness = { id: string; label: string; pty_command: string[]; default?: boolean };

export type Version = { version: string };

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

// Error-handling convention: mutations should not call toast.push("error", ...)
// for the default case. The QueryClient in main.tsx is configured with a global
// onError handler that toasts every mutation/query failure via this helper.
// Per-mutation onError is reserved for behaviors beyond toasting (closing a
// menu, rolling back optimistic state, resetting a ref, etc.).
export function formatApiError(err: unknown): string {
  if (err instanceof ApiError) return `${err.status}: ${err.message}`;
  if (err instanceof Error) return err.message;
  return String(err);
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  // Count the request as in-flight for the developer toolbar. finally covers
  // every exit (network reject, !ok throw, 204, JSON body) so the gauge can't
  // leak. Long-lived SSE streams go through EventSource, not request(), so
  // they're excluded by construction.
  trackRequestStart();
  try {
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
    return (await res.json()) as T;
  } finally {
    trackRequestEnd();
  }
}

export const api = {
  listBoards: () => request<Board[]>("/api/boards"),
  createBoard: (input: {
    name: string;
    repo_path?: string;
    mount_path?: string;
    project_dir?: string;
    worktree_root?: string;
    base_branch?: string;
    branch_prefix?: string;
    git_author_name?: string;
    git_author_email?: string;
  }) => request<Board>("/api/boards", { method: "POST", body: JSON.stringify(input) }),
  updateBoard: (
    id: number,
    input: {
      name?: string;
      repo_path?: string;
      mount_path?: string;
      project_dir?: string;
      worktree_root?: string;
      base_branch?: string;
      branch_prefix?: string;
      git_author_name?: string;
      git_author_email?: string;
    },
  ) => request<Board>(`/api/boards/${id}`, { method: "PATCH", body: JSON.stringify(input) }),
  deleteBoard: (id: number) => request<void>(`/api/boards/${id}`, { method: "DELETE" }),
  moveBoard: (id: number, position: number) =>
    request<void>(`/api/boards/${id}/move`, {
      method: "PATCH",
      body: JSON.stringify({ position }),
    }),
  boardState: (id: number) => request<BoardState>(`/api/boards/${id}/state`),
  // Board env vars are write-only: responses carry key names only, never values.
  listBoardEnv: (id: number) => request<BoardEnv>(`/api/boards/${id}/env`),
  patchBoardEnv: (id: number, input: { set?: Record<string, string>; unset?: string[] }) =>
    request<BoardEnv>(`/api/boards/${id}/env`, { method: "PATCH", body: JSON.stringify(input) }),

  createTicket: (boardId: number, input: { column_id: number; title: string; body?: string }) =>
    request<Ticket>(`/api/boards/${boardId}/tickets`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  updateTicket: (id: number, input: { title?: string; body?: string }) =>
    request<Ticket>(`/api/tickets/${id}`, { method: "PATCH", body: JSON.stringify(input) }),
  moveTicket: (id: number, input: { column_id: number; position: number }) =>
    request<void>(`/api/tickets/${id}/move`, { method: "PATCH", body: JSON.stringify(input) }),
  archiveTicket: (id: number) => request<void>(`/api/tickets/${id}/archive`, { method: "POST" }),
  archiveColumnTickets: (columnId: number) =>
    request<void>(`/api/columns/${columnId}/archive-all`, { method: "POST" }),
  unarchiveTicket: (id: number) =>
    request<void>(`/api/tickets/${id}/unarchive`, { method: "POST" }),
  listArchivedTickets: (boardId: number) => request<Ticket[]>(`/api/boards/${boardId}/archived`),
  deleteTicket: (id: number) => request<void>(`/api/tickets/${id}`, { method: "DELETE" }),
  deleteAllArchived: (boardId: number) =>
    request<void>(`/api/boards/${boardId}/archived`, { method: "DELETE" }),
  syncTicket: (id: number, strategy: "rebase" | "merge") =>
    request<void>(`/api/tickets/${id}/sync`, {
      method: "POST",
      body: JSON.stringify({ strategy }),
    }),
  mergeTicket: (id: number, strategy: "merge-commit" | "squash" | "rebase") =>
    request<void>(`/api/tickets/${id}/merge`, {
      method: "POST",
      body: JSON.stringify({ strategy }),
    }),
  doneTicket: (id: number) => request<void>(`/api/tickets/${id}/done`, { method: "POST" }),

  sessionSummary: () => request<SessionSummary>("/api/sessions/summary"),

  ensureSession: (ticketId: number) =>
    request<Session>(`/api/tickets/${ticketId}/session`, { method: "POST" }),
  startSession: (id: number) => request<Session>(`/api/sessions/${id}/start`, { method: "POST" }),
  stopSession: (id: number) => request<void>(`/api/sessions/${id}/stop`, { method: "POST" }),
  restartSession: (id: number) =>
    request<Session>(`/api/sessions/${id}/restart`, { method: "POST" }),
  repointSessionBranch: (id: number, branch_name: string) =>
    request<Session>(`/api/sessions/${id}/branch`, {
      method: "PATCH",
      body: JSON.stringify({ branch_name }),
    }),

  discoverTasks: (sessionId: number) =>
    request<DiscoverTasksResult>(`/api/sessions/${sessionId}/discover-tasks`),
  listTaskRuns: (sessionId: number) => request<TaskRun[]>(`/api/sessions/${sessionId}/task-runs`),
  startTaskRun: (sessionId: number, label: string) =>
    request<TaskRun>(`/api/sessions/${sessionId}/task-runs`, {
      method: "POST",
      body: JSON.stringify({ label }),
    }),
  stopTaskRun: (id: number) => request<void>(`/api/task-runs/${id}`, { method: "DELETE" }),

  prDetail: (sessionId: number) => request<PRDetail>(`/api/sessions/${sessionId}/pr-detail`),

  listPorts: (sessionId: number) => request<PortAllocation[]>(`/api/sessions/${sessionId}/ports`),
  createPort: (sessionId: number, input: { label: string; container_port: number }) =>
    request<PortAllocation[]>(`/api/sessions/${sessionId}/ports`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  deletePort: (id: number) => request<void>(`/api/ports/${id}`, { method: "DELETE" }),

  listPreviews: (sessionId: number) => request<Preview[]>(`/api/sessions/${sessionId}/previews`),
  createPreview: (sessionId: number) =>
    request<Preview>(`/api/sessions/${sessionId}/previews`, { method: "POST" }),

  listAllPreviews: () => request<DashboardPreview[]>("/api/previews"),
  previewLogs: async (previewId: number): Promise<string> => {
    const res = await fetch(`/api/previews/${previewId}/logs`);
    if (!res.ok) throw new ApiError(res.status, `HTTP ${res.status}`, "");
    return res.text();
  },
  createBoardPreview: (boardId: number, ref?: string) =>
    request<Preview>(`/api/boards/${boardId}/previews`, {
      method: "POST",
      body: JSON.stringify({ ref: ref ?? "" }),
    }),
  stopPreview: (previewId: number) =>
    request<void>(`/api/previews/${previewId}/stop`, { method: "POST" }),
  deletePreview: (previewId: number) =>
    request<void>(`/api/previews/${previewId}`, { method: "DELETE" }),
  previewStorage: () => request<PreviewStorage>("/api/previews/storage"),
  previewRetention: () => request<RetentionPolicy>("/api/previews/retention"),
  setPreviewRetention: (policy: RetentionPolicy) =>
    request<RetentionPolicy>("/api/previews/retention", {
      method: "PUT",
      body: JSON.stringify(policy),
    }),
  collectPreviewGarbage: () => request<GCResult>("/api/previews/gc", { method: "POST" }),

  getSettings: () => request<AppSettings>("/api/settings"),
  updateSettings: (input: { harness?: string; worktrees_root?: string; sign_commits?: boolean }) =>
    request<AppSettings>("/api/settings", { method: "PATCH", body: JSON.stringify(input) }),
  listHarnesses: () => request<Harness[]>("/api/harnesses"),

  fsCheck: (path: string) =>
    request<{ state: "git" | "not_git" | "unknown" }>(
      `/api/fs/check?path=${encodeURIComponent(path)}`,
    ),

  listSessionPlans: (sessionId: number) => request<PlanMeta[]>(`/api/sessions/${sessionId}/plans`),
  getSessionPlan: async (sessionId: number, name: string): Promise<string> => {
    const res = await fetch(`/api/sessions/${sessionId}/plans/${encodeURIComponent(name)}`);
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
    return res.text();
  },

  getSessionDiff: (sessionId: number) => request<SessionDiff>(`/api/sessions/${sessionId}/diff`),

  getSessionFile: (sessionId: number, path: string) =>
    request<SessionFile>(`/api/sessions/${sessionId}/file?path=${encodeURIComponent(path)}`),

  getSessionFileDiff: (sessionId: number, path: string, oldPath?: string) => {
    const params = new URLSearchParams({ path });
    if (oldPath && oldPath !== path) params.set("old_path", oldPath);
    return request<SessionFileDiff>(`/api/sessions/${sessionId}/file-diff?${params}`);
  },

  getVersion: () => request<Version>("/api/version"),

  // Reads the merged config surface (GET /api/config). scope defaults to
  // "effective" (project file + user file merged). Entries are the registered
  // dotted keys plus a few read-only runtime values.
  getConfig: (scope: ConfigScope = "effective") =>
    request<ConfigView>(`/api/config?scope=${encodeURIComponent(scope)}`),
};

export type ConfigScope = "effective" | "global" | "local";

export type ConfigEntry = {
  key: string;
  value: unknown;
  source: string;
  writable: boolean;
};

export type ConfigView = {
  scope: string;
  board?: string;
  entries: ConfigEntry[];
};

export type PlanMeta = {
  name: string;
  mod_time: string;
  size: number;
};

// SessionDiff is the unified patch of a session worktree vs. where its branch
// diverged from the board base (committed + uncommitted tracked changes).
// `patch` is empty when there are no changes (or the session has no worktree).
export type SessionDiff = {
  base: string;
  patch: string;
};

// SessionFile is the current working-tree contents of a single file in a
// session worktree — the "new" side of SessionDiff — used by the diff viewer's
// per-file "View file" (whole-file) view.
export type SessionFile = {
  path: string;
  contents: string;
};

// SessionFileDiff carries both sides of one changed file — the merge-base
// version and the current working-tree version — so the diff viewer can rebuild
// the file's diff with full surrounding context and offer GitHub-style
// expand-up/down. A side that doesn't exist at its ref comes back empty: an
// empty `old_contents` is a newly-added file, an empty `new_contents` a deleted
// one.
export type SessionFileDiff = {
  path: string;
  old_contents: string;
  new_contents: string;
};

export type SubscribeOptions = {
  onEvent: (type: string, data: unknown) => void;
  onStatus?: (status: "open" | "error" | "closed") => void;
};

// postError reports a frontend exception to the backend, which may file a
// kanban ticket if error reporting is enabled in `.kanban.toml`. Always
// best-effort: failures are silently swallowed.
//
// Uses raw fetch (not request<T>) so an HTTP error here can't throw into the
// React Query global onError and re-trigger this function.
//
// Concurrency: capped at POST_ERROR_MAX_IN_FLIGHT concurrent requests so a
// burst (PerformanceObserver flushing many buffered longtasks at once) can't
// be silenced by a single in-flight call. Each request gets a hard timeout
// via AbortController — without this, a wedged backend would permanently
// stick the in-flight count at the cap and mute all future reports.
const POST_ERROR_MAX_IN_FLIGHT = 4;
const POST_ERROR_TIMEOUT_MS = 3000;
let postErrorInFlight = 0;
export async function postError(p: {
  message: string;
  stack?: string;
  source: string;
  url?: string;
  user_agent?: string;
  meta?: Record<string, string>;
}): Promise<void> {
  if (postErrorInFlight >= POST_ERROR_MAX_IN_FLIGHT) return;
  postErrorInFlight++;
  const controller = new AbortController();
  const timeoutID = setTimeout(() => controller.abort(), POST_ERROR_TIMEOUT_MS);
  try {
    await fetch("/api/errors", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        message: p.message,
        stack: p.stack ?? "",
        source: p.source,
        url: p.url ?? (typeof window !== "undefined" ? window.location.href : ""),
        user_agent: p.user_agent ?? (typeof navigator !== "undefined" ? navigator.userAgent : ""),
        meta: p.meta,
      }),
      signal: controller.signal,
    });
  } catch {
    // never toast, never recurse
  } finally {
    clearTimeout(timeoutID);
    postErrorInFlight--;
  }
}

// Event types fanned out from the backend bus. Kept in sync with the
// handlers in internal/api/handlers.go that call h.bus.Publish.
const BOARD_EVENT_TYPES = [
  "ticket_created",
  "ticket_updated",
  "ticket_moved",
  "ticket_archived",
  "ticket_unarchived",
  "ticket_deleted",
  "session_updated",
  "session_pull_progress",
  "ready",
] as const;

type MultiplexListener = {
  boardID: number;
  onEvent: (type: string, data: unknown) => void;
  onStatus?: (status: "open" | "error" | "closed") => void;
};

// boardEventManager keeps a single EventSource open for the union of board
// ids any consumer is currently listening on, and demultiplexes incoming
// events by board_id. The dedicated /api/boards/{id}/events endpoint still
// works for one-off callers, but the multiplexed /api/events?boards=… stream
// avoids saturating the browser's HTTP/1.1 per-origin connection pool (6
// connections on Chrome/Firefox) when Overview subscribes to many boards at
// once. Without this, a freshly-started session's WebSocket handshake gets
// queued behind the SSE streams and never opens — the terminal hangs on a
// blank cursor until the user refreshes.
// See REGRESSIONS.md: "Per-origin SSE streams starve the WebSocket pool".
class BoardEventManager {
  private listeners = new Set<MultiplexListener>();
  private es: EventSource | null = null;
  private currentKey = "";
  private syncQueued = false;

  subscribe(l: MultiplexListener): () => void {
    this.listeners.add(l);
    this.scheduleSync();
    return () => {
      this.listeners.delete(l);
      this.scheduleSync();
    };
  }

  // Coalesces rapid add/remove sequences (e.g. BoardTree mounting subscribers
  // for N boards in one render) into one EventSource open instead of N.
  private scheduleSync(): void {
    if (this.syncQueued) return;
    this.syncQueued = true;
    queueMicrotask(() => {
      this.syncQueued = false;
      this.sync();
    });
  }

  private sync(): void {
    const ids = Array.from(new Set(Array.from(this.listeners, (l) => l.boardID))).sort(
      (a, b) => a - b,
    );
    const key = ids.join(",");
    if (key === this.currentKey) return;
    if (this.es) {
      this.es.close();
      this.es = null;
      sseStatus.set("closed");
    }
    this.currentKey = key;
    if (ids.length === 0) return;

    const es = new EventSource(`/api/events?boards=${key}`);
    const handler = (e: MessageEvent) => {
      let parsed: unknown;
      try {
        parsed = JSON.parse(e.data);
      } catch {
        return;
      }
      if (!parsed || typeof parsed !== "object") return;
      const obj = parsed as { board_id?: unknown; data?: unknown };
      const boardID = typeof obj.board_id === "number" ? obj.board_id : null;
      if (boardID == null) return;
      const data = obj.data;
      for (const l of this.listeners) {
        if (l.boardID === boardID) l.onEvent(e.type, data);
      }
    };
    for (const t of BOARD_EVENT_TYPES) {
      es.addEventListener(t, handler as EventListener);
    }
    es.onopen = () => {
      sseStatus.set("open");
      for (const l of this.listeners) l.onStatus?.("open");
    };
    es.onerror = () => {
      sseStatus.set("error");
      for (const l of this.listeners) l.onStatus?.("error");
    };
    this.es = es;
  }
}

const boardEventManager = new BoardEventManager();

export function subscribeBoard(boardId: number, opts: SubscribeOptions): () => void {
  const cleanup = boardEventManager.subscribe({
    boardID: boardId,
    onEvent: opts.onEvent,
    onStatus: opts.onStatus,
  });
  return () => {
    cleanup();
    opts.onStatus?.("closed");
  };
}
