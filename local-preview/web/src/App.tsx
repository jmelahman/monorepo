import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useEffect, useRef, useState } from "react";
import {
  api,
  type Deploy,
  type DeployArtifact,
  type DeployStatus,
  type DeployStatusFilter,
  type GCResult,
  type LogSide,
  type ProcessState,
  type Repo,
  type SideStats,
  type StatsReport,
} from "@/api/client";

const THEME_KEY = "app.themeMode";
type Theme = "light" | "dark";

function storedTheme(): Theme | null {
  try {
    const raw = localStorage.getItem(THEME_KEY);
    return raw === "light" || raw === "dark" ? raw : null;
  } catch {
    return null;
  }
}

function systemTheme(): Theme {
  return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(() => storedTheme() ?? systemTheme());
  // Follow the OS appearance until the user explicitly picks a side; after
  // that the choice is persisted and system flips are ignored.
  const chosen = useRef(storedTheme() != null);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  useEffect(() => {
    const mq = matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      if (!chosen.current) setTheme(mq.matches ? "dark" : "light");
    };
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const other: Theme = theme === "dark" ? "light" : "dark";
  return (
    <button
      type="button"
      onClick={() => {
        chosen.current = true;
        setTheme(other);
        try {
          localStorage.setItem(THEME_KEY, other);
        } catch {
          // Private browsing; theme just won't persist.
        }
      }}
      title={`Switch to ${other} theme`}
      aria-label={`Switch to ${other} theme`}
      className="inline-flex h-7 w-7 items-center justify-center rounded bg-surface-2 text-fg transition-colors duration-150 hover:bg-surface-3"
    >
      {theme === "dark" ? <IconMoon /> : <IconSun />}
    </button>
  );
}

// UserMenu shows the signed-in GitHub identity and a sign-out button. It
// renders nothing when SSO is disabled (the viewer is anonymous), so an
// unauthenticated instance's header is unchanged. Shares the ["me"] query with
// AuthGate, so it reads from cache rather than refetching.
function UserMenu() {
  const queryClient = useQueryClient();
  const me = useQuery({ queryKey: ["me"], queryFn: api.me, retry: false });
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["me"] }),
  });
  if (!me.data || me.data.anonymous) return null;
  const label = me.data.login || me.data.email || "account";
  return (
    <div className="flex items-center gap-2">
      <span
        className="flex items-center gap-1.5 text-xs text-fg-muted"
        title={me.data.email || label}
      >
        {me.data.avatar_url ? (
          <img src={me.data.avatar_url} alt="" className="h-5 w-5 rounded-full" />
        ) : null}
        <span className="max-w-[10rem] truncate">{label}</span>
      </span>
      <button
        type="button"
        onClick={() => logout.mutate()}
        disabled={logout.isPending}
        title="Sign out"
        aria-label="Sign out"
        className="inline-flex h-7 items-center rounded bg-surface-2 px-2 text-xs text-fg transition-colors duration-150 hover:bg-surface-3 disabled:opacity-50"
      >
        Sign out
      </button>
    </div>
  );
}

// A deploy's displayed state: the build status until it's ready, then the
// merged runtime state of its supervised processes (backend and, for
// process-mode frontends, the frontend) — crashed if any side died, starting
// while any side warms up, running only once every side is warm, idle
// otherwise.
type DeployState = DeployStatus | ProcessState;

function deployState(d: Deploy): DeployState {
  if (d.status !== "ready") return d.status;
  const procs = [d.process, d.fe_process].filter((p): p is ProcessState => p != null);
  if (procs.length === 0) return d.status;
  // A dead side outranks a live one: half a deploy can't serve the preview.
  if (procs.includes("crashed")) return "crashed";
  if (procs.includes("starting")) return "starting";
  return procs.every((p) => p === "running") ? "running" : "idle";
}

// processError is the message behind a crashed deploy, naming the side that
// died — both can be crashed at once (a frontend outlives its backend only
// until its next start).
function processError(d: Deploy): string | null {
  const sides = [
    d.process === "crashed" ? `backend: ${d.process_error || "exited"}` : null,
    d.fe_process === "crashed" ? `frontend: ${d.fe_process_error || "exited"}` : null,
  ].filter((s): s is string => s != null);
  return sides.length > 0 ? sides.join(" · ") : null;
}

const stateStyles: Record<DeployState, { dot: string; pill: string; hint: string }> = {
  queued: {
    dot: "bg-fg-muted",
    pill: "border-border text-fg-muted",
    hint: "Waiting for a build slot",
  },
  building: {
    dot: "animate-pulse bg-warning",
    pill: "border-warning/30 text-warning",
    hint: "Build in progress",
  },
  ready: {
    dot: "bg-success",
    pill: "border-success/30 text-success",
    hint: "Static preview — served instantly",
  },
  idle: {
    dot: "bg-success/40",
    pill: "border-success/20 text-success/70",
    hint: "Built — starts on the first request",
  },
  starting: {
    dot: "animate-pulse bg-success",
    pill: "border-success/30 text-success",
    hint: "Starting up",
  },
  running: {
    dot: "bg-success",
    pill: "border-success/30 text-success",
    hint: "Warm — served instantly",
  },
  crashed: {
    dot: "bg-danger",
    pill: "border-danger/30 text-danger",
    hint: "Exited unexpectedly — check the run log; the next request starts it again",
  },
  failed: {
    dot: "bg-danger",
    pill: "border-danger/30 text-danger",
    hint: "Build failed",
  },
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
      className={`inline-flex w-22 items-center justify-center gap-1.5 rounded-full border py-0.5 text-xs font-medium ${s.pill}`}
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

// useDebounced trails value by ms, so per-keystroke consumers (the search
// query) fire once per pause instead of once per key.
function useDebounced<T>(value: T, ms: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return debounced;
}

const inputClass =
  "w-full rounded bg-surface px-2 py-1 text-sm text-fg placeholder:text-fg-muted/60";
const buttonClass =
  "inline-flex shrink-0 items-center rounded bg-accent-700 px-3 py-1 text-sm text-white transition-colors duration-150 hover:bg-accent-600 disabled:cursor-not-allowed disabled:opacity-50";
const neutralButtonClass =
  "inline-flex shrink-0 items-center rounded bg-surface-2 px-2 py-1 text-xs text-fg transition-colors duration-150 hover:bg-surface-3";
const accentButtonClass =
  "inline-flex shrink-0 items-center rounded bg-accent-700 px-2 py-1 text-xs text-white transition-colors duration-150 hover:bg-accent-600";
const dangerButtonClass =
  "inline-flex shrink-0 items-center rounded bg-accent-600 px-3 py-1 text-sm text-white transition-colors duration-150 hover:bg-accent-500 disabled:cursor-not-allowed disabled:opacity-50";

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: children slot holds the control at runtime.
    <label className="flex flex-col gap-1">
      <span className="text-xs text-fg-muted">{label}</span>
      {children}
    </label>
  );
}

function DialogFooter({
  error,
  hint,
  children,
}: {
  error?: unknown;
  hint: string;
  children: ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-t border-border px-4 py-2">
      {error ? (
        <p className="min-w-0 truncate text-xs text-danger" title={String(error)}>
          {String(error)}
        </p>
      ) : (
        <p className="min-w-0 truncate text-xs text-fg-muted">{hint}</p>
      )}
      {children}
    </div>
  );
}

function Modal({
  title,
  onClose,
  wide = false,
  children,
}: {
  title: string;
  onClose: () => void;
  wide?: boolean;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    // Guarded and without a close() cleanup: StrictMode re-runs the effect, and
    // closing here would fire the close event and unmount the dialog instantly.
    const dialog = ref.current;
    if (!dialog) return;
    if (!dialog.open) dialog.showModal();
    // showModal() runs the native dialog-focusing steps, which land on the first
    // focusable control (the header close button). Hand focus to an opt-in target
    // afterward so, e.g., Enter confirms a destructive action instead of cancelling.
    dialog.querySelector<HTMLElement>("[data-autofocus]")?.focus();
  }, []);
  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: backdrop click-to-close; Escape is handled natively by <dialog>.
    <dialog
      ref={ref}
      onClose={onClose}
      onClick={(e) => {
        if (e.target === ref.current) onClose();
      }}
      className={`m-auto ${wide ? "w-[780px]" : "w-[520px]"} max-w-[calc(100vw-2rem)] rounded border border-border bg-bg p-0 text-fg shadow-lg backdrop:bg-black/50`}
    >
      <header className="flex items-center justify-between border-b border-border px-3 py-2">
        <h2 className="text-sm font-semibold">{title}</h2>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close"
          className="inline-flex h-7 w-7 items-center justify-center rounded bg-surface-2 text-fg transition-colors duration-150 hover:bg-surface-3"
        >
          <IconX />
        </button>
      </header>
      {children}
    </dialog>
  );
}

// ConfirmDialog is a reusable modal for a destructive action: a body
// explaining the consequence and a danger-styled confirm button. The confirm
// button drives the caller's mutation; disable interaction while it's in
// flight and surface its error inline.
function ConfirmDialog({
  title,
  confirmLabel,
  onConfirm,
  onClose,
  pending,
  error,
  children,
}: {
  title: string;
  confirmLabel: string;
  onConfirm: () => void;
  onClose: () => void;
  pending: boolean;
  error?: unknown;
  children: ReactNode;
}) {
  return (
    <Modal title={title} onClose={() => !pending && onClose()}>
      <div className="flex flex-col gap-2 p-4 text-sm">{children}</div>
      <div className="flex items-center justify-end gap-2 border-t border-border px-4 py-2">
        {error ? (
          <p className="mr-auto min-w-0 truncate text-xs text-danger" title={String(error)}>
            {String(error)}
          </p>
        ) : null}
        <button
          type="button"
          onClick={onClose}
          disabled={pending}
          className={`${neutralButtonClass} px-3 py-1 text-sm`}
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={onConfirm}
          disabled={pending}
          data-autofocus
          className={dangerButtonClass}
        >
          {confirmLabel}
        </button>
      </div>
    </Modal>
  );
}

function RegisterRepoDialog({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [source, setSource] = useState("");
  // Once the POST is accepted the server clones in the background; the
  // dialog tracks that repo until it turns ready (auto-close) or failed.
  const [registered, setRegistered] = useState<string | null>(null);
  const createRepo = useMutation({
    mutationFn: () => api.createRepo(name.trim(), source.trim()),
    onSuccess: (repo) => {
      queryClient.invalidateQueries({ queryKey: ["repos"] });
      // Seed the tracking query with the accepted row: a stale "failed"
      // entry from an earlier attempt at this name would otherwise stick
      // (its interval callback sees a terminal status and stops polling).
      queryClient.setQueryData(["repo", repo.name], repo);
      setRegistered(repo.name);
    },
  });
  const cloning = useQuery({
    queryKey: ["repo", registered],
    queryFn: () => api.getRepo(registered as string),
    enabled: registered != null,
    refetchInterval: (query) => (query.state.data?.status === "cloning" ? 750 : false),
  });
  // Deletes the failed row so the same name can be resubmitted from the
  // form, which keeps its values.
  const discard = useMutation({
    mutationFn: () => api.deleteRepo(registered as string),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ["repo", registered] });
      queryClient.invalidateQueries({ queryKey: ["repos"] });
      setRegistered(null);
      createRepo.reset();
    },
  });

  const status = cloning.data?.status;
  useEffect(() => {
    if (status === "ready") {
      queryClient.invalidateQueries({ queryKey: ["repos"] });
      onClose();
    }
  }, [status, queryClient, onClose]);

  if (registered != null) {
    return (
      <Modal title="Register a repository" onClose={onClose}>
        <div className="flex flex-col gap-3 p-4 text-sm">
          {status === "failed" ? (
            <>
              <p>
                Cloning <code className="font-mono text-xs">{source.trim()}</code> failed.
              </p>
              {cloning.data?.error && (
                <p className="break-words font-mono text-xs text-danger">{cloning.data.error}</p>
              )}
            </>
          ) : (
            <>
              <div className="flex items-center gap-2.5">
                <span className="h-4 w-4 shrink-0 animate-spin rounded-full border-2 border-fg-muted border-t-transparent" />
                <p className="min-w-0">
                  Cloning <code className="break-all font-mono text-xs">{source.trim()}</code>…
                </p>
              </div>
              {cloning.data?.progress && (
                <p className="font-mono text-xs text-fg-muted">{cloning.data.progress}</p>
              )}
              <p className="text-xs text-fg-muted">
                Large repositories can take a while. Closing this dialog won't interrupt the clone —
                the repository becomes deployable once it finishes.
              </p>
            </>
          )}
        </div>
        <DialogFooter
          error={discard.error}
          hint={
            status === "failed"
              ? "Fix the source and try again, or close to keep the failed entry."
              : ""
          }
        >
          {status === "failed" && (
            <button
              type="button"
              onClick={() => discard.mutate()}
              disabled={discard.isPending}
              className={buttonClass}
            >
              Edit &amp; retry
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            className={`${neutralButtonClass} px-3 py-1 text-sm`}
          >
            Close
          </button>
        </DialogFooter>
      </Modal>
    );
  }

  return (
    <Modal title="Register a repository" onClose={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim() && source.trim()) createRepo.mutate();
        }}
      >
        <div className="flex flex-col gap-3 p-4 text-sm">
          <p className="text-xs text-fg-muted">
            Point at a local path or clone URL to start deploying from it.
          </p>
          <Field label="Name">
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="my-app"
              className={inputClass}
            />
          </Field>
          <Field label="Source">
            <input
              value={source}
              onChange={(e) => setSource(e.target.value)}
              placeholder="/path/to/repo or clone URL"
              className={inputClass}
            />
          </Field>
        </div>
        <DialogFooter
          error={createRepo.error}
          hint="The name becomes the preview subdomain (DNS label)."
        >
          <button
            type="submit"
            disabled={!name.trim() || !source.trim() || createRepo.isPending}
            className={buttonClass}
          >
            Register
          </button>
        </DialogFooter>
      </form>
    </Modal>
  );
}

function DeployDialog({
  repos,
  previewDomain,
  onClose,
}: {
  repos: Repo[];
  // Absent until /api/health lands; the hint drops the domain until then
  // rather than guessing the default, which --preview-domain can override.
  previewDomain?: string;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  // Only ready repos are deployable; ones still cloning stay visible but
  // disabled so a fresh registration is discoverable here.
  const [repo, setRepo] = useState(repos.find((r) => r.status === "ready")?.name ?? "");
  const [gitRef, setGitRef] = useState("");
  // Once the POST is accepted the server builds in the background; the
  // dialog tracks that deploy until it turns ready or failed rather than
  // closing on an accepted request that has produced nothing yet.
  const [deployId, setDeployId] = useState<number | null>(null);
  const chosen = repos.find((r) => r.name === repo);
  const createDeploy = useMutation({
    mutationFn: () => api.createDeploy(repo, gitRef.trim()),
    onSuccess: (deploy) => {
      queryClient.invalidateQueries({ queryKey: ["deploys"] });
      queryClient.setQueryData(["deploy", deploy.id], deploy);
      setDeployId(deploy.id);
    },
  });
  const tracked = useQuery({
    queryKey: ["deploy", deployId],
    queryFn: () => api.getDeploy(deployId as number),
    enabled: deployId != null,
    refetchInterval: (query) => {
      const s = query.state.data?.status;
      return s === "queued" || s === "building" ? 750 : false;
    },
  });

  const deploy = tracked.data;
  const status = deploy?.status;
  // Each transition (queued → building → ready/failed) is worth a list
  // refresh: the row behind the dialog tracks the same build.
  useEffect(() => {
    if (status) queryClient.invalidateQueries({ queryKey: ["deploys"] });
  }, [status, queryClient]);

  if (deployId != null) {
    const building = status === "queued" || status === "building";
    const failed = status === "failed";
    return (
      <Modal
        title={failed ? "Deploy failed" : status === "ready" ? "Deploy ready" : "Deploying"}
        onClose={onClose}
      >
        <div className="flex flex-col gap-3 p-4 text-sm">
          <div className="flex items-center gap-2.5">
            {failed ? (
              <span className="shrink-0 text-danger">
                <IconX />
              </span>
            ) : status === "ready" ? (
              <IconCheck className="h-4 w-4 shrink-0 text-success" />
            ) : (
              <span className="h-4 w-4 shrink-0 animate-spin rounded-full border-2 border-fg-muted border-t-transparent" />
            )}
            <p className="min-w-0">
              {failed
                ? "Build failed."
                : status === "ready"
                  ? "Deploy finished."
                  : status === "building"
                    ? "Building frontend and backend…"
                    : "Queued for a build slot…"}{" "}
              <span className="text-fg-muted">
                {deploy?.repo ?? repo}
                {deploy?.short_sha && (
                  <>
                    {" "}
                    <code className="font-mono text-xs">{deploy.short_sha}</code>
                  </>
                )}
              </span>
            </p>
          </div>
          <DeployStep label="Commit resolved" state={deploy?.sha ? "done" : "active"} />
          <DeployStep
            label="Building content-addressed artifacts"
            state={failed ? "failed" : status === "ready" ? "done" : "active"}
          />
          <DeployStep
            label="Preview serving"
            state={status === "ready" ? "done" : failed ? "skipped" : "pending"}
          />
          {failed && deploy?.error && (
            <p className="break-words font-mono text-xs text-danger">{deploy.error}</p>
          )}
          <BuildLogPane deployId={deployId} building={building} />
        </div>
        <DialogFooter
          hint={
            building
              ? "Closing this dialog won't interrupt the build."
              : status === "ready"
                ? (deploy?.preview_url ?? "")
                : "Fix the ref or the repository's preview.toml and try again."
          }
        >
          {failed && (
            <button
              type="button"
              onClick={() => {
                setDeployId(null);
                createDeploy.reset();
              }}
              className={buttonClass}
            >
              Back
            </button>
          )}
          {status === "ready" && deploy?.preview_url && (
            <a
              href={deploy.preview_url}
              target="_blank"
              rel="noreferrer"
              className={`${accentButtonClass} gap-1 px-3 py-1 text-sm`}
            >
              Open preview
              <IconArrowUpRight className="h-3 w-3" />
            </a>
          )}
          <button
            type="button"
            onClick={onClose}
            className={`${neutralButtonClass} px-3 py-1 text-sm`}
          >
            Close
          </button>
        </DialogFooter>
      </Modal>
    );
  }

  return (
    <Modal title="Deploy a commit" onClose={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (gitRef.trim() && repo) createDeploy.mutate();
        }}
      >
        <div className="flex flex-col gap-3 p-4 text-sm">
          <p className="text-xs text-fg-muted">
            Builds are content-addressed; unchanged halves are reused.
          </p>
          <Field label="Repository">
            <select
              value={repo}
              onChange={(e) => setRepo(e.target.value)}
              className="w-full cursor-pointer rounded bg-surface px-2 py-1 text-sm"
            >
              {!repos.some((r) => r.status === "ready") && (
                <option value="">
                  {repos.length === 0 ? "no repositories yet" : "no ready repositories yet"}
                </option>
              )}
              {repos.map((r) => (
                <option key={r.id} value={r.name} disabled={r.status !== "ready"}>
                  {r.name}
                  {r.status === "cloning" && " (cloning…)"}
                  {r.status === "failed" && " (clone failed)"}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Ref">
            <input
              value={gitRef}
              onChange={(e) => setGitRef(e.target.value)}
              placeholder="sha or branch"
              className={inputClass}
            />
          </Field>
        </div>
        <DialogFooter
          error={createDeploy.error}
          hint={
            !repos.length
              ? "Register a repository first."
              : !repos.some((r) => r.status === "ready")
                ? "Repositories are still cloning — deploy once one is ready."
                : previewDomain
                  ? `Served at <sha>-<repo>.${previewDomain}.`
                  : "Served at <sha>-<repo> on the preview domain."
          }
        >
          <button
            type="submit"
            disabled={!gitRef.trim() || chosen?.status !== "ready" || createDeploy.isPending}
            className={`${buttonClass} gap-2`}
          >
            {createDeploy.isPending && (
              <span className="h-3 w-3 shrink-0 animate-spin rounded-full border-2 border-white/70 border-t-transparent" />
            )}
            {createDeploy.isPending ? "Submitting…" : "Deploy"}
          </button>
        </DialogFooter>
      </form>
    </Modal>
  );
}

// DeployStep is one line of the deploy progress checklist: a leading marker
// (done / running / not yet / never ran) plus its label.
function DeployStep({
  label,
  state,
}: {
  label: string;
  state: "pending" | "active" | "done" | "failed" | "skipped";
}) {
  return (
    <div
      className={`flex items-center gap-2.5 text-xs ${state === "pending" || state === "skipped" ? "text-fg-muted/60" : "text-fg-muted"}`}
    >
      <span className="flex h-4 w-4 shrink-0 items-center justify-center">
        {state === "done" ? (
          <IconCheck className="h-3.5 w-3.5 text-success" />
        ) : state === "failed" ? (
          <span className="text-danger">
            <IconX />
          </span>
        ) : state === "active" ? (
          <span className="h-3 w-3 animate-spin rounded-full border-2 border-fg-muted border-t-transparent" />
        ) : (
          <span className="h-1.5 w-1.5 rounded-full bg-fg-muted/40" />
        )}
      </span>
      {label}
    </div>
  );
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

function uptime(iso: string): string {
  const secs = Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 1000));
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ${secs % 60}s`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ${mins % 60}m`;
  return `${Math.floor(hours / 24)}d ${hours % 24}h`;
}

// usePinnedScroll keeps a log pane glued to its bottom edge as content
// streams in, unless the user scrolled up to read history.
function usePinnedScroll(dep: unknown) {
  const ref = useRef<HTMLPreElement>(null);
  const pinned = useRef(true);
  // biome-ignore lint/correctness/useExhaustiveDependencies: dep drives re-scroll on content change.
  useEffect(() => {
    const el = ref.current;
    if (el && pinned.current) el.scrollTop = el.scrollHeight;
  }, [dep]);
  const onScroll = () => {
    const el = ref.current;
    if (el) pinned.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  };
  return { ref, onScroll };
}

const logPaneClass =
  "h-72 overflow-auto whitespace-pre-wrap break-words rounded border border-border bg-surface p-2 font-mono text-xs leading-relaxed";

// CopyLogsButton floats over a log pane's top-right corner. It sits outside
// the scrolling <pre> so it stays put as output streams past, and confirms in
// place for a moment — the clipboard gives no other feedback.
function CopyLogsButton({ text }: { text: string }) {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");
  const timer = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(timer.current), []);

  const copy = async () => {
    let next: "copied" | "failed";
    try {
      await navigator.clipboard.writeText(text);
      next = "copied";
    } catch {
      // Blocked clipboard (denied permission, insecure origin) — say so
      // rather than looking like a no-op.
      next = "failed";
    }
    setState(next);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setState("idle"), 1500);
  };

  const label = state === "copied" ? "Copied" : state === "failed" ? "Copy failed" : "Copy";
  // Spelled out rather than composed from neutralButtonClass: the text color
  // varies by state, and Tailwind can't be trusted to resolve a
  // same-property override by class order.
  return (
    <button
      type="button"
      onClick={copy}
      disabled={!text}
      title={text ? "Copy these logs to the clipboard" : "No output to copy"}
      className={`absolute right-3 top-2 inline-flex shrink-0 items-center gap-1 rounded border border-border bg-surface-2 px-2 py-1 text-xs transition-colors duration-150 hover:bg-surface-3 disabled:cursor-not-allowed disabled:opacity-50 ${
        state === "failed" ? "text-danger" : "text-fg"
      }`}
    >
      {state === "copied" ? <IconCheck className="h-3 w-3" /> : <IconCopy className="h-3 w-3" />}
      {label}
    </button>
  );
}

function Metric({ label, value, title }: { label: string; value: string; title?: string }) {
  return (
    <span className="inline-flex items-baseline gap-1 tabular-nums" title={title}>
      <span className="text-[10px] uppercase tracking-wide text-fg-muted">{label}</span>
      {value}
    </span>
  );
}

// StatsRow is one side's docker-stats-like line: state, CPU, memory, uptime.
function StatsRow({ label, stats }: { label: string; stats: SideStats }) {
  const mem =
    stats.memory_bytes != null
      ? `${formatBytes(stats.memory_bytes)}${
          stats.memory_limit_bytes ? ` / ${formatBytes(stats.memory_limit_bytes)}` : ""
        }`
      : "—";
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-1.5 text-xs">
      <span className="w-16 font-medium">{label}</span>
      <StatusBadge state={stats.state} />
      <Metric
        label="cpu"
        value={stats.cpu_percent != null ? `${stats.cpu_percent.toFixed(1)}%` : "—"}
        title="Percent of one CPU core, like docker stats"
      />
      <Metric label="mem" value={mem} title="Resident memory / total available" />
      <Metric label="up" value={stats.started_at ? uptime(stats.started_at) : "—"} />
      {stats.runtime && <span className="ml-auto text-fg-muted">{stats.runtime}</span>}
      {stats.error && (
        <span className="w-full truncate text-danger" title={stats.error}>
          {stats.error}
        </span>
      )}
    </div>
  );
}

// RunLogPane tails a process run log: an initial tail, then only appended
// bytes each poll. A restart (new attempt) resets the view.
function RunLogPane({ deployId, side }: { deployId: number; side: LogSide }) {
  const [text, setText] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [truncated, setTruncated] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const cursor = useRef({ attempt: 0, offset: 0 });
  const { ref, onScroll } = usePinnedScroll(text);

  useEffect(() => {
    setText("");
    setAttempt(0);
    setTruncated(false);
    cursor.current = { attempt: 0, offset: 0 };
    let stopped = false;
    let inFlight = false;
    const tick = async () => {
      if (stopped || inFlight) return;
      inFlight = true;
      try {
        const c = await api.getRunLog(
          deployId,
          side,
          cursor.current.attempt,
          cursor.current.offset,
        );
        if (stopped) return;
        setError(null);
        if (c.attempt !== cursor.current.attempt) {
          // First fetch, or the process restarted into a fresh log file.
          setText(c.content);
          setTruncated(c.truncated ?? false);
          setAttempt(c.attempt);
        } else if (c.content) {
          setText((t) => t + c.content);
        }
        cursor.current = { attempt: c.attempt, offset: c.offset };
      } catch (e) {
        if (!stopped) setError(String(e));
      } finally {
        inFlight = false;
      }
    };
    tick();
    const interval = setInterval(tick, 1000);
    return () => {
      stopped = true;
      clearInterval(interval);
    };
  }, [deployId, side]);

  return (
    <div className="flex flex-col gap-1">
      <div className="relative">
        <pre ref={ref} onScroll={onScroll} className={logPaneClass}>
          {truncated && <span className="text-fg-muted">{"… earlier output omitted\n"}</span>}
          {text ||
            (attempt === 0 ? (
              <span className="text-fg-muted">
                No output yet — the process starts on the preview's first request.
              </span>
            ) : (
              <span className="text-fg-muted">The process hasn't written any output.</span>
            ))}
        </pre>
        <CopyLogsButton text={text} />
      </div>
      <div className="flex justify-between font-mono text-[11px] text-fg-muted">
        <span>{error ? `log fetch failed: ${error}` : "stdout+stderr, refreshed live"}</span>
        {attempt > 0 && <span>start attempt {attempt}</span>}
      </div>
    </div>
  );
}

// BuildLogPane shows the build-log snapshot, refreshing while a build runs.
function BuildLogPane({ deployId, building }: { deployId: number; building: boolean }) {
  const logs = useQuery({
    queryKey: ["deploy-build-log", deployId],
    queryFn: () => api.getBuildLogs(deployId),
    refetchInterval: building ? 1000 : false,
  });
  const { ref, onScroll } = usePinnedScroll(logs.data);
  return (
    <div className="flex flex-col gap-1">
      <div className="relative">
        <pre ref={ref} onScroll={onScroll} className={logPaneClass}>
          {logs.error ? String(logs.error) : (logs.data ?? "loading…")}
        </pre>
        <CopyLogsButton text={logs.error ? "" : (logs.data ?? "")} />
      </div>
      <div className="font-mono text-[11px] text-fg-muted">
        {building ? "build in progress — refreshing" : "frontend and backend build output"}
      </div>
    </div>
  );
}

// ArtifactDownload is one artifact's download affordance: a plain link when
// the build produced a single file, a dropdown listing every file (e.g. one
// binary per platform) when it produced several. Artifacts build after the
// deploy turns ready, so the chip may first show the build still running (or
// failed) before the download appears. The menu is fixed-positioned so the
// list container's overflow-hidden can't clip it.
function ArtifactDownload({ artifact }: { artifact: DeployArtifact }) {
  const [menuPos, setMenuPos] = useState<{ top: number; right: number } | null>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!menuPos) return;
    const close = () => setMenuPos(null);
    const onPointerDown = (e: PointerEvent) => {
      if (!buttonRef.current?.parentElement?.contains(e.target as Node)) close();
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    window.addEventListener("scroll", close, true);
    window.addEventListener("resize", close);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("resize", close);
    };
  }, [menuPos]);

  if (artifact.status === "building") {
    return (
      <span
        title={`Building ${artifact.name}…`}
        className="inline-flex shrink-0 animate-pulse items-center gap-1 rounded bg-surface-2 px-2 py-1 font-mono text-xs text-fg-muted"
      >
        <IconDownload className="h-3 w-3" />
        {artifact.name}
      </span>
    );
  }
  if (artifact.status === "failed") {
    return (
      <span
        title={`${artifact.name} build failed${artifact.error ? `: ${artifact.error}` : ""}`}
        className="inline-flex shrink-0 items-center gap-1 rounded bg-surface-2 px-2 py-1 font-mono text-xs text-danger"
      >
        <IconDownload className="h-3 w-3" />
        {artifact.name}
      </span>
    );
  }

  if (artifact.files.length === 1) {
    const f = artifact.files[0];
    return (
      <a
        href={f.url}
        download={f.name}
        title={`${f.name} (${formatBytes(f.size)})`}
        className={`${neutralButtonClass} gap-1 font-mono`}
      >
        <IconDownload className="h-3 w-3" />
        {artifact.name}
      </a>
    );
  }

  return (
    <div className="relative">
      <button
        ref={buttonRef}
        type="button"
        aria-haspopup="menu"
        aria-expanded={menuPos != null}
        title={`Download ${artifact.name} for your platform`}
        onClick={() => {
          if (menuPos) {
            setMenuPos(null);
            return;
          }
          const r = buttonRef.current?.getBoundingClientRect();
          if (r) setMenuPos({ top: r.bottom + 4, right: window.innerWidth - r.right });
        }}
        className={`${neutralButtonClass} gap-1 font-mono`}
      >
        <IconDownload className="h-3 w-3" />
        {artifact.name}
        <IconChevronDown className="h-3 w-3" />
      </button>
      {menuPos && (
        <div
          role="menu"
          style={{ top: menuPos.top, right: menuPos.right }}
          className="fixed z-10 min-w-48 rounded border border-border bg-bg py-1 shadow-lg"
        >
          {artifact.files.map((f) => (
            <a
              key={f.name}
              role="menuitem"
              href={f.url}
              download={f.name}
              onClick={() => setMenuPos(null)}
              className="flex items-baseline justify-between gap-4 px-3 py-1.5 font-mono text-xs text-fg transition-colors duration-150 hover:bg-surface-2"
            >
              {f.name}
              <span className="text-[10px] text-fg-muted tabular-nums">{formatBytes(f.size)}</span>
            </a>
          ))}
        </div>
      )}
    </div>
  );
}

type LogTab = LogSide | "build";

// DeployDetailDialog is the observability view of one deployment: live
// resource stats plus docker-logs-like views of its process and build logs.
function DeployDetailDialog({ deploy, onClose }: { deploy: Deploy; onClose: () => void }) {
  // Artifact builds run on after the deploy turns ready and append to the
  // same build-log snapshot, so they keep the pane refreshing too.
  const building =
    deploy.status === "queued" ||
    deploy.status === "building" ||
    (deploy.artifacts?.some((a) => a.status === "building") ?? false);
  const hasBackend = !!deploy.be_hash;
  const hasFeProcess = deploy.fe_process != null;
  const [tab, setTab] = useState<LogTab>(hasBackend && !building ? "be" : "build");

  const stats = useQuery({
    queryKey: ["deploy-stats", deploy.id],
    queryFn: () => api.getDeployStats(deploy.id),
    // Two samples make a CPU percentage, so keep a steady cadence.
    refetchInterval: 2000,
  });

  const tabs: { id: LogTab; label: string }[] = [
    ...(hasBackend ? [{ id: "be" as const, label: "backend log" }] : []),
    ...(hasFeProcess ? [{ id: "fe" as const, label: "frontend log" }] : []),
    { id: "build", label: "build log" },
  ];

  return (
    <Modal title={`${deploy.repo} @ ${deploy.short_sha}`} onClose={onClose} wide>
      <div className="flex flex-col gap-3 p-4">
        {(stats.data?.backend || stats.data?.frontend) && (
          <div className="divide-y divide-border rounded border border-border">
            {stats.data.backend && <StatsRow label="backend" stats={stats.data.backend} />}
            {stats.data.frontend && <StatsRow label="frontend" stats={stats.data.frontend} />}
          </div>
        )}
        <div className="flex gap-1">
          {tabs.map((t) => (
            <button
              key={t.id}
              type="button"
              onClick={() => setTab(t.id)}
              className={`rounded px-2 py-1 text-xs transition-colors duration-150 ${
                tab === t.id
                  ? "bg-surface-3 font-medium text-fg"
                  : "bg-surface-2 text-fg-muted hover:bg-surface-3"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
        {tab === "build" ? (
          <BuildLogPane deployId={deploy.id} building={building} />
        ) : (
          <RunLogPane deployId={deploy.id} side={tab} />
        )}
      </div>
    </Modal>
  );
}

// StopButton quiesces a deploy's running processes; they cold-start again on
// the next request, so it needs no confirmation. Rendered only when there's a
// live process to stop.
// StopButton stops a deploy's processes. On a crashed deploy there's nothing
// left to stop — the same request acknowledges the crash instead, so the
// button says what it does.
function StopButton({ deploy, crashed }: { deploy: Deploy; crashed?: boolean }) {
  const queryClient = useQueryClient();
  const stop = useMutation({
    mutationFn: () => api.stopDeploy(deploy.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["deploys"] }),
  });
  return (
    <button
      type="button"
      onClick={() => stop.mutate()}
      disabled={stop.isPending}
      title={
        crashed
          ? "Clear the crash (starts again on the next request)"
          : "Stop the running process (restarts on the next request)"
      }
      className={`${neutralButtonClass} gap-1 disabled:opacity-50`}
    >
      <IconStop className="h-3 w-3" />
      {crashed ? "dismiss" : "stop"}
    </button>
  );
}

// RedeployButton revives an evicted deploy: POSTing its own commit to
// /api/deploys re-queues the build, rebuilding the reclaimed artifacts. Kept
// inline (no confirmation) because it only spends a build. Failure surfaces
// on the row — a commit pruned from the repo can't be revived.
function RedeployButton({ deploy }: { deploy: Deploy }) {
  const queryClient = useQueryClient();
  const redeploy = useMutation({
    mutationFn: () => api.createDeploy(deploy.repo, deploy.sha),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["deploys"] }),
  });
  return (
    <>
      <button
        type="button"
        onClick={() => redeploy.mutate()}
        disabled={redeploy.isPending}
        title="Rebuild this commit and revive its preview"
        className={`${neutralButtonClass} gap-1 disabled:opacity-50`}
      >
        <IconRefresh className="h-3 w-3" />
        redeploy
      </button>
      {redeploy.error != null && (
        <span className="max-w-40 truncate text-xs text-danger" title={String(redeploy.error)}>
          {String(redeploy.error)}
        </span>
      )}
    </>
  );
}

// DeleteDeployButton hard-deletes a deploy behind a confirmation: the row and
// its unshared artifacts/state are removed and the subdomain freed.
function DeleteDeployButton({ deploy }: { deploy: Deploy }) {
  const queryClient = useQueryClient();
  const [confirming, setConfirming] = useState(false);
  const del = useMutation({
    mutationFn: () => api.deleteDeploy(deploy.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["deploys"] });
      setConfirming(false);
    },
  });
  return (
    <>
      <button
        type="button"
        onClick={() => setConfirming(true)}
        title="Delete this deployment"
        aria-label="Delete this deployment"
        className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded bg-surface-2 text-fg-muted transition-colors duration-150 hover:bg-danger/15 hover:text-danger"
      >
        <IconTrash className="h-3.5 w-3.5" />
      </button>
      {confirming && (
        <ConfirmDialog
          title="Delete deployment"
          confirmLabel={del.isPending ? "Deleting…" : "Delete"}
          onConfirm={() => del.mutate()}
          onClose={() => setConfirming(false)}
          pending={del.isPending}
          error={del.error}
        >
          <p>
            Delete the <span className="font-medium">{deploy.repo}</span> preview at{" "}
            <code className="font-mono text-fg-muted">{deploy.short_sha}</code>?
          </p>
          <p className="text-xs text-fg-muted">
            Its build artifacts and state are removed and the{" "}
            <code className="font-mono">{deploy.short_sha}</code> subdomain is freed. Artifacts
            still shared with another deployment are kept.
          </p>
        </ConfirmDialog>
      )}
    </>
  );
}

// RepoCloneBadge marks a repo whose mirror clone hasn't completed: pulsing
// while it runs (live progress in the hover), red once it failed.
function RepoCloneBadge({ repo }: { repo: Repo }) {
  if (repo.status === "cloning") {
    return (
      <span
        title={repo.progress || "Mirror clone in progress"}
        className="inline-flex shrink-0 items-center gap-1.5 rounded-full border border-warning/30 px-2 py-0.5 text-xs font-medium text-warning"
      >
        <span className="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-warning" />
        cloning
      </span>
    );
  }
  if (repo.status === "failed") {
    return (
      <span
        title={repo.error}
        className="inline-flex shrink-0 items-center gap-1.5 rounded-full border border-danger/30 px-2 py-0.5 text-xs font-medium text-danger"
      >
        <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-danger" />
        clone failed
      </span>
    );
  }
  return null;
}

// RepoRow is one registered repository in the management modal: its name and
// source, an editable watch toggle + branch globs, and an unregister action.
function RepoRow({ repo }: { repo: Repo }) {
  const queryClient = useQueryClient();
  const [watch, setWatch] = useState(repo.watch);
  const [branches, setBranches] = useState(repo.watch_branches);
  const [confirming, setConfirming] = useState(false);

  // Track the server's value when it changes under us (e.g. after a save
  // refetch) so the dirty check and inputs stay accurate.
  useEffect(() => {
    setWatch(repo.watch);
    setBranches(repo.watch_branches);
  }, [repo.watch, repo.watch_branches]);

  const save = useMutation({
    mutationFn: () => api.updateRepo(repo.name, { watch, watch_branches: branches.trim() }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["repos"] }),
  });
  const del = useMutation({
    mutationFn: () => api.deleteRepo(repo.name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["repos"] });
      queryClient.invalidateQueries({ queryKey: ["deploys"] });
      setConfirming(false);
    },
  });

  const dirty = watch !== repo.watch || branches.trim() !== repo.watch_branches;
  return (
    <li className="flex flex-col gap-2 px-3 py-2.5">
      <div className="flex items-center gap-2">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <div className="text-sm font-medium">{repo.name}</div>
            <RepoCloneBadge repo={repo} />
          </div>
          <div className="truncate font-mono text-[11px] text-fg-muted" title={repo.source}>
            {repo.source}
          </div>
          {repo.status === "cloning" && repo.progress && (
            <div className="truncate font-mono text-[11px] text-fg-muted">{repo.progress}</div>
          )}
        </div>
        <button
          type="button"
          onClick={() => setConfirming(true)}
          className="inline-flex shrink-0 items-center rounded bg-surface-2 px-2 py-1 text-xs text-fg-muted transition-colors duration-150 hover:bg-danger/15 hover:text-danger"
        >
          Unregister
        </button>
      </div>
      {repo.status === "failed" && repo.error && (
        <p className="text-xs text-danger" title={repo.error}>
          {repo.error}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <label className="flex shrink-0 cursor-pointer items-center gap-1.5 text-xs text-fg-muted">
          <input
            type="checkbox"
            checked={watch}
            onChange={(e) => setWatch(e.target.checked)}
            className="accent-accent-600"
          />
          Watch
        </label>
        <input
          value={branches}
          onChange={(e) => setBranches(e.target.value)}
          placeholder="all branches (e.g. main,release/* or !main)"
          className={`${inputClass} min-w-40 flex-1`}
        />
        {dirty && (
          <button
            type="button"
            onClick={() => save.mutate()}
            disabled={save.isPending}
            className={`${neutralButtonClass} px-3 py-1 text-sm`}
          >
            Save
          </button>
        )}
      </div>
      {save.error && (
        <p className="text-xs text-danger" title={String(save.error)}>
          {String(save.error)}
        </p>
      )}
      {confirming && (
        <ConfirmDialog
          title="Unregister repository"
          confirmLabel={del.isPending ? "Removing…" : "Unregister"}
          onConfirm={() => del.mutate()}
          onClose={() => setConfirming(false)}
          pending={del.isPending}
          error={del.error}
        >
          <p>
            Unregister <span className="font-medium">{repo.name}</span>?
          </p>
          <p className="text-xs text-fg-muted">
            This stops its preview backends and deletes all of its deployments, artifacts, state,
            build logs, and mirror clone. The name becomes reusable immediately.
          </p>
        </ConfirmDialog>
      )}
    </li>
  );
}

// ManageReposDialog lists registered repositories and lets you edit their
// watch settings or unregister them — the repo-side counterpart to the deploy
// list.
function ManageReposDialog({ onClose }: { onClose: () => void }) {
  const repos = useQuery({ queryKey: ["repos"], queryFn: api.listRepos });
  return (
    <Modal title="Registered repositories" onClose={onClose} wide>
      <div className="flex flex-col gap-3 p-4 text-sm">
        {repos.data && repos.data.length > 0 ? (
          <ul className="divide-y divide-border rounded border border-border">
            {repos.data.map((r) => (
              <RepoRow key={r.id} repo={r} />
            ))}
          </ul>
        ) : (
          <p className="px-1 py-6 text-center text-xs text-fg-muted">
            No repositories registered yet.
          </p>
        )}
      </div>
      <DialogFooter
        error={repos.error}
        hint="Watch polls a repo and deploys new branch tips automatically."
      >
        <button
          type="button"
          onClick={onClose}
          className={`${neutralButtonClass} px-3 py-1 text-sm`}
        >
          Done
        </button>
      </DialogFooter>
    </Modal>
  );
}

// UsageCell right-aligns one byte count in the storage table; zero renders
// muted so the eye lands on the rows that actually cost disk.
function UsageCell({ bytes }: { bytes: number }) {
  return (
    <td className={`px-3 py-1.5 text-right tabular-nums ${bytes === 0 ? "text-fg-muted/50" : ""}`}>
      {formatBytes(bytes)}
    </td>
  );
}

// WarmForm edits the warm-process cap: how many preview processes stay
// running per serving node before the least-recently-used are stopped.
// Applies without a restart — locally on the next reaper tick, and to every
// worker via the control node's heartbeat loop.
function WarmForm() {
  const queryClient = useQueryClient();
  const policy = useQuery({ queryKey: ["warm"], queryFn: api.getWarm });
  const [maxWarm, setMaxWarm] = useState("0");
  const [minWarm, setMinWarm] = useState("0");
  const [idleMinutes, setIdleMinutes] = useState("0");

  useEffect(() => {
    if (policy.data) {
      setMaxWarm(String(policy.data.max_warm));
      setMinWarm(String(policy.data.min_warm));
      setIdleMinutes(String(Math.round(policy.data.idle_timeout_seconds / 60)));
    }
  }, [policy.data]);

  const parsed = {
    max_warm: Math.max(0, Math.floor(Number(maxWarm) || 0)),
    min_warm: Math.max(0, Math.floor(Number(minWarm) || 0)),
    idle_timeout_seconds: Math.max(0, Math.floor(Number(idleMinutes) || 0)) * 60,
  };
  const dirty =
    policy.data != null &&
    (parsed.max_warm !== policy.data.max_warm ||
      parsed.min_warm !== policy.data.min_warm ||
      parsed.idle_timeout_seconds !== policy.data.idle_timeout_seconds);

  const save = useMutation({
    mutationFn: () => api.putWarm(parsed),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["warm"] }),
  });

  const numberInputClass =
    "w-28 rounded bg-surface px-2 py-1 text-sm text-fg placeholder:text-fg-muted/60";
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-end gap-3">
        <Field label="Warm target per node">
          <input
            type="number"
            min={0}
            value={maxWarm}
            onChange={(e) => setMaxWarm(e.target.value)}
            className={numberInputClass}
          />
        </Field>
        <Field label="Always keep warm (min)">
          <input
            type="number"
            min={0}
            value={minWarm}
            onChange={(e) => setMinWarm(e.target.value)}
            className={numberInputClass}
          />
        </Field>
        <Field label="Idle timeout (minutes)">
          <input
            type="number"
            min={0}
            value={idleMinutes}
            onChange={(e) => setIdleMinutes(e.target.value)}
            className={numberInputClass}
          />
        </Field>
        {dirty && (
          <button
            type="button"
            onClick={() => save.mutate()}
            disabled={save.isPending}
            className={`${neutralButtonClass} px-3 py-1 text-sm`}
          >
            Save
          </button>
        )}
      </div>
      <p className="text-xs text-fg-muted">
        0 = unlimited / per-manifest. Beyond the cap the least-recently-used processes stop (they
        cold-start on their next visit); a deployment with a frontend process counts as two. The
        idle timeout stops a preview that long after its last request — a value here overrides every
        manifest's idle_timeout (default 30m) and governs already-running previews. Both apply to
        every worker without a restart.
      </p>
      {save.error != null && (
        <p className="text-xs text-danger" title={String(save.error)}>
          {String(save.error)}
        </p>
      )}
    </div>
  );
}

// RetentionForm edits the instance's retention policy: per-repo deploy cap
// and max age, 0 = unlimited. Saving never evicts by itself — limits apply
// on the next sweep (hourly, or via "Run GC now").
function RetentionForm() {
  const queryClient = useQueryClient();
  const policy = useQuery({ queryKey: ["retention"], queryFn: api.getRetention });
  const [maxDeploys, setMaxDeploys] = useState("0");
  const [maxAge, setMaxAge] = useState("0");

  // Track the server's value when it changes under us (initial load, save
  // refetch) so the dirty check stays accurate.
  useEffect(() => {
    if (policy.data) {
      setMaxDeploys(String(policy.data.max_deploys_per_repo));
      setMaxAge(String(policy.data.max_age_days));
    }
  }, [policy.data]);

  const parsed = {
    max_deploys_per_repo: Math.max(0, Math.floor(Number(maxDeploys) || 0)),
    max_age_days: Math.max(0, Math.floor(Number(maxAge) || 0)),
  };
  const dirty =
    policy.data != null &&
    (parsed.max_deploys_per_repo !== policy.data.max_deploys_per_repo ||
      parsed.max_age_days !== policy.data.max_age_days);

  const save = useMutation({
    mutationFn: () => api.putRetention(parsed),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["retention"] }),
  });

  // Not inputClass: its w-full would stretch a small numeric field across
  // the dialog.
  const numberInputClass =
    "w-28 rounded bg-surface px-2 py-1 text-sm text-fg placeholder:text-fg-muted/60";
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-end gap-3">
        <Field label="Deploys kept per repo">
          <input
            type="number"
            min={0}
            value={maxDeploys}
            onChange={(e) => setMaxDeploys(e.target.value)}
            className={numberInputClass}
          />
        </Field>
        <Field label="Max age (days)">
          <input
            type="number"
            min={0}
            value={maxAge}
            onChange={(e) => setMaxAge(e.target.value)}
            className={numberInputClass}
          />
        </Field>
        {dirty && (
          <button
            type="button"
            onClick={() => save.mutate()}
            disabled={save.isPending}
            className={`${neutralButtonClass} px-3 py-1 text-sm`}
          >
            Save
          </button>
        )}
      </div>
      <p className="text-xs text-fg-muted">
        0 = unlimited. Each repo's newest ready deployment always survives; deployments a branch
        alias routes to and in-flight builds are never evicted.
      </p>
      {save.error != null && (
        <p className="text-xs text-danger" title={String(save.error)}>
          {String(save.error)}
        </p>
      )}
    </div>
  );
}

// formatSeconds renders a duration for the stats view: sub-second as ms,
// otherwise seconds (or minutes past a minute).
function formatSeconds(s: number): string {
  if (s < 1) return `${Math.round(s * 1000)}ms`;
  if (s < 60) return `${s.toFixed(1)}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${Math.round(s - m * 60)}s`;
}

// StatCard is a labelled headline figure with an optional sub-line, the unit
// tile the statistics view is built from.
function StatCard({
  label,
  value,
  sub,
  title,
}: {
  label: string;
  value: string;
  sub?: string;
  title?: string;
}) {
  return (
    <div
      className="flex flex-col gap-0.5 rounded border border-border bg-surface-2 px-3 py-2"
      title={title}
    >
      <span className="text-[10px] font-semibold uppercase tracking-wide text-fg-muted">
        {label}
      </span>
      <span className="text-xl font-semibold tabular-nums">{value}</span>
      {sub && <span className="text-[11px] text-fg-muted">{sub}</span>}
    </div>
  );
}

// StatisticsDialog surfaces instance-wide metrics: cold-start latency
// percentiles, the warm-cache hit ratio, and the live serving footprint.
// Runtime figures are live, so the query refetches on an interval.
function StatisticsDialog({ onClose }: { onClose: () => void }) {
  const stats = useQuery<StatsReport>({
    queryKey: ["stats"],
    queryFn: api.getStats,
    refetchInterval: 5000,
  });
  const s = stats.data;

  const warmPct = s && s.hits.warm_ratio != null ? `${(s.hits.warm_ratio * 100).toFixed(1)}%` : "—";
  const totalHits = s ? s.hits.warm + s.hits.cold : 0;

  return (
    <Modal title="Statistics" onClose={onClose} wide>
      <div className="flex flex-col gap-5 p-4 text-sm">
        {stats.isError && (
          <p className="text-xs text-danger">Failed to load statistics: {String(stats.error)}</p>
        )}
        {!s && !stats.isError && <p className="text-xs text-fg-muted">measuring…</p>}
        {s && (
          <>
            <section className="flex flex-col gap-2">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted">
                Cold-start latency
                <span className="ml-2 font-normal normal-case text-fg-muted">
                  last 30 days · {s.startup.count} start{s.startup.count === 1 ? "" : "s"}
                </span>
              </h3>
              {s.startup.count > 0 ? (
                <div className="grid grid-cols-3 gap-2">
                  <StatCard
                    label="p50"
                    value={formatSeconds(s.startup.p50_seconds ?? 0)}
                    title="Median time from process start to first healthy response"
                  />
                  <StatCard
                    label="p90"
                    value={formatSeconds(s.startup.p90_seconds ?? 0)}
                    title="90th-percentile cold-start time"
                  />
                  <StatCard
                    label="p99"
                    value={formatSeconds(s.startup.p99_seconds ?? 0)}
                    title="99th-percentile cold-start time"
                  />
                </div>
              ) : (
                <p className="text-xs text-fg-muted">
                  No cold starts recorded yet — deploy and open a preview to populate this.
                </p>
              )}
            </section>

            <section className="flex flex-col gap-2">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted">
                Warm cache
                <span className="ml-2 font-normal normal-case text-fg-muted">since startup</span>
              </h3>
              <div className="grid grid-cols-3 gap-2">
                <StatCard
                  label="warm hit rate"
                  value={warmPct}
                  sub={`${totalHits.toLocaleString()} request${totalHits === 1 ? "" : "s"}`}
                  title="Share of preview requests served by an already-running process (no cold start)"
                />
                <StatCard
                  label="warm hits"
                  value={s.hits.warm.toLocaleString()}
                  title="Requests served by an already-running preview process"
                />
                <StatCard
                  label="cold starts"
                  value={s.hits.cold.toLocaleString()}
                  title="Requests that had to start a preview process"
                />
              </div>
            </section>

            <section className="flex flex-col gap-2">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted">
                Runtime
              </h3>
              <div className="grid grid-cols-3 gap-2">
                <StatCard
                  label={s.runtime.workers === 1 ? "node" : "workers"}
                  value={s.runtime.workers.toLocaleString()}
                  title="Serving nodes currently reporting healthy"
                />
                <StatCard
                  label="running"
                  value={s.runtime.running.toLocaleString()}
                  sub="warm processes"
                  title="Preview processes currently running (committed warm slots)"
                />
                <StatCard
                  label="capacity"
                  value={
                    s.runtime.capacity === 0 ? "unlimited" : s.runtime.capacity.toLocaleString()
                  }
                  sub={s.runtime.capacity === 0 ? undefined : "warm slots"}
                  title="Total bounded warm-process capacity across serving nodes (0 = unlimited)"
                />
              </div>
            </section>
          </>
        )}
      </div>
    </Modal>
  );
}

// StorageDialog is the instance-management view: how much disk the data
// directory uses (by category and by repo), the retention policy, and a
// manual GC trigger.
function StorageDialog({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient();
  const storage = useQuery({ queryKey: ["storage"], queryFn: api.getStorage });
  const [gcResult, setGcResult] = useState<GCResult | null>(null);
  const gc = useMutation({
    mutationFn: api.runGC,
    onSuccess: (res) => {
      setGcResult(res);
      queryClient.invalidateQueries({ queryKey: ["storage"] });
      queryClient.invalidateQueries({ queryKey: ["deploys"] });
    },
  });

  const s = storage.data;
  return (
    <Modal title="Storage & retention" onClose={onClose} wide>
      <div className="flex flex-col gap-4 p-4 text-sm">
        <section className="flex flex-col gap-2">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted">
              Disk usage
            </h3>
            {s ? (
              <>
                <Metric label="total" value={formatBytes(s.total_bytes)} />
                <Metric
                  label={s.durable_tier_configured ? "artifacts (resident)" : "artifacts"}
                  value={formatBytes(s.artifacts_bytes)}
                  title={
                    s.durable_tier_configured
                      ? "Published builds resident on local disk — a cache of the durable tier; evicted artifacts re-hydrate on serve"
                      : "Published builds (frontend, backend, downloadable artifacts)"
                  }
                />
                {s.durable_tier_configured && (
                  <Metric
                    label="artifacts (durable)"
                    value={formatBytes(s.durable_bytes)}
                    title="Total footprint of the durable artifact tier (S3/MinIO)"
                  />
                )}
                <Metric
                  label="state"
                  value={formatBytes(s.state_bytes)}
                  title="Mutable backend state directories"
                />
                <Metric label="logs" value={formatBytes(s.logs_bytes)} />
                <Metric
                  label="mirrors"
                  value={formatBytes(s.mirror_bytes)}
                  title="Bare git clones of registered repos"
                />
                <Metric label="tmp" value={formatBytes(s.tmp_bytes)} />
                <Metric label="db" value={formatBytes(s.db_bytes)} />
              </>
            ) : (
              <span className="text-xs text-fg-muted">measuring…</span>
            )}
          </div>
          {s && s.repos.length > 0 && (
            <div className="overflow-x-auto rounded border border-border">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border text-left text-fg-muted">
                    <th className="px-3 py-1.5 font-medium">repo</th>
                    <th className="px-3 py-1.5 text-right font-medium">deploys</th>
                    <th className="px-3 py-1.5 text-right font-medium">artifacts</th>
                    <th className="px-3 py-1.5 text-right font-medium">state</th>
                    <th className="px-3 py-1.5 text-right font-medium">logs</th>
                    <th className="px-3 py-1.5 text-right font-medium">mirror</th>
                    <th className="px-3 py-1.5 text-right font-medium">total</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {s.repos.map((r) => (
                    <tr key={r.repo}>
                      <td className="px-3 py-1.5 font-medium">{r.repo}</td>
                      <td
                        className="px-3 py-1.5 text-right tabular-nums"
                        title={`${r.deploys} active, ${r.evicted_deploys} evicted`}
                      >
                        {r.deploys}
                        {r.evicted_deploys > 0 && (
                          <span className="text-fg-muted"> +{r.evicted_deploys} evicted</span>
                        )}
                      </td>
                      <UsageCell bytes={r.artifacts_bytes} />
                      <UsageCell bytes={r.state_bytes} />
                      <UsageCell bytes={r.logs_bytes} />
                      <UsageCell bytes={r.mirror_bytes} />
                      <UsageCell bytes={r.total_bytes} />
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="flex flex-col gap-2">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted">
            Warm previews
          </h3>
          <WarmForm />
        </section>

        <section className="flex flex-col gap-2">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted">
            Retention policy
          </h3>
          <RetentionForm />
        </section>

        <section className="flex flex-col gap-2">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted">
            Garbage collection
          </h3>
          <div className="flex flex-wrap items-center gap-3">
            <button
              type="button"
              onClick={() => gc.mutate()}
              disabled={gc.isPending}
              className={buttonClass}
            >
              {gc.isPending ? "Collecting…" : "Run GC now"}
            </button>
            {gcResult && !gc.isPending && (
              <span className="text-xs text-fg-muted">
                {gcResult.evicted.length > 0
                  ? `Evicted ${gcResult.evicted.length} deployment${
                      gcResult.evicted.length === 1 ? "" : "s"
                    }, freed ${formatBytes(gcResult.freed_bytes)}.`
                  : "Nothing to collect."}
              </span>
            )}
          </div>
          <p className="text-xs text-fg-muted">
            Applies the retention policy immediately and clears stale staging files. The sweep also
            runs hourly on its own.
          </p>
        </section>
      </div>
      <DialogFooter
        error={storage.error ?? gc.error}
        hint="Evicted deployments keep their history — the row's redeploy button rebuilds one."
      >
        <button
          type="button"
          onClick={onClose}
          className={`${neutralButtonClass} px-3 py-1 text-sm`}
        >
          Done
        </button>
      </DialogFooter>
    </Modal>
  );
}

// Filter options, in build-lifecycle order with "crashed" beside the
// runtime state it names: a crashed deploy built fine, so it sorts after
// "ready" — which no longer answers for it.
const deployStatuses: DeployStatusFilter[] = [
  "queued",
  "building",
  "ready",
  "crashed",
  "failed",
  "evicted",
];

// The list fetches one page at a time — deploys accumulate per commit, and
// the 5s poll shouldn't grow with a repo's whole history. Older deploys are
// a page away; search and the status filter narrow the whole history, not
// just the page on screen.
const pageSize = 50;

export default function App() {
  const [dialog, setDialog] = useState<
    "register" | "deploy" | "repos" | "storage" | "stats" | null
  >(null);
  const [detailId, setDetailId] = useState<number | null>(null);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<DeployStatusFilter | "">("");
  const query = useDebounced(search.trim(), 200);
  const filtersActive = query !== "" || statusFilter !== "";
  const [offset, setOffset] = useState(0);
  // A narrower filter can leave the current page past the end of the results,
  // so every filter change starts over at the first page.
  // biome-ignore lint/correctness/useExhaustiveDependencies: resetting on filter change is the point
  useEffect(() => setOffset(0), [query, statusFilter]);

  const health = useQuery({
    queryKey: ["health"],
    queryFn: api.health,
    // Keep polling so the unreachable banner clears itself on recovery.
    refetchInterval: 5000,
  });
  const repos = useQuery({
    queryKey: ["repos"],
    queryFn: api.listRepos,
    // Poll while a registration's background clone runs so its badge (and
    // the deploy dialog) flip to ready on their own.
    refetchInterval: (query) =>
      query.state.data?.some((r) => r.status === "cloning") ? 1000 : false,
  });
  const deploys = useQuery({
    queryKey: ["deploys", query, statusFilter, offset],
    queryFn: () => api.listDeploys({ q: query, status: statusFilter, limit: pageSize, offset }),
    // Keep the previous page while a filter or page change refetches, so the
    // list doesn't blank out between keystrokes.
    placeholderData: keepPreviousData,
    // Poll while anything is in flight (builds, post-ready artifact builds,
    // or cold starts) so state flips surface promptly.
    refetchInterval: (query) =>
      query.state.data?.deploys.some(
        (d) =>
          d.status === "queued" ||
          d.status === "building" ||
          d.process === "starting" ||
          d.fe_process === "starting" ||
          d.artifacts?.some((a) => a.status === "building"),
      )
        ? 1000
        : 5000,
  });

  const hasRepos = (repos.data?.length ?? 0) > 0;
  const rows = deploys.data?.deploys;
  const total = deploys.data?.total ?? 0;
  // Deletion and eviction can shrink the list out from under the current
  // page; step back to the last one that still has rows instead of stranding
  // the user on an empty page with no way forward.
  useEffect(() => {
    if (offset > 0 && rows?.length === 0) setOffset(Math.max(0, total - pageSize));
  }, [offset, rows, total]);
  // Resolve from the live list each render so the dialog tracks state
  // changes (build finishing, processes warming) instead of a snapshot.
  const detail = detailId != null ? rows?.find((d) => d.id === detailId) : undefined;

  return (
    <div className="flex min-h-screen flex-col bg-bg text-fg">
      <header className="flex items-center gap-3 border-b border-border px-3 py-2">
        <div className="flex items-center gap-2.5">
          <Logo />
          <h1 className="text-lg font-semibold">Previews</h1>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <button
            type="button"
            onClick={() => setDialog("register")}
            className={`${neutralButtonClass} h-7 gap-1`}
          >
            <IconPlus className="h-4 w-4" />
            register repo
          </button>
          <a
            href="https://jamison.lahman.dev/local-preview/guide/"
            target="_blank"
            rel="noopener noreferrer"
            title="Help"
            aria-label="Help"
            className="inline-flex h-7 w-7 items-center justify-center rounded bg-surface-2 text-fg transition-colors duration-150 hover:bg-surface-3"
          >
            <IconHelp />
          </a>
          <button
            type="button"
            onClick={() => setDialog("repos")}
            title="Manage registered repositories"
            aria-label="Manage registered repositories"
            className="inline-flex h-7 w-7 items-center justify-center rounded bg-surface-2 text-fg transition-colors duration-150 hover:bg-surface-3"
          >
            <IconSettings />
          </button>
          <button
            type="button"
            onClick={() => setDialog("stats")}
            title="Statistics"
            aria-label="Statistics"
            className="inline-flex h-7 w-7 items-center justify-center rounded bg-surface-2 text-fg transition-colors duration-150 hover:bg-surface-3"
          >
            <IconChart />
          </button>
          <button
            type="button"
            onClick={() => setDialog("storage")}
            title="Storage & retention"
            aria-label="Storage & retention"
            className="inline-flex h-7 w-7 items-center justify-center rounded bg-surface-2 text-fg transition-colors duration-150 hover:bg-surface-3"
          >
            <IconDatabase />
          </button>
          <UserMenu />
          <ThemeToggle />
        </div>
      </header>
      {health.isError && (
        <div className="border-b border-amber-700 bg-amber-950/60 px-4 py-1 text-xs text-amber-200">
          Server unreachable — retrying…
        </div>
      )}

      <main className="mx-auto w-full max-w-5xl flex-1 px-3 py-4">
        <section>
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <div className="flex items-baseline gap-2">
              <h2 className="text-sm font-semibold uppercase tracking-wide">Deployments</h2>
              {total > 0 && <span className="text-xs text-fg-muted tabular-nums">{total}</span>}
            </div>
            <div className="ml-auto flex flex-wrap items-center gap-2">
              <div className="relative">
                <IconSearch className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-fg-muted/60" />
                <input
                  type="search"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="Search sha, branch, author…"
                  aria-label="Search deployments"
                  className="h-7 w-56 rounded border border-border bg-surface pl-7 pr-2 text-sm text-fg placeholder:text-fg-muted/60"
                />
              </div>
              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value as DeployStatusFilter | "")}
                aria-label="Filter by status"
                className="h-7 cursor-pointer rounded border border-border bg-surface px-2 text-sm text-fg"
              >
                <option value="">all statuses</option>
                {deployStatuses.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
              <button type="button" onClick={() => setDialog("deploy")} className={buttonClass}>
                Deploy
              </button>
            </div>
          </div>
          <div className="overflow-hidden rounded border border-border bg-surface">
            {deploys.isPending && (
              <div className="divide-y divide-border">
                {[0, 1, 2].map((i) => (
                  <div key={i} className="flex animate-pulse items-center gap-4 px-3 py-3">
                    <div className="h-5 w-22 rounded-full bg-surface-2" />
                    <div className="h-4 w-40 rounded bg-surface-2" />
                    <div className="ml-auto h-4 w-24 rounded bg-surface-2" />
                  </div>
                ))}
              </div>
            )}
            {rows?.length === 0 && filtersActive && (
              <div className="flex flex-col items-center gap-1 px-6 py-16 text-center">
                <IconSearch className="mb-2 h-8 w-8 text-fg-muted/50" />
                <p className="text-sm font-medium">No matching deployments</p>
                <p className="text-xs text-fg-muted">
                  Try a different search or clear the filters.
                </p>
                <button
                  type="button"
                  onClick={() => {
                    setSearch("");
                    setStatusFilter("");
                  }}
                  className={`${neutralButtonClass} mt-3`}
                >
                  Clear filters
                </button>
              </div>
            )}
            {rows?.length === 0 && !filtersActive && (
              <div className="flex flex-col items-center gap-1 px-6 py-16 text-center">
                <IconDeploy className="mb-2 h-8 w-8 text-fg-muted/50" />
                <p className="text-sm font-medium">No deployments yet</p>
                <p className="text-xs text-fg-muted">
                  {hasRepos
                    ? "Deploy a commit to get your first preview."
                    : "Register a repository to get started."}
                </p>
                <button
                  type="button"
                  onClick={() => setDialog(hasRepos ? "deploy" : "register")}
                  className={`${buttonClass} mt-3`}
                >
                  {hasRepos ? "Deploy a commit" : "Register a repository"}
                </button>
              </div>
            )}
            <ul className="divide-y divide-border">
              {rows?.map((d) => {
                // Per-service content hashes live in the commit sha's hover
                // instead of taking up a row of their own.
                const shaTitle = [
                  d.sha,
                  d.fe_hash ? `frontend ${d.fe_hash.slice(0, 12)}` : null,
                  d.be_hash ? `backend ${d.be_hash.slice(0, 12)}` : null,
                  ...(d.artifacts?.map((a) => `${a.name} ${a.hash.slice(0, 12)}`) ?? []),
                ]
                  .filter(Boolean)
                  .join("\n");
                const state = deployState(d);
                const crashError = state === "crashed" ? processError(d) : null;
                // A process is worth stopping once it's warm or warming; a
                // crashed one has nothing to stop but a crash to dismiss.
                const stoppable =
                  state === "running" || state === "starting" || state === "crashed";
                return (
                  <li
                    key={d.id}
                    className="flex flex-wrap items-center gap-x-4 gap-y-1.5 px-3 py-3 transition-colors hover:bg-surface-2/40"
                  >
                    <StatusBadge state={state} />
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                        <span className="text-sm font-medium">{d.repo}</span>
                        <code className="font-mono text-xs text-fg-muted" title={shaTitle}>
                          {d.short_sha}
                        </code>
                        {d.author_name && (
                          <span className="text-xs text-fg-muted" title={d.author_email}>
                            by {d.author_name}
                          </span>
                        )}
                      </div>
                      {(d.branch || (d.ref && d.ref !== d.branch)) && (
                        <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5">
                          {d.branch && (
                            <span
                              className="inline-flex min-w-0 items-center gap-1 rounded-full border border-border bg-surface-2 px-2 py-px font-mono text-[11px] text-fg-muted"
                              title={d.branch}
                            >
                              <IconGitBranch className="h-3 w-3 shrink-0" />
                              <span className="truncate">{d.branch}</span>
                            </span>
                          )}
                          {d.ref && d.ref !== d.branch && (
                            <span className="min-w-0 truncate rounded-full border border-border bg-surface-2 px-2 py-px font-mono text-[11px] text-fg-muted">
                              {d.ref}
                            </span>
                          )}
                        </div>
                      )}
                      {d.status === "failed" && d.error && (
                        <p
                          className="mt-0.5 max-w-full truncate text-xs text-danger"
                          title={d.error}
                        >
                          {d.error}
                        </p>
                      )}
                      {crashError && (
                        <p
                          className="mt-0.5 max-w-full truncate text-xs text-danger"
                          title={crashError}
                        >
                          {crashError}
                        </p>
                      )}
                    </div>
                    {d.artifacts?.map((a) => (
                      <ArtifactDownload key={a.name} artifact={a} />
                    ))}
                    {stoppable && <StopButton deploy={d} crashed={state === "crashed"} />}
                    {d.status === "evicted" && <RedeployButton deploy={d} />}
                    <button
                      type="button"
                      onClick={() => setDetailId(d.id)}
                      title="Logs and resource usage"
                      className={`${neutralButtonClass} gap-1`}
                    >
                      <IconPulse className="h-3 w-3" />
                      logs
                    </button>
                    {d.preview_url ? (
                      <a
                        href={d.preview_url}
                        target="_blank"
                        rel="noreferrer"
                        className={`${accentButtonClass} gap-1`}
                      >
                        open
                        <IconArrowUpRight className="h-3 w-3" />
                      </a>
                    ) : (
                      <span className="w-14 text-center text-xs text-fg-muted/50">—</span>
                    )}
                    <time
                      dateTime={d.created_at}
                      title={d.created_at}
                      className="w-14 shrink-0 text-right text-xs text-fg-muted tabular-nums"
                    >
                      {timeAgo(d.created_at)}
                    </time>
                    <DeleteDeployButton deploy={d} />
                  </li>
                );
              })}
            </ul>
            {total > pageSize && rows && rows.length > 0 && (
              <div className="flex items-center gap-2 border-t border-border px-3 py-2 text-xs text-fg-muted">
                <span className="tabular-nums">
                  {offset + 1}–{offset + rows.length} of {total}
                </span>
                <div className="ml-auto flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => setOffset(Math.max(0, offset - pageSize))}
                    disabled={offset === 0}
                    className={`${neutralButtonClass} px-2 py-0.5 disabled:opacity-40`}
                  >
                    Newer
                  </button>
                  <button
                    type="button"
                    onClick={() => setOffset(offset + pageSize)}
                    disabled={offset + rows.length >= total}
                    className={`${neutralButtonClass} px-2 py-0.5 disabled:opacity-40`}
                  >
                    Older
                  </button>
                </div>
              </div>
            )}
          </div>
        </section>
      </main>

      <footer className="border-t border-border px-3 py-2 font-mono text-[11px] text-fg-muted">
        {health.data ? `server ${health.data.version}` : "server …"}
      </footer>

      {dialog === "register" && <RegisterRepoDialog onClose={() => setDialog(null)} />}
      {dialog === "repos" && <ManageReposDialog onClose={() => setDialog(null)} />}
      {dialog === "storage" && <StorageDialog onClose={() => setDialog(null)} />}
      {dialog === "stats" && <StatisticsDialog onClose={() => setDialog(null)} />}
      {dialog === "deploy" && (
        <DeployDialog
          repos={repos.data ?? []}
          previewDomain={health.data?.preview_domain}
          onClose={() => setDialog(null)}
        />
      )}
      {detail && <DeployDetailDialog deploy={detail} onClose={() => setDetailId(null)} />}
    </div>
  );
}

function Logo() {
  return (
    <svg viewBox="0 0 32 32" className="h-5 w-5" aria-hidden="true">
      <rect x="2" y="2" width="28" height="28" rx="6" className="fill-surface-3" />
      <rect x="7" y="7" width="18" height="5" rx="2" className="fill-accent-500" />
      <rect x="7" y="14" width="18" height="5" rx="2" className="fill-fg-muted" />
      <rect x="7" y="21" width="11" height="5" rx="2" className="fill-fg-muted" />
    </svg>
  );
}

function IconSun() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-4 w-4"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4m11.4-11.4 1.4-1.4" />
    </svg>
  );
}

function IconMoon() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-4 w-4"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8z" />
    </svg>
  );
}

function IconX() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-4 w-4"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M18 6 6 18M6 6l12 12" />
    </svg>
  );
}

function IconPlus({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

function IconGitBranch({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <line x1="6" y1="3" x2="6" y2="15" />
      <circle cx="18" cy="6" r="3" />
      <circle cx="6" cy="18" r="3" />
      <path d="M18 9a9 9 0 0 1-9 9" />
    </svg>
  );
}

function IconArrowUpRight({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M7 17 17 7M8 7h9v9" />
    </svg>
  );
}

function IconSearch({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="11" cy="11" r="7" />
      <path d="m21 21-4.35-4.35" />
    </svg>
  );
}

function IconCopy({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <rect x="9" y="9" width="11" height="11" rx="2" />
      <path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1" />
    </svg>
  );
}

function IconCheck({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="m5 13 4 4L19 7" />
    </svg>
  );
}

function IconDownload({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M12 3v12m0 0 4-4m-4 4-4-4" />
      <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2" />
    </svg>
  );
}

function IconChevronDown({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="m6 9 6 6 6-6" />
    </svg>
  );
}

function IconRefresh({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8" />
      <path d="M21 3v5h-5" />
    </svg>
  );
}

function IconPulse({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
    </svg>
  );
}

function IconDeploy({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="m12 3 9 5-9 5-9-5 9-5z" />
      <path d="m3 13 9 5 9-5" opacity="0.5" />
    </svg>
  );
}

function IconStop({ className = "" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} fill="currentColor" aria-hidden="true">
      <rect x="6" y="6" width="12" height="12" rx="1.5" />
    </svg>
  );
}

function IconTrash({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M3 6h18M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2m2 0v14a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V6" />
      <path d="M10 11v6M14 11v6" />
    </svg>
  );
}

function IconChart() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-4 w-4"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M3 3v18h18" />
      <rect x="7" y="12" width="3" height="6" />
      <rect x="12" y="8" width="3" height="10" />
      <rect x="17" y="4" width="3" height="14" />
    </svg>
  );
}

function IconDatabase() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-4 w-4"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <ellipse cx="12" cy="5" rx="8" ry="3" />
      <path d="M4 5v14c0 1.66 3.58 3 8 3s8-1.34 8-3V5" />
      <path d="M4 12c0 1.66 3.58 3 8 3s8-1.34 8-3" />
    </svg>
  );
}

function IconHelp() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-4 w-4"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="10" />
      <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
  );
}

function IconSettings() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="h-4 w-4"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  );
}
