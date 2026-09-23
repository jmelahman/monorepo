import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { api, type Board, type DashboardPreview, type PreviewArtifact } from "@/api/client";
import { queryKeys } from "@/api/keys";
import {
  BranchIcon,
  DatabaseIcon,
  DownloadIcon,
  ExternalLinkIcon,
  RocketIcon,
  TrashIcon,
} from "@/icons";
import { Button } from "./Button";
import { ConfirmModal, Modal } from "./Modal";

// The previews dashboard: every deploy the embedded local-preview
// orchestrator knows about, across every board, plus the controls to deploy
// an arbitrary ref. The per-session previews tab covers one branch; this is
// the whole fleet.

/**
 * A deploy's displayed state: its build status until it's ready, then the
 * merged runtime state of whichever sides it supervises — starting while any
 * warms up, running only once every side is warm, idle otherwise.
 */
type DeployState = DashboardPreview["status"] | "starting" | "running" | "idle";

function deployState(p: DashboardPreview): DeployState {
  if (p.status !== "ready") return p.status;
  const procs = [p.process, p.fe_process].filter((s): s is string => !!s);
  if (procs.length === 0) return p.status;
  if (procs.includes("starting")) return "starting";
  return procs.every((s) => s === "running") ? "running" : "idle";
}

const stateStyles: Record<DeployState, { dot: string; pill: string; hint: string }> = {
  queued: {
    dot: "bg-fg-muted",
    pill: "border-border text-fg-muted",
    hint: "Waiting for a build slot",
  },
  building: {
    dot: "animate-pulse bg-amber-400",
    pill: "border-amber-500/30 text-amber-400",
    hint: "Build in progress",
  },
  ready: {
    dot: "bg-emerald-400",
    pill: "border-emerald-500/30 text-emerald-400",
    hint: "Static preview — served instantly",
  },
  idle: {
    dot: "bg-emerald-400/40",
    pill: "border-emerald-500/20 text-emerald-400/70",
    hint: "Built — starts on the first request",
  },
  starting: {
    dot: "animate-pulse bg-emerald-400",
    pill: "border-emerald-500/30 text-emerald-400",
    hint: "Starting up",
  },
  running: {
    dot: "bg-emerald-400",
    pill: "border-emerald-500/30 text-emerald-400",
    hint: "Warm — served instantly",
  },
  failed: { dot: "bg-danger", pill: "border-danger/30 text-danger", hint: "Build failed" },
  evicted: {
    dot: "bg-fg-muted/50",
    pill: "border-border text-fg-muted/70 line-through",
    hint: "Artifacts were cleaned up — redeploy to rebuild",
  },
};

function StatusBadge({ state }: { state: DeployState }) {
  const s = stateStyles[state];
  return (
    <span
      title={s.hint}
      className={`inline-flex w-[5.5rem] shrink-0 items-center justify-center gap-1.5 rounded-full border py-0.5 text-xs font-medium ${s.pill}`}
    >
      <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${s.dot}`} />
      {state}
    </span>
  );
}

function timeAgo(iso: string): string {
  const secs = Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 1000));
  if (secs < 45) return "just now";
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${Math.max(1, mins)}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KiB", "MiB", "GiB", "TiB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v >= 100 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`;
}

const pillClass =
  "inline-flex shrink-0 items-center gap-1 rounded bg-surface-2 px-2 py-0.5 text-xs text-fg transition-colors duration-150 hover:bg-surface-3";

/**
 * One artifact's download affordance: a plain link when the build produced a
 * single file, a disclosure listing every file (one binary per platform, say)
 * when it produced several.
 */
function ArtifactDownload({
  previewId,
  artifact,
}: {
  previewId: number;
  artifact: PreviewArtifact;
}) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: PointerEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const href = (file: string) =>
    `/api/previews/${previewId}/artifacts/${encodeURIComponent(artifact.name)}/${encodeURIComponent(file)}`;

  if (artifact.files.length === 1) {
    const f = artifact.files[0];
    return (
      <a
        href={href(f.name)}
        download={f.name}
        title={`${f.name} (${formatBytes(f.size)})`}
        className={`${pillClass} font-mono`}
      >
        <DownloadIcon size={12} />
        {artifact.name}
      </a>
    );
  }

  return (
    <div ref={wrapRef} className="relative shrink-0">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        title={`Download ${artifact.name} for your platform`}
        onClick={() => setOpen((v) => !v)}
        className={`${pillClass} font-mono`}
      >
        <DownloadIcon size={12} />
        {artifact.name}
        <span className="text-fg-muted">▾</span>
      </button>
      {open && (
        <div
          role="menu"
          className="absolute right-0 top-full z-10 mt-1 whitespace-nowrap rounded border border-border bg-bg py-1 shadow-lg"
        >
          {artifact.files.map((f) => (
            <a
              key={f.name}
              role="menuitem"
              href={href(f.name)}
              download={f.name}
              onClick={() => setOpen(false)}
              className="flex items-baseline justify-between gap-4 px-3 py-1.5 font-mono text-xs text-fg transition-colors duration-150 hover:bg-surface-2"
            >
              {f.name}
              <span className="tabular-nums text-[10px] text-fg-muted">{formatBytes(f.size)}</span>
            </a>
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * The deploy's build-log snapshot. Kanban's orchestrator is embedded rather
 * than standing alone, so build output is all it exposes — there are no
 * per-process run logs to tail here.
 */
function BuildLogModal({ preview, onClose }: { preview: DashboardPreview; onClose: () => void }) {
  const building = preview.status === "queued" || preview.status === "building";
  const logsQ = useQuery({
    queryKey: ["preview-logs", preview.id],
    queryFn: () => api.previewLogs(preview.id),
    refetchInterval: building ? 1000 : false,
  });
  const ref = useRef<HTMLPreElement>(null);
  const pinned = useRef(true);
  // biome-ignore lint/correctness/useExhaustiveDependencies: new log content drives the re-scroll.
  useEffect(() => {
    const el = ref.current;
    if (el && pinned.current) el.scrollTop = el.scrollHeight;
  }, [logsQ.data]);

  return (
    <Modal
      open
      wide
      onClose={onClose}
      title={`${preview.board_name ?? preview.repo} @ ${preview.short_sha}`}
    >
      <div className="flex flex-col gap-1 p-4">
        <pre
          ref={ref}
          onScroll={() => {
            const el = ref.current;
            if (el) pinned.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
          }}
          className="h-72 overflow-auto whitespace-pre-wrap break-words rounded border border-border bg-surface p-2 font-mono text-xs leading-relaxed"
        >
          {logsQ.error ? String(logsQ.error) : (logsQ.data ?? "loading…")}
        </pre>
        <p className="font-mono text-[11px] text-fg-muted">
          {building
            ? "build in progress — refreshing"
            : "frontend, backend, and artifact build output"}
        </p>
      </div>
    </Modal>
  );
}

/** Deploy any ref of any git-linked board. Empty ref means the base branch. */
function DeployModal({ boards, onClose }: { boards: Board[]; onClose: () => void }) {
  const qc = useQueryClient();
  const [boardId, setBoardId] = useState(boards[0]?.id ?? 0);
  const [ref, setRef] = useState("");
  const board = boards.find((b) => b.id === boardId);
  const deployMut = useMutation({
    mutationFn: () => api.createBoardPreview(boardId, ref.trim()),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.allPreviews });
      onClose();
    },
  });

  return (
    <Modal open onClose={onClose} title="Deploy a commit" busy={deployMut.isPending}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (boardId) deployMut.mutate();
        }}
      >
        <div className="flex flex-col gap-3 p-4 text-sm">
          <p className="text-xs text-fg-muted">
            Builds are content-addressed per commit — an unchanged side is reused, not rebuilt.
          </p>
          <label className="flex flex-col gap-1">
            <span className="text-xs text-fg-muted">Board</span>
            <select
              value={boardId}
              onChange={(e) => setBoardId(Number(e.target.value))}
              className="w-full cursor-pointer rounded bg-surface px-2 py-1 text-sm"
            >
              {boards.length === 0 && <option value={0}>no git-linked boards</option>}
              {boards.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-xs text-fg-muted">Ref</span>
            <input
              value={ref}
              onChange={(e) => setRef(e.target.value)}
              placeholder={board?.base_branch || "branch, tag, or sha"}
              className="w-full rounded bg-surface px-2 py-1 text-sm text-fg placeholder:text-fg-muted/60"
            />
          </label>
        </div>
        <div className="flex items-center justify-between gap-4 border-t border-border px-4 py-2">
          {deployMut.error ? (
            <p className="min-w-0 truncate text-xs text-danger" title={String(deployMut.error)}>
              {String(deployMut.error)}
            </p>
          ) : (
            <p className="min-w-0 truncate text-xs text-fg-muted">
              {boards.length
                ? `Empty ref deploys ${board?.base_branch || "the base branch"}.`
                : "Link a git repo in board settings first."}
            </p>
          )}
          <Button
            type="submit"
            variant="primary"
            size="lg"
            disabled={!boards.length}
            pending={deployMut.isPending}
            idleLabel="deploy"
            pendingLabel="deploying…"
          />
        </div>
      </form>
    </Modal>
  );
}

/**
 * Where the orchestrator's disk goes, and the policy that bounds it. Every
 * commit previewed leaves artifacts, backend state, and logs behind, so
 * without a policy this only ever grows. Sizes are measured by walking the
 * data dir, so they're fetched on open rather than polled.
 */
function StorageModal({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient();
  const storageQ = useQuery({ queryKey: queryKeys.previewStorage, queryFn: api.previewStorage });
  const policyQ = useQuery({ queryKey: queryKeys.previewRetention, queryFn: api.previewRetention });

  // Empty means "unlimited" here, so the fields stay strings until saved —
  // a bare 0 in a number input is indistinguishable from a cleared one.
  const [deploys, setDeploys] = useState<string | null>(null);
  const [days, setDays] = useState<string | null>(null);
  const asField = (n: number) => (n > 0 ? String(n) : "");
  const deploysValue = deploys ?? asField(policyQ.data?.max_deploys_per_repo ?? 0);
  const daysValue = days ?? asField(policyQ.data?.max_age_days ?? 0);

  const saveMut = useMutation({
    mutationFn: () =>
      api.setPreviewRetention({
        max_deploys_per_repo: Math.max(0, Number(deploysValue) || 0),
        max_age_days: Math.max(0, Number(daysValue) || 0),
      }),
    onSuccess: (p) => {
      qc.setQueryData(queryKeys.previewRetention, p);
      setDeploys(null);
      setDays(null);
    },
  });
  const gcMut = useMutation({
    mutationFn: api.collectPreviewGarbage,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.previewStorage });
      qc.invalidateQueries({ queryKey: queryKeys.allPreviews });
    },
  });

  const s = storageQ.data;
  const categories = s
    ? [
        ["artifacts", s.artifacts_bytes],
        ["state", s.state_bytes],
        ["logs", s.logs_bytes],
        ["git mirrors", s.mirror_bytes],
        ["staging", s.tmp_bytes],
        ["database", s.db_bytes],
      ]
    : [];
  const repos = [...(s?.repos ?? [])].sort((a, b) => b.total_bytes - a.total_bytes);

  return (
    <Modal open wide onClose={onClose} title="Storage & retention" busy={saveMut.isPending}>
      <div className="flex flex-col gap-4 p-4">
        <section>
          <div className="mb-2 flex items-baseline justify-between gap-2">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted">
              Disk usage
            </h3>
            <span className="font-mono text-sm tabular-nums">
              {storageQ.isPending ? "…" : formatBytes(s?.total_bytes ?? 0)}
            </span>
          </div>
          {storageQ.isError ? (
            <p className="text-xs text-danger">{String(storageQ.error)}</p>
          ) : (
            <div className="grid grid-cols-2 gap-x-6 gap-y-1 sm:grid-cols-3">
              {categories.map(([label, bytes]) => (
                <div key={label} className="flex items-baseline justify-between gap-2 text-xs">
                  <span className="text-fg-muted">{label}</span>
                  <span className="font-mono tabular-nums">{formatBytes(Number(bytes))}</span>
                </div>
              ))}
            </div>
          )}
          {repos.length > 0 && (
            <ul className="mt-3 divide-y divide-border rounded border border-border">
              {repos.map((r) => (
                <li
                  key={r.repo}
                  className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 px-2 py-1.5 text-xs"
                >
                  <span className="min-w-0 flex-1 truncate font-medium">
                    {r.board_name ?? r.repo}
                  </span>
                  <span className="text-fg-muted">
                    {r.deploys} deploy{r.deploys === 1 ? "" : "s"}
                    {r.evicted_deploys > 0 && `, ${r.evicted_deploys} evicted`}
                  </span>
                  <span className="w-20 shrink-0 text-right font-mono tabular-nums">
                    {formatBytes(r.total_bytes)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="border-t border-border pt-4">
          <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-fg-muted">
            Retention
          </h3>
          <p className="mb-3 text-xs text-fg-muted">
            Sweeps run hourly. Evicting reclaims a deploy's build output but keeps its row —
            redeploying the same commit rebuilds it. A board's newest ready deploy is never evicted.
            Leave a field empty for no limit.
          </p>
          <form
            className="flex flex-wrap items-end gap-3"
            onSubmit={(e) => {
              e.preventDefault();
              saveMut.mutate();
            }}
          >
            <label className="flex flex-col gap-1">
              <span className="text-xs text-fg-muted">Deploys per board</span>
              <input
                type="number"
                min={0}
                value={deploysValue}
                onChange={(e) => setDeploys(e.target.value)}
                placeholder="unlimited"
                className="w-36 rounded bg-surface px-2 py-1 text-sm text-fg placeholder:text-fg-muted/60"
              />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-fg-muted">Max age (days)</span>
              <input
                type="number"
                min={0}
                value={daysValue}
                onChange={(e) => setDays(e.target.value)}
                placeholder="unlimited"
                className="w-36 rounded bg-surface px-2 py-1 text-sm text-fg placeholder:text-fg-muted/60"
              />
            </label>
            <Button
              type="submit"
              variant="primary"
              pending={saveMut.isPending}
              idleLabel="save"
              pendingLabel="saving…"
            />
          </form>
          {saveMut.error && <p className="mt-2 text-xs text-danger">{String(saveMut.error)}</p>}
        </section>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-2">
        <p className="min-w-0 flex-1 truncate text-xs text-fg-muted">
          {gcMut.error ? (
            <span className="text-danger">{String(gcMut.error)}</span>
          ) : gcMut.data ? (
            `Evicted ${gcMut.data.evicted.length}, freed ${formatBytes(gcMut.data.freed_bytes)}.`
          ) : (
            "Sweeping now also clears stale staging leftovers."
          )}
        </p>
        <Button
          type="button"
          onClick={() => gcMut.mutate()}
          pending={gcMut.isPending}
          idleLabel="sweep now"
          pendingLabel="sweeping…"
        />
      </div>
    </Modal>
  );
}

export function PreviewsDashboard() {
  const [deployOpen, setDeployOpen] = useState(false);
  const [storageOpen, setStorageOpen] = useState(false);
  const [logsId, setLogsId] = useState<number | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<DashboardPreview | null>(null);
  const [boardFilter, setBoardFilter] = useState<number | "all">("all");

  const qc = useQueryClient();
  const invalidate = () => qc.invalidateQueries({ queryKey: queryKeys.allPreviews });
  const stopMut = useMutation({ mutationFn: api.stopPreview, onSuccess: invalidate });
  const deleteMut = useMutation({
    mutationFn: api.deletePreview,
    onSuccess: () => {
      setDeleteTarget(null);
      invalidate();
    },
  });

  const boardsQ = useQuery({ queryKey: queryKeys.boards, queryFn: api.listBoards });
  const previewsQ = useQuery({
    queryKey: queryKeys.allPreviews,
    queryFn: api.listAllPreviews,
    // Poll fast while anything is in flight (a build, a cold start) so state
    // flips surface promptly; idle otherwise.
    refetchInterval: (query) =>
      query.state.data?.some(
        (p) =>
          p.status === "queued" ||
          p.status === "building" ||
          p.process === "starting" ||
          p.fe_process === "starting",
      )
        ? 1000
        : 5000,
  });

  const deployableBoards = (boardsQ.data ?? []).filter((b) => b.repo_path);
  const all = previewsQ.data ?? [];
  const previews = boardFilter === "all" ? all : all.filter((p) => p.board_id === boardFilter);
  // Resolve from the live list each render so the log modal follows a build
  // to completion instead of freezing on the snapshot that opened it.
  const logsPreview = logsId != null ? all.find((p) => p.id === logsId) : undefined;

  return (
    <div className="h-full overflow-y-auto [scrollbar-gutter:stable]">
      <div className="mx-auto w-full max-w-5xl px-3 py-4">
        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-baseline gap-2">
            <h2 className="text-sm font-semibold uppercase tracking-wide">Deployments</h2>
            {all.length > 0 && (
              <span className="text-xs tabular-nums text-fg-muted">{previews.length}</span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <select
              value={boardFilter}
              onChange={(e) =>
                setBoardFilter(e.target.value === "all" ? "all" : Number(e.target.value))
              }
              className="cursor-pointer rounded bg-surface px-2 py-1 text-sm"
              aria-label="Filter by board"
            >
              <option value="all">all boards</option>
              {(boardsQ.data ?? []).map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </select>
            <button
              type="button"
              onClick={() => setStorageOpen(true)}
              title="Storage & retention"
              aria-label="Storage and retention"
              className="rounded p-1.5 text-fg-muted transition-colors duration-150 hover:bg-surface-2 hover:text-fg"
            >
              <DatabaseIcon size={16} />
            </button>
            <Button variant="primary" size="lg" onClick={() => setDeployOpen(true)}>
              deploy
            </Button>
          </div>
        </div>

        {previewsQ.isError && (
          <p className="rounded border border-border bg-surface px-3 py-6 text-center text-sm text-fg-muted">
            Previews unavailable: {String(previewsQ.error)}
          </p>
        )}

        {!previewsQ.isError && (
          <div className="overflow-hidden rounded border border-border bg-surface">
            {previewsQ.isPending && (
              <div className="divide-y divide-border">
                {[0, 1, 2].map((i) => (
                  <div key={i} className="flex animate-pulse items-center gap-4 px-3 py-3">
                    <div className="h-5 w-[5.5rem] rounded-full bg-surface-2" />
                    <div className="h-4 w-40 rounded bg-surface-2" />
                    <div className="ml-auto h-4 w-24 rounded bg-surface-2" />
                  </div>
                ))}
              </div>
            )}
            {previewsQ.isSuccess && previews.length === 0 && (
              <div className="flex flex-col items-center gap-1 px-6 py-16 text-center">
                <RocketIcon size={32} className="mb-2 text-fg-muted/50" />
                <p className="text-sm font-medium">
                  {all.length === 0 ? "No deployments yet" : "No deployments for this board"}
                </p>
                <p className="max-w-md text-xs text-fg-muted">
                  A board's repo needs a preview manifest — a <code>preview.toml</code> at its root,
                  a <code>[previews]</code> table in its <code>.kanban.toml</code>, or a manifest
                  named for the board slug in the server's manifest directory.
                </p>
                <Button
                  variant="primary"
                  size="lg"
                  className="mt-3"
                  onClick={() => setDeployOpen(true)}
                >
                  deploy a commit
                </Button>
              </div>
            )}
            <ul className="divide-y divide-border">
              {previews.map((p) => {
                // Per-side content hashes ride along in the sha's tooltip
                // rather than taking a row of their own.
                const shaTitle = [
                  p.sha,
                  p.fe_hash ? `frontend ${p.fe_hash.slice(0, 12)}` : null,
                  p.be_hash ? `backend ${p.be_hash.slice(0, 12)}` : null,
                  ...(p.artifacts?.map((a) => `${a.name} ${a.hash.slice(0, 12)}`) ?? []),
                ]
                  .filter(Boolean)
                  .join("\n");
                const state = deployState(p);
                // Stopping only means something while something is up; a
                // stopped deploy stays listed and cold-starts on next request.
                const stoppable = state === "running" || state === "starting";
                return (
                  <li
                    key={p.id}
                    className="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-3 py-3 transition-colors hover:bg-surface-2/40"
                  >
                    <StatusBadge state={state} />
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                        <span className="text-sm font-medium">{p.board_name ?? p.repo}</span>
                        <code className="font-mono text-xs text-fg-muted" title={shaTitle}>
                          {p.short_sha}
                        </code>
                        {p.author_name && (
                          <span className="text-xs text-fg-muted" title={p.author_email}>
                            by {p.author_name}
                          </span>
                        )}
                      </div>
                      {(p.branch || (p.ref && p.ref !== p.branch)) && (
                        <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5">
                          {p.branch && (
                            <span className="inline-flex min-w-0 items-center gap-1 rounded-full border border-border bg-surface-2 px-2 py-px font-mono text-[11px] text-fg-muted">
                              <BranchIcon size={11} className="shrink-0" />
                              <span className="truncate">{p.branch}</span>
                            </span>
                          )}
                          {p.ref && p.ref !== p.branch && (
                            <span className="min-w-0 truncate rounded-full border border-border bg-surface-2 px-2 py-px font-mono text-[11px] text-fg-muted">
                              {p.ref}
                            </span>
                          )}
                        </div>
                      )}
                      {p.status === "failed" && p.error && (
                        <p
                          className="mt-0.5 max-w-full truncate text-xs text-danger"
                          title={p.error}
                        >
                          {p.error}
                        </p>
                      )}
                    </div>
                    {p.artifacts?.map((a) => (
                      <ArtifactDownload key={a.name} previewId={p.id} artifact={a} />
                    ))}
                    <button
                      type="button"
                      onClick={() => setLogsId(p.id)}
                      title="Build logs"
                      className={pillClass}
                    >
                      logs
                    </button>
                    {stoppable && (
                      <button
                        type="button"
                        onClick={() => stopMut.mutate(p.id)}
                        disabled={stopMut.isPending && stopMut.variables === p.id}
                        title="Stop this deploy's processes — it restarts on the next request"
                        className={`${pillClass} disabled:opacity-50`}
                      >
                        stop
                      </button>
                    )}
                    {p.preview_url ? (
                      <a
                        href={p.preview_url}
                        target="_blank"
                        rel="noreferrer"
                        className={pillClass}
                      >
                        open
                        <ExternalLinkIcon size={12} />
                      </a>
                    ) : (
                      <span className="w-14 text-center text-xs text-fg-muted/50">—</span>
                    )}
                    <time
                      dateTime={p.created_at}
                      title={p.created_at}
                      className="w-16 shrink-0 text-right text-xs tabular-nums text-fg-muted"
                    >
                      {timeAgo(p.created_at)}
                    </time>
                    <button
                      type="button"
                      onClick={() => setDeleteTarget(p)}
                      title="Delete this deploy"
                      aria-label={`Delete deploy ${p.short_sha}`}
                      className="shrink-0 rounded p-1 text-fg-muted transition-colors duration-150 hover:bg-surface-3 hover:text-danger"
                    >
                      <TrashIcon size={13} />
                    </button>
                  </li>
                );
              })}
            </ul>
          </div>
        )}
      </div>
      {deployOpen && <DeployModal boards={deployableBoards} onClose={() => setDeployOpen(false)} />}
      {storageOpen && <StorageModal onClose={() => setStorageOpen(false)} />}
      {logsPreview && <BuildLogModal preview={logsPreview} onClose={() => setLogsId(null)} />}
      <ConfirmModal
        open={deleteTarget != null}
        onClose={() => setDeleteTarget(null)}
        title="Delete deployment"
        description={`Delete ${deleteTarget?.board_name ?? deleteTarget?.repo} @ ${deleteTarget?.short_sha}?`}
        consequence="Its processes stop and any build output no other deploy shares is reclaimed. Deploying the same commit again rebuilds it."
        onConfirm={() => deleteTarget && deleteMut.mutate(deleteTarget.id)}
        confirmLabel="delete"
        confirmPendingLabel="deleting…"
        pending={deleteMut.isPending}
      />
    </div>
  );
}
