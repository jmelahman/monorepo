export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export type Health = {
  status: string;
  version: string;
  // Base domain previews are served under, e.g. "preview.localhost". Fixed
  // at server startup; the dashboard has no other way to learn it.
  preview_domain: string;
};

// Mirror-clone state of a registered repo. Registration returns while the
// clone runs in the background; a repo is deployable only once "ready".
export type RepoStatus = "cloning" | "ready" | "failed";

export type Repo = {
  id: number;
  name: string;
  source: string;
  // Watch polls the repo for new commits and deploys matching branch tips.
  // watch_branches narrows which branches (comma-separated globs, "" = all).
  watch: boolean;
  watch_branches: string;
  status: RepoStatus;
  // The clone failure message, while status is "failed".
  error?: string;
  // The clone's live progress line ("Receiving objects: 42% …"), while
  // status is "cloning" and the transport reports one.
  progress?: string;
  created_at: string;
};

export type DeployStatus = "queued" | "building" | "ready" | "failed" | "evicted";

// What GET /api/deploys accepts as `status`: a build status, or "crashed"
// for the ready deploys whose supervised process died — a runtime state no
// row carries, which the server resolves against the live supervisor.
export type DeployStatusFilter = DeployStatus | "crashed";

// Live runtime state of a supervised process, present on ready deploys:
// `process` for the backend, `fe_process` for process-mode frontends.
// "crashed" means the last run or start attempt ended unexpectedly — the
// next request still starts it, but nothing is serving in the meantime.
export type ProcessState = "idle" | "starting" | "running" | "crashed";

export type ArtifactFile = {
  name: string;
  size: number;
  // Download path on this host.
  url: string;
};

// Build state of one downloadable artifact. Artifacts build after the
// deploy itself turns ready — they never gate the preview — so a ready
// deploy can still be building (or have failed to build) an artifact.
export type ArtifactStatus = "building" | "ready" | "failed";

// A named downloadable artifact ([artifacts.<name>] in preview.toml),
// present on ready deploys.
export type DeployArtifact = {
  name: string;
  hash: string;
  status: ArtifactStatus;
  // Build failure summary, while status is "failed".
  error?: string;
  files: ArtifactFile[];
};

export type Deploy = {
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
  status: DeployStatus;
  error?: string;
  attempt_count: number;
  preview_url?: string;
  process?: ProcessState;
  // Why the side crashed (exit status or start failure), while its state is
  // "crashed".
  process_error?: string;
  fe_process?: ProcessState;
  fe_process_error?: string;
  artifacts?: DeployArtifact[];
  created_at: string;
  updated_at: string;
};

export type ProcessRuntime = "host" | "container";

// One side's live resource sample. Sampled fields are absent while the
// process isn't running; cpu_percent needs two samples, so it appears from
// the second poll onward.
export type SideStats = {
  state: ProcessState;
  // Why the side crashed, while its state is "crashed".
  error?: string;
  runtime?: ProcessRuntime;
  cpu_percent?: number;
  memory_bytes?: number;
  memory_limit_bytes?: number;
  started_at?: string;
};

// Live resource usage of a deploy's processes; a side the deploy doesn't
// have is null.
export type DeployStats = {
  backend: SideStats | null;
  frontend: SideStats | null;
};

// Instance-wide statistics (GET /api/stats). Percentile and ratio fields are
// absent until there's data to compute them from.
export type StatsReport = {
  startup: {
    count: number;
    p50_seconds?: number;
    p90_seconds?: number;
    p99_seconds?: number;
  };
  hits: {
    warm: number;
    cold: number;
    warm_ratio?: number;
  };
  runtime: {
    workers: number;
    running: number;
    capacity: number; // 0 = unlimited
  };
};

export type LogSide = "be" | "fe";

// An incremental slice of a process run log (the supervised server's
// stdout+stderr). Echo attempt/offset back to receive only new bytes; a
// changed attempt means the process restarted and the view reset.
export type RunLogChunk = {
  side: LogSide;
  attempt: number;
  offset: number;
  content: string;
  truncated?: boolean;
  process?: ProcessState;
};

// Warm-process policy, per serving node. max_warm is the soft target: above
// it, idle least-recently-used processes are pruned back — actively-used ones
// never are, so bursts serve in full (0 = unlimited). min_warm is the floor:
// that many most-recent processes never idle out. idle_timeout_seconds
// overrides every manifest's idle_timeout (0 = per-manifest, default 30m).
// A process-mode deploy counts as two processes (frontend + backend). Saved
// values override the server's flags/manifests and are pushed to every worker.
export type WarmPolicy = {
  max_warm: number;
  min_warm: number;
  idle_timeout_seconds: number;
};

// Instance-wide artifact retention policy. 0 disables a limit; with both at
// 0 the hourly sweep evicts nothing.
export type RetentionPolicy = {
  // Keep at most N non-evicted deploys per repo, newest first.
  max_deploys_per_repo: number;
  // Evict deploys created more than N days ago.
  max_age_days: number;
};

// One repo's slice of the storage report.
export type RepoUsage = {
  repo: string;
  artifacts_bytes: number;
  state_bytes: number;
  logs_bytes: number;
  mirror_bytes: number;
  total_bytes: number;
  deploys: number;
  evicted_deploys: number;
};

// GET /api/storage: how much disk the instance uses, by category and by repo.
export type StorageReport = {
  total_bytes: number;
  // Local-disk (resident) artifact bytes. With a durable tier configured this
  // is a cache of durable_bytes, not the whole retained set.
  artifacts_bytes: number;
  state_bytes: number;
  logs_bytes: number;
  mirror_bytes: number;
  tmp_bytes: number;
  db_bytes: number;
  // Whether a durable artifact tier backs local disk. When true, a shrinking
  // artifacts_bytes reflects cache eviction (artifacts re-hydrate on serve),
  // not data loss. durable_bytes is the tier's total footprint (0 if unknown);
  // it is not included in total_bytes, which measures local disk.
  durable_tier_configured: boolean;
  durable_bytes: number;
  repos: RepoUsage[];
};

export type EvictedDeploy = {
  id: number;
  repo: string;
  short_sha: string;
  branch?: string;
};

// POST /api/gc: what one sweep evicted and how much disk it gave back.
export type GCResult = {
  policy: RetentionPolicy;
  evicted: EvictedDeploy[];
  freed_bytes: number;
};

// Narrows GET /api/deploys; empty fields don't filter. q is a free-text
// search matching a commit-sha prefix or a substring of the repo, branch,
// ref, or author (case-insensitive); status matches the build status
// exactly, plus "crashed" for the ready deploys whose process died — those
// are excluded from "ready"; limit and offset take one page of the
// newest-first order.
export type DeployFilter = {
  q?: string;
  status?: DeployStatusFilter | "";
  limit?: number;
  offset?: number;
};

// One page of deploys plus how many match the filter in total, so a pager
// knows whether another page exists without fetching it.
export type DeployPage = {
  deploys: Deploy[];
  total: number;
};

// The current viewer. `anonymous` is true when the server has SSO disabled
// (the API is open) — the dashboard then renders without a login wall. When
// SSO is on, the fields describe the signed-in GitHub account.
export type Me = {
  anonymous?: boolean;
  login?: string;
  email?: string;
  avatar_url?: string;
};

async function send(path: string, init?: RequestInit): Promise<Response> {
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
  return res;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await send(path, init);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

// Watch settings a PATCH /api/repos/{name} can update; omitted fields keep
// their stored value.
export type RepoUpdate = {
  watch?: boolean;
  watch_branches?: string;
};

export const api = {
  health: () => request<Health>("/api/health"),
  me: () => request<Me>("/api/auth/me"),
  logout: () => request<void>("/api/auth/logout", { method: "POST" }),
  listRepos: () => request<Repo[]>("/api/repos"),
  getRepo: (name: string) => request<Repo>(`/api/repos/${encodeURIComponent(name)}`),
  createRepo: (name: string, source: string) =>
    request<Repo>("/api/repos", { method: "POST", body: JSON.stringify({ name, source }) }),
  updateRepo: (name: string, patch: RepoUpdate) =>
    request<Repo>(`/api/repos/${encodeURIComponent(name)}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  deleteRepo: (name: string) =>
    request<void>(`/api/repos/${encodeURIComponent(name)}`, { method: "DELETE" }),
  listDeploys: async (filter?: DeployFilter): Promise<DeployPage> => {
    const params = new URLSearchParams();
    if (filter?.q) params.set("q", filter.q);
    if (filter?.status) params.set("status", filter.status);
    if (filter?.limit) params.set("limit", String(filter.limit));
    if (filter?.offset) params.set("offset", String(filter.offset));
    const qs = params.toString();
    const res = await send(`/api/deploys${qs ? `?${qs}` : ""}`);
    const deploys = (await res.json()) as Deploy[];
    // The count of everything matching the filter, not just this page.
    const total = res.headers.get("X-Total-Count");
    return { deploys, total: total === null ? deploys.length : Number(total) };
  },
  getDeploy: (id: number) => request<Deploy>(`/api/deploys/${id}`),
  createDeploy: (repo: string, ref: string) =>
    request<Deploy>("/api/deploys", { method: "POST", body: JSON.stringify({ repo, ref }) }),
  stopDeploy: (id: number) => request<Deploy>(`/api/deploys/${id}/stop`, { method: "POST" }),
  deleteDeploy: (id: number) => request<void>(`/api/deploys/${id}`, { method: "DELETE" }),
  getDeployStats: (id: number) => request<DeployStats>(`/api/deploys/${id}/stats`),
  getRunLog: (id: number, side: LogSide, attempt: number, offset: number) =>
    request<RunLogChunk>(
      `/api/deploys/${id}/logs/run?side=${side}&attempt=${attempt}&offset=${offset}`,
    ),
  getStorage: () => request<StorageReport>("/api/storage"),
  getRetention: () => request<RetentionPolicy>("/api/retention"),
  putRetention: (policy: RetentionPolicy) =>
    request<RetentionPolicy>("/api/retention", { method: "PUT", body: JSON.stringify(policy) }),
  getStats: () => request<StatsReport>("/api/stats"),
  getWarm: () => request<WarmPolicy>("/api/warm"),
  putWarm: (policy: WarmPolicy) =>
    request<WarmPolicy>("/api/warm", { method: "PUT", body: JSON.stringify(policy) }),
  runGC: () => request<GCResult>("/api/gc", { method: "POST" }),
  getBuildLogs: async (id: number): Promise<string> => {
    const res = await fetch(`/api/deploys/${id}/logs`);
    if (!res.ok) throw new ApiError(res.status, `${res.status} ${res.statusText}`);
    return res.text();
  },
};
