import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  api,
  type ReleaseBump,
  type ReleaseCI,
  type ReleaseProject,
  type ReleaseReminder,
  type ReleaseRun,
  type ReleaseSubtree,
} from "@/api/client";
import { queryKeys } from "@/api/keys";
import { readShowUnreleased, writeShowUnreleased } from "@/storage";
import { Button } from "./Button";
import { Modal } from "./Modal";

// The releases dashboard: notes on what's meant to be released, each git-orchard
// subtree's state, and a confirmed release through the same code as
// `git orchard release`. Reminders track nothing and never fire by themselves.

const BUMPS: ReleaseBump[] = ["patch", "minor", "major"];

// The summary is cheap and only changes on a refresh or a run.
const POLL_MS = 15_000;
const POLL_REFRESHING_MS = 3_000;

// A preview fetches, so reopening the modal reuses a recent one. Run refuses a
// release that has moved on since, so a stale preview can't publish the wrong thing.
const PREVIEW_STALE_MS = 60_000;
const PREVIEW_CACHE_MS = 5 * 60_000;

const chip = "rounded-full border border-border";

const runStyles: Record<ReleaseRun["status"], string> = {
  running: "border-warning/30 text-warning",
  released: "border-success/30 text-success",
  failed: "border-danger/30 text-danger",
  interrupted: "border-danger/30 text-danger",
};

function shortRev(rev: string) {
  return rev.slice(0, 12);
}

function BumpSelect({
  value,
  onChange,
}: {
  value: ReleaseBump;
  onChange: (bump: ReleaseBump) => void;
}) {
  return (
    <select
      aria-label="Bump"
      className="cursor-pointer rounded bg-surface px-2 py-1 text-sm"
      value={value}
      onChange={(e) => onChange(e.target.value as ReleaseBump)}
    >
      {BUMPS.map((b) => (
        <option key={b} value={b}>
          {b}
        </option>
      ))}
    </select>
  );
}

// Errored subtrees are unknown, not unreleased.
function neverReleased(s: ReleaseSubtree) {
  return !s.error && !s.latest;
}

function unreleasedLabel(s: ReleaseSubtree | undefined) {
  if (!s) return "";
  if (s.error) return "state unknown";
  if (!s.counted) return "unreleased commits unknown";
  return `${s.unreleased} unreleased commit${s.unreleased === 1 ? "" : "s"}`;
}

function previewOptions(repo: string, prefix: string, bump: ReleaseBump, suffix: string) {
  return {
    queryKey: queryKeys.releasePreview(repo, prefix, bump, suffix),
    queryFn: () => api.previewRelease(repo, prefix, bump, suffix),
    // A preview is reusable for a while; the server rechecks it when the run starts.
    gcTime: PREVIEW_CACHE_MS,
    staleTime: PREVIEW_STALE_MS,
  };
}

type ReleaseTarget = {
  repo: string;
  prefix: string;
  bump: ReleaseBump;
  suffix: string;
  reminderId?: number;
};

/**
 * Previews on demand, since it fetches and may split, then posts back what it
 * showed: the server refuses if that's no longer what would be released.
 */
function ReleaseModal({
  target,
  onClose,
  onStarted,
  onRemind,
}: {
  target: ReleaseTarget;
  onClose: () => void;
  onStarted: (run: ReleaseRun) => void;
  /** Absent where a reminder already exists or this release came from one. */
  onRemind?: (bump: ReleaseBump) => void;
}) {
  const [bump, setBump] = useState(target.bump);
  const previewQ = useQuery({
    ...previewOptions(target.repo, target.prefix, bump, target.suffix),
    retry: false,
  });
  const preview = previewQ.data;
  const runMut = useMutation({
    mutationFn: () => {
      if (!preview) throw new Error("nothing to release");
      return api.runRelease({
        repo: target.repo,
        prefix: target.prefix,
        bump,
        suffix: target.suffix,
        version: preview.version,
        rev: preview.rev,
        reminder_id: target.reminderId,
      });
    },
    onSuccess: onStarted,
  });
  const error = previewQ.error ?? runMut.error;

  return (
    <Modal open onClose={onClose} title={`Release ${target.prefix}`} busy={runMut.isPending}>
      <div className="flex flex-col gap-3 p-4 text-sm">
        <div className="flex items-center gap-2">
          <span className="text-fg-muted">Bump</span>
          <BumpSelect value={bump} onChange={setBump} />
        </div>
        {!preview && previewQ.isFetching ? (
          <p className="text-fg-muted">Fetching and working out the version…</p>
        ) : preview ? (
          <>
            <p>
              Tag <span className="font-medium">{preview.version}</span> at{" "}
              <code>{shortRev(preview.rev)}</code> and push it to origin.
            </p>
            {preview.released && (
              <p className="text-fg-muted">
                This commit is already tagged, so this publishes it again.
              </p>
            )}
            {preview.blocked && <p className="text-danger">{preview.blocked}</p>}
          </>
        ) : null}
        {error && <p className="text-danger">{String(error)}</p>}
        <div className="flex justify-end gap-2">
          {onRemind && (
            <Button
              variant="ghost"
              size="lg"
              className="mr-auto"
              onClick={() => onRemind(bump)}
              disabled={runMut.isPending}
            >
              remind me later
            </Button>
          )}
          <Button variant="neutral" size="lg" onClick={onClose} disabled={runMut.isPending}>
            cancel
          </Button>
          <Button
            variant="primary"
            size="lg"
            disabled={!preview || !!preview.blocked}
            pending={runMut.isPending}
            idleLabel="release"
            pendingLabel="releasing…"
            onClick={() => runMut.mutate()}
          />
        </div>
      </div>
    </Modal>
  );
}

/** Adds a reminder, or edits one: a second thought goes in the same note. */
function ReminderModal({
  repo,
  prefix,
  reminder,
  bump: initialBump,
  onClose,
}: {
  repo: string;
  prefix: string;
  reminder?: ReleaseReminder;
  /** What a new reminder starts with, such as the bump just being looked at. */
  bump?: ReleaseBump;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [bump, setBump] = useState<ReleaseBump>(reminder?.bump ?? initialBump ?? "patch");
  const [note, setNote] = useState(reminder?.note ?? "");
  const saveMut = useMutation({
    mutationFn: () =>
      reminder
        ? api.updateReleaseReminder(reminder.id, { bump, note })
        : api.createReleaseReminder({ repo_path: repo, prefix, bump, note }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.releases });
      onClose();
    },
  });
  return (
    <Modal open onClose={onClose} title={`Remind me to release ${prefix}`} busy={saveMut.isPending}>
      <form
        className="flex flex-col gap-3 p-4 text-sm"
        onSubmit={(e) => {
          e.preventDefault();
          saveMut.mutate();
        }}
      >
        <div className="flex items-center gap-2">
          <span className="text-fg-muted">Bump</span>
          <BumpSelect value={bump} onChange={setBump} />
        </div>
        <label className="flex flex-col gap-1">
          <span className="text-fg-muted">Note</span>
          <textarea
            // the note is why the modal opened
            autoFocus
            rows={3}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="What's in it?"
            className="w-full rounded bg-surface px-2 py-1 text-fg placeholder:text-fg-muted/60"
          />
        </label>
        {saveMut.error && <p className="text-danger">{String(saveMut.error)}</p>}
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="neutral"
            size="lg"
            onClick={onClose}
            disabled={saveMut.isPending}
          >
            cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            size="lg"
            pending={saveMut.isPending}
            idleLabel="save"
            pendingLabel="saving…"
          />
        </div>
      </form>
    </Modal>
  );
}

/** Streams a run's log, from the start, for as long as it's showing. */
function RunLog({
  run,
  onEnd,
  onClose,
}: {
  run: ReleaseRun;
  onEnd: () => void;
  onClose: () => void;
}) {
  const [lines, setLines] = useState<string[]>([]);
  useEffect(() => {
    setLines([]);
    const es = new EventSource(`/api/release-runs/${run.id}/output`);
    // A reconnect replays the log from the start.
    es.onopen = () => setLines([]);
    es.onmessage = (e) => setLines((prev) => [...prev, e.data]);
    es.addEventListener("end", () => {
      es.close();
      onEnd();
    });
    return () => es.close();
  }, [run.id, onEnd]);
  return (
    <section className="rounded border border-border">
      <header className="flex items-center justify-between gap-2 border-b border-border px-3 py-2">
        <h3 className="min-w-0 truncate text-sm font-medium">
          {run.prefix} {run.version}
        </h3>
        <Button variant="ghost" size="sm" onClick={onClose}>
          close
        </Button>
      </header>
      <pre
        data-testid="release-log"
        className="max-h-96 overflow-auto whitespace-pre-wrap break-all p-3 text-xs"
      >
        {lines.join("\n")}
      </pre>
    </section>
  );
}

function ReminderCard({
  reminder,
  subtree,
  onRelease,
  onEdit,
}: {
  reminder: ReleaseReminder;
  subtree?: ReleaseSubtree;
  onRelease: () => void;
  onEdit: () => void;
}) {
  const qc = useQueryClient();
  const dismissMut = useMutation({
    mutationFn: () => api.dismissReleaseReminder(reminder.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.releases }),
  });
  return (
    <li className="flex flex-col gap-2 rounded border border-border p-3 text-sm">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <span className="font-medium" title={reminder.repo_path}>
          {reminder.prefix}
        </span>
        <span className="text-xs text-fg-muted">
          {reminder.bump}
          {reminder.suffix && ` (${reminder.suffix})`}
          {subtree && ` · ${unreleasedLabel(subtree)}`}
        </span>
      </div>
      {reminder.note && <p className="whitespace-pre-wrap">{reminder.note}</p>}
      {dismissMut.error && <p className="text-xs text-danger">{String(dismissMut.error)}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onEdit}>
          edit
        </Button>
        <Button
          variant="ghost"
          size="sm"
          pending={dismissMut.isPending}
          idleLabel="dismiss"
          pendingLabel="dismissing…"
          onClick={() => dismissMut.mutate()}
        />
        <Button variant="primary" size="sm" className={chip} onClick={onRelease}>
          release
        </Button>
      </div>
    </li>
  );
}

// CICell is information only: a release doesn't check CI.
function CICell({ ci }: { ci?: ReleaseCI }) {
  if (!ci || (ci.status === "none" && !ci.error)) return <>—</>;
  if (ci.status === "failing") {
    return (
      <a href={ci.url} target="_blank" rel="noreferrer" className="text-danger hover:underline">
        failing: {ci.failed?.join(", ")}
      </a>
    );
  }
  if (ci.error) {
    return (
      <span className="text-danger" title={ci.error}>
        error
      </span>
    );
  }
  const color =
    ci.status === "passing"
      ? "text-success"
      : ci.status === "running"
        ? "text-fg-muted"
        : undefined;
  return <span className={color}>{ci.status}</span>;
}

function ProjectTable({
  project,
  onRelease,
  showUnreleased,
  onShowUnreleased,
}: {
  project: ReleaseProject;
  onRelease: (prefix: string) => void;
  showUnreleased: boolean;
  onShowUnreleased: (show: boolean) => void;
}) {
  const qc = useQueryClient();
  // Reaching for the button is the cue to start the fetch the modal will want.
  const warm = (prefix: string) =>
    qc.prefetchQuery(previewOptions(project.repo_path, prefix, "patch", ""));
  // Until its first refresh, a project doesn't know which have been released.
  const subtrees =
    showUnreleased || !project.refreshed_at
      ? project.subtrees
      : project.subtrees.filter((s) => !neverReleased(s));
  return (
    <section className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-medium">
          {project.repo_path}{" "}
          <span className="font-normal text-fg-muted">
            {project.base}
            {project.refreshing && " · refreshing…"}
          </span>
        </h3>
        <label className="flex cursor-pointer items-center gap-1.5 text-sm">
          <input
            type="checkbox"
            checked={showUnreleased}
            onChange={(e) => onShowUnreleased(e.target.checked)}
          />
          show unreleased
        </label>
      </div>
      {project.error && <p className="text-sm text-danger">{project.error}</p>}
      {subtrees.length === 0 ? (
        <p className="text-sm text-fg-muted">No subtree here has been released yet.</p>
      ) : (
        <div className="overflow-x-auto rounded border border-border">
          <table className="w-full text-left text-sm">
            <thead className="text-xs text-fg-muted">
              <tr>
                <th className="px-3 py-2 font-normal">subtree</th>
                <th className="px-3 py-2 font-normal">ahead / behind</th>
                <th className="px-3 py-2 font-normal">latest</th>
                <th className="px-3 py-2 font-normal">unreleased</th>
                <th className="px-3 py-2 font-normal">ci</th>
                <th className="px-3 py-2" />
              </tr>
            </thead>
            <tbody>
              {subtrees.map((s) => (
                <tr key={s.prefix} className="border-t border-border">
                  <td className="px-3 py-2">{s.prefix}</td>
                  {s.error ? (
                    <td colSpan={3} className="px-3 py-2 text-xs text-danger">
                      {s.error}
                    </td>
                  ) : (
                    <>
                      <td className="px-3 py-2">
                        {s.ahead} / {s.behind}
                      </td>
                      <td className="px-3 py-2">{s.latest || "—"}</td>
                      <td className="px-3 py-2">{s.counted ? s.unreleased : "—"}</td>
                    </>
                  )}
                  <td className="px-3 py-2">
                    <CICell ci={s.ci} />
                  </td>
                  <td className="px-3 py-2 text-right whitespace-nowrap">
                    <Button
                      variant="neutral"
                      size="sm"
                      className={chip}
                      onMouseEnter={() => warm(s.prefix)}
                      onFocus={() => warm(s.prefix)}
                      onClick={() => onRelease(s.prefix)}
                    >
                      release
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function RunList({
  runs,
  onOpen,
  onRetry,
}: {
  runs: ReleaseRun[];
  onOpen: (run: ReleaseRun) => void;
  onRetry: (run: ReleaseRun) => void;
}) {
  return (
    <ul className="flex flex-col gap-1 text-sm">
      {runs.map((run) => (
        <li key={run.id} className="flex flex-wrap items-center gap-2">
          <span className={`rounded border px-1.5 text-xs ${runStyles[run.status]}`}>
            {run.status}
          </span>
          <span className="min-w-0 truncate">
            {run.prefix} {run.version}
          </span>
          {run.error && (
            <span className="min-w-0 truncate text-xs text-danger" title={run.error}>
              {run.error}
            </span>
          )}
          <span className="ml-auto flex gap-1">
            {(run.status === "failed" || run.status === "interrupted") && (
              <Button variant="ghost" size="sm" onClick={() => onRetry(run)}>
                retry…
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={() => onOpen(run)}>
              log
            </Button>
          </span>
        </li>
      ))}
    </ul>
  );
}

export function ReleasesDashboard() {
  const qc = useQueryClient();
  const summaryQ = useQuery({
    queryKey: queryKeys.releases,
    queryFn: api.releases,
    refetchInterval: (q) =>
      q.state.data?.projects.some((p) => p.refreshing) ? POLL_REFRESHING_MS : POLL_MS,
  });
  const refreshMut = useMutation({
    mutationFn: api.refreshReleases,
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.releases }),
  });
  const [releasing, setReleasing] = useState<ReleaseTarget | null>(null);
  const [reminding, setReminding] = useState<{
    repo: string;
    prefix: string;
    reminder?: ReleaseReminder;
    bump?: ReleaseBump;
  } | null>(null);
  const [showUnreleased, setShowUnreleased] = useState(readShowUnreleased);
  const [openRun, setOpenRun] = useState<ReleaseRun | null>(null);
  const [onRunEnd] = useState(() => () => {
    qc.invalidateQueries({ queryKey: queryKeys.releases });
  });

  const summary = summaryQ.data;
  const reminders = summary?.reminders;
  // Reminders are what's about to be released, so have their previews ready.
  useEffect(() => {
    for (const r of reminders ?? []) {
      qc.prefetchQuery(previewOptions(r.repo_path, r.prefix, r.bump, r.suffix));
    }
  }, [qc, reminders]);
  if (summaryQ.error) {
    return <p className="p-4 text-sm text-danger">{String(summaryQ.error)}</p>;
  }
  if (!summary) return <p className="p-4 text-sm text-fg-muted">Loading…</p>;

  const subtreeOf = (r: ReleaseReminder) =>
    summary.projects
      .find((p) => p.repo_path === r.repo_path)
      ?.subtrees.find((s) => s.prefix === r.prefix);

  return (
    <div className="h-full overflow-y-auto [scrollbar-gutter:stable]">
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 px-3 py-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-sm font-semibold uppercase tracking-wide">Releases</h2>
          <Button
            variant="primary"
            size="lg"
            pending={refreshMut.isPending}
            idleLabel="refresh"
            pendingLabel="refreshing…"
            onClick={() => refreshMut.mutate()}
          />
        </div>
        {summary.unavailable && <p className="text-sm text-danger">{summary.unavailable}</p>}
        {refreshMut.error && <p className="text-sm text-danger">{String(refreshMut.error)}</p>}

        {openRun && <RunLog run={openRun} onEnd={onRunEnd} onClose={() => setOpenRun(null)} />}

        <section className="flex flex-col gap-2">
          <h3 className="text-sm font-medium">Reminders</h3>
          {summary.reminders.length === 0 ? (
            <p className="text-sm text-fg-muted">
              Nothing to remember. Choose &ldquo;remind me later&rdquo; when releasing a subtree.
            </p>
          ) : (
            <ul className="grid gap-2 md:grid-cols-2">
              {summary.reminders.map((r) => (
                <ReminderCard
                  key={r.id}
                  reminder={r}
                  subtree={subtreeOf(r)}
                  onEdit={() => setReminding({ repo: r.repo_path, prefix: r.prefix, reminder: r })}
                  onRelease={() =>
                    setReleasing({
                      repo: r.repo_path,
                      prefix: r.prefix,
                      bump: r.bump,
                      suffix: r.suffix,
                      reminderId: r.id,
                    })
                  }
                />
              ))}
            </ul>
          )}
        </section>

        {summary.projects.length === 0 && !summary.unavailable ? (
          <p className="text-sm text-fg-muted">
            No board's repo has a git-orchard manifest with subtrees.
          </p>
        ) : (
          summary.projects.map((p) => (
            <ProjectTable
              key={p.repo_path}
              project={p}
              showUnreleased={showUnreleased}
              onShowUnreleased={(show) => {
                setShowUnreleased(show);
                writeShowUnreleased(show);
              }}
              onRelease={(prefix) =>
                setReleasing({ repo: p.repo_path, prefix, bump: "patch", suffix: "" })
              }
            />
          ))
        )}

        {summary.runs.length > 0 && (
          <section className="flex flex-col gap-2">
            <h3 className="text-sm font-medium">Recent runs</h3>
            <RunList
              runs={summary.runs}
              onOpen={setOpenRun}
              onRetry={(run) =>
                setReleasing({
                  repo: run.repo_path,
                  prefix: run.prefix,
                  bump: run.bump,
                  suffix: run.suffix,
                  reminderId: run.reminder_id,
                })
              }
            />
          </section>
        )}

        {releasing && (
          <ReleaseModal
            target={releasing}
            onClose={() => setReleasing(null)}
            onRemind={
              releasing.reminderId === undefined &&
              !summary.reminders.some(
                (r) => r.repo_path === releasing.repo && r.prefix === releasing.prefix,
              )
                ? (bump) => {
                    setReleasing(null);
                    setReminding({ repo: releasing.repo, prefix: releasing.prefix, bump });
                  }
                : undefined
            }
            onStarted={(run) => {
              setReleasing(null);
              setOpenRun(run);
              qc.invalidateQueries({ queryKey: queryKeys.releases });
            }}
          />
        )}
        {reminding && <ReminderModal {...reminding} onClose={() => setReminding(null)} />}
      </div>
    </div>
  );
}
