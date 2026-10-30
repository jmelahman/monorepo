import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  api,
  type ReleaseBump,
  type ReleaseProject,
  type ReleaseReminder,
  type ReleaseRun,
  type ReleaseSubtree,
} from "@/api/client";
import { queryKeys } from "@/api/keys";
import { Button } from "./Button";
import { Modal } from "./Modal";

// The releases dashboard: notes on what's meant to be released, each git-orchard
// subtree's state, and a confirmed release through the same code as
// `git orchard release`. Reminders track nothing and never fire by themselves.

const BUMPS: ReleaseBump[] = ["patch", "minor", "major"];

// The summary is cheap and only changes on a refresh or a run.
const POLL_MS = 15_000;
const POLL_REFRESHING_MS = 3_000;

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

function unreleasedLabel(s: ReleaseSubtree | undefined) {
  if (!s) return "";
  if (s.error) return "state unknown";
  if (!s.counted) return "unreleased commits unknown";
  return `${s.unreleased} unreleased commit${s.unreleased === 1 ? "" : "s"}`;
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
}: {
  target: ReleaseTarget;
  onClose: () => void;
  onStarted: (run: ReleaseRun) => void;
}) {
  const [bump, setBump] = useState(target.bump);
  const previewQ = useQuery({
    queryKey: queryKeys.releasePreview(target.repo, target.prefix, bump, target.suffix),
    queryFn: () => api.previewRelease(target.repo, target.prefix, bump, target.suffix),
    gcTime: 0,
    staleTime: 0,
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
        {previewQ.isFetching ? (
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
          <Button variant="neutral" size="lg" onClick={onClose} disabled={runMut.isPending}>
            cancel
          </Button>
          <Button
            variant="primary"
            size="lg"
            disabled={!preview || !!preview.blocked || previewQ.isFetching}
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
  onClose,
}: {
  repo: string;
  prefix: string;
  reminder?: ReleaseReminder;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [bump, setBump] = useState<ReleaseBump>(reminder?.bump ?? "patch");
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
            // biome-ignore lint/a11y/noAutofocus: the note is why the modal opened
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
        <span className="font-medium">{reminder.prefix}</span>
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
        <Button variant="primary" size="sm" onClick={onRelease}>
          release…
        </Button>
      </div>
    </li>
  );
}

function ProjectTable({
  project,
  reminders,
  onRelease,
  onRemind,
}: {
  project: ReleaseProject;
  reminders: ReleaseReminder[];
  onRelease: (prefix: string) => void;
  onRemind: (prefix: string) => void;
}) {
  return (
    <section className="flex flex-col gap-2">
      <h3 className="text-sm font-medium">
        {project.repo_path}{" "}
        <span className="font-normal text-fg-muted">
          {project.base}
          {project.refreshing && " · refreshing…"}
        </span>
      </h3>
      {project.error && <p className="text-sm text-danger">{project.error}</p>}
      <div className="overflow-x-auto rounded border border-border">
        <table className="w-full text-left text-sm">
          <thead className="text-xs text-fg-muted">
            <tr>
              <th className="px-3 py-2 font-normal">subtree</th>
              <th className="px-3 py-2 font-normal">ahead / behind</th>
              <th className="px-3 py-2 font-normal">latest</th>
              <th className="px-3 py-2 font-normal">unreleased</th>
              <th className="px-3 py-2" />
            </tr>
          </thead>
          <tbody>
            {project.subtrees.map((s) => (
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
                <td className="px-3 py-2 text-right whitespace-nowrap">
                  {!reminders.some((r) => r.prefix === s.prefix) && (
                    <Button variant="ghost" size="sm" onClick={() => onRemind(s.prefix)}>
                      remind
                    </Button>
                  )}
                  <Button variant="ghost" size="sm" onClick={() => onRelease(s.prefix)}>
                    release…
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
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
  } | null>(null);
  const [openRun, setOpenRun] = useState<ReleaseRun | null>(null);
  const [onRunEnd] = useState(() => () => {
    qc.invalidateQueries({ queryKey: queryKeys.releases });
  });

  const summary = summaryQ.data;
  if (summaryQ.error) {
    return <p className="p-4 text-sm text-danger">{String(summaryQ.error)}</p>;
  }
  if (!summary) return <p className="p-4 text-sm text-fg-muted">Loading…</p>;

  const remindersIn = (repo: string) => summary.reminders.filter((r) => r.repo_path === repo);
  const subtreeOf = (r: ReleaseReminder) =>
    summary.projects
      .find((p) => p.repo_path === r.repo_path)
      ?.subtrees.find((s) => s.prefix === r.prefix);

  return (
    <div className="flex h-full min-h-0 flex-col gap-6 overflow-auto p-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-lg font-semibold">Releases</h2>
        <Button
          variant="neutral"
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
            Nothing to remember. Add a reminder from a subtree below.
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
            reminders={remindersIn(p.repo_path)}
            onRemind={(prefix) => setReminding({ repo: p.repo_path, prefix })}
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
          onStarted={(run) => {
            setReleasing(null);
            setOpenRun(run);
            qc.invalidateQueries({ queryKey: queryKeys.releases });
          }}
        />
      )}
      {reminding && <ReminderModal {...reminding} onClose={() => setReminding(null)} />}
    </div>
  );
}
