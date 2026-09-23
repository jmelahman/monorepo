import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { api, type Session } from "@/api/client";
import { queryKeys } from "@/api/keys";
import { useToast } from "@/toast";
import { useCopyFeedback } from "@/hooks/useCopyFeedback";
import { CheckIcon, CopyIcon } from "@/icons";
import { Button } from "./Button";

export function TasksPanel({ session }: { session: Session; boardId: number }) {
  const qc = useQueryClient();
  const toast = useToast();
  const tasksQ = useQuery({
    queryKey: queryKeys.tasks(session.id),
    queryFn: () => api.discoverTasks(session.id),
  });
  const runsQ = useQuery({
    queryKey: queryKeys.runs(session.id),
    queryFn: () => api.listTaskRuns(session.id),
    refetchInterval: 2000,
  });
  const portsQ = useQuery({
    queryKey: queryKeys.ports(session.id),
    queryFn: () => api.listPorts(session.id),
    refetchInterval: 2000,
  });

  const [openOutputId, setOpenOutputId] = useState<number | null>(null);
  const [outputLines, setOutputLines] = useState<string[]>([]);
  const { copied, markCopied, reset: resetCopied } = useCopyFeedback(1500);

  useEffect(() => {
    resetCopied();
    setOutputLines([]);
    if (openOutputId === null) return;
    const es = new EventSource(`/api/task-runs/${openOutputId}/output`);
    es.onmessage = (e) => setOutputLines((prev) => [...prev.slice(-500), e.data]);
    es.addEventListener("end", () => es.close());
    return () => es.close();
  }, [openOutputId, resetCopied]);

  const onCopy = () => {
    navigator.clipboard.writeText(outputLines.join("\n"));
    markCopied();
  };

  const startMut = useMutation({
    mutationFn: (label: string) => api.startTaskRun(session.id, label),
    onSuccess: (run) => {
      setOpenOutputId(run.id);
      qc.invalidateQueries({ queryKey: queryKeys.runs(session.id) });
      qc.invalidateQueries({ queryKey: queryKeys.ports(session.id) });
    },
  });
  const stopMut = useMutation({
    mutationFn: (id: number) => api.stopTaskRun(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.runs(session.id) }),
  });

  const warnings = tasksQ.data?.warnings ?? [];
  const lastWarningKey = useRef<string>("");
  useEffect(() => {
    const key = warnings.join("\n");
    if (key && key !== lastWarningKey.current) {
      lastWarningKey.current = key;
      for (const w of warnings) toast.push("error", w);
    }
  }, [warnings, toast]);

  if (session.status === "stopped") {
    return <p className="p-4 text-sm text-fg-muted">Start the session to discover tasks.</p>;
  }

  const tasks = tasksQ.data?.tasks ?? [];
  const runs = runsQ.data ?? [];
  const ports = portsQ.data ?? [];
  const portByContainer = new Map(ports.map((p) => [p.container_port, p]));

  return (
    <div className="flex h-full flex-col gap-3 overflow-y-auto p-3 text-sm [scrollbar-gutter:stable]">
      <section>
        <h3 className="mb-2 text-xs uppercase tracking-wide text-fg-muted">Detected tasks</h3>
        {tasks.length === 0 && (
          <p className="text-fg-muted">No .vscode/tasks.json or launch.json detected.</p>
        )}
        <ul className="flex flex-col gap-1">
          {tasks.map((t) => {
            const port = t.has_port ? portByContainer.get(t.container_port!) : undefined;
            return (
              <li
                key={t.label}
                className="flex items-center justify-between rounded bg-surface px-2 py-1"
              >
                <div>
                  <div className="font-medium">{t.label}</div>
                  <div className="text-xs text-fg-muted">
                    {t.command} {t.args?.join(" ")}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {t.has_port && (
                    <span className="text-xs text-fg-muted">
                      :{t.container_port}
                      {port && (
                        <a
                          className="ml-1 text-accent-500"
                          href={`http://localhost:${port.host_port}`}
                          target="_blank"
                          rel="noreferrer"
                        >
                          → :{port.host_port}
                        </a>
                      )}
                    </span>
                  )}
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => startMut.mutate(t.label)}
                    pending={startMut.isPending && startMut.variables === t.label}
                    idleLabel="run"
                    pendingLabel="starting…"
                  />
                </div>
              </li>
            );
          })}
        </ul>
      </section>
      <section>
        <h3 className="mb-2 text-xs uppercase tracking-wide text-fg-muted">Runs</h3>
        <ul className="flex flex-col gap-1">
          {runs.map((r) => (
            <li key={r.id} className="rounded bg-surface p-2">
              <div className="flex items-center justify-between">
                <div>
                  <div className="font-medium">{r.task_label}</div>
                  <div className="text-xs text-fg-muted">
                    {r.status}
                    {r.exit_code != null ? ` (exit ${r.exit_code})` : ""}
                  </div>
                </div>
                <div className="flex gap-2">
                  {openOutputId === r.id && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={onCopy}
                      aria-label={copied ? "copied" : "copy output"}
                      title={copied ? "copied" : "copy output"}
                    >
                      {copied ? <CheckIcon /> : <CopyIcon />}
                    </Button>
                  )}
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setOpenOutputId(openOutputId === r.id ? null : r.id)}
                  >
                    {openOutputId === r.id ? "hide" : "output"}
                  </Button>
                  {r.status === "running" && (
                    <Button
                      variant="neutral"
                      size="sm"
                      onClick={() => stopMut.mutate(r.id)}
                      pending={stopMut.isPending && stopMut.variables === r.id}
                      idleLabel="stop"
                      pendingLabel="stopping…"
                    />
                  )}
                </div>
              </div>
              {openOutputId === r.id && <TaskOutput lines={outputLines} />}
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

function TaskOutput({ lines }: { lines: string[] }) {
  const ref = useRef<HTMLPreElement>(null);

  // `lines` isn't referenced inside, but every append produces a new array
  // identity, so listing it as a dep is what triggers the autoscroll.
  // biome-ignore lint/correctness/useExhaustiveDependencies: see above.
  useEffect(() => {
    ref.current?.scrollTo({ top: ref.current.scrollHeight });
  }, [lines]);

  return (
    <pre
      ref={ref}
      className="mt-2 max-h-64 overflow-y-auto rounded bg-bg p-2 text-xs leading-tight text-fg [scrollbar-gutter:stable]"
    >
      {lines.join("\n")}
    </pre>
  );
}
