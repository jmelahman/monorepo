import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, Session } from "../api/client";
import { PtyTerminal } from "./PtyTerminal";
import { TasksPanel } from "./TasksPanel";

export function SessionPane({
  boardId,
  ticketId,
  session,
  onClose,
}: {
  boardId: number;
  ticketId: number | null;
  session: Session | null;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [tab, setTab] = useState<"terminal" | "tasks">("terminal");

  const ensureMut = useMutation({
    mutationFn: () => api.ensureSession(ticketId!),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board", boardId] }),
  });
  const startMut = useMutation({
    mutationFn: () => api.startSession(session!.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board", boardId] }),
  });
  const stopMut = useMutation({
    mutationFn: () => api.stopSession(session!.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board", boardId] }),
  });
  const archiveMut = useMutation({
    mutationFn: () => api.archiveTicket(ticketId!),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["board", boardId] });
      onClose();
    },
  });

  if (ticketId == null) return null;
  const isRunning = session?.status && !["stopped", "error"].includes(session.status);

  return (
    <aside className="flex w-[640px] flex-col border-l border-zinc-800 bg-zinc-950">
      <div className="flex items-center gap-2 border-b border-zinc-800 px-3 py-2 text-sm">
        <span className="font-medium">Ticket #{ticketId}</span>
        <span className="text-zinc-400">{session?.branch_name}</span>
        <div className="ml-auto flex gap-2">
          {!session && (
            <button className="rounded bg-red-700 px-2 py-1" onClick={() => ensureMut.mutate()} disabled={ensureMut.isPending}>
              create session
            </button>
          )}
          {session && session.status === "stopped" && (
            <button className="rounded bg-red-700 px-2 py-1" onClick={() => startMut.mutate()} disabled={startMut.isPending}>
              start
            </button>
          )}
          {session && isRunning && (
            <button className="rounded bg-zinc-700 px-2 py-1" onClick={() => stopMut.mutate()} disabled={stopMut.isPending}>
              stop
            </button>
          )}
          <button className="rounded bg-zinc-800 px-2 py-1 text-zinc-300" onClick={() => archiveMut.mutate()}>
            archive
          </button>
          <button className="text-zinc-400" onClick={onClose}>
            ✕
          </button>
        </div>
      </div>
      <div className="flex border-b border-zinc-800 text-sm">
        <Tab active={tab === "terminal"} onClick={() => setTab("terminal")} label="terminal" />
        <Tab active={tab === "tasks"} onClick={() => setTab("tasks")} label="tasks" />
      </div>
      <div className="min-h-0 flex-1">
        {tab === "terminal" && (
          <div className="h-full">
            {session && isRunning ? (
              <PtyTerminal sessionId={session.id} />
            ) : (
              <p className="p-4 text-sm text-zinc-400">Start the session to attach a terminal.</p>
            )}
          </div>
        )}
        {tab === "tasks" && session && <TasksPanel session={session} boardId={boardId} />}
      </div>
    </aside>
  );
}

function Tab({ active, onClick, label }: { active: boolean; onClick: () => void; label: string }) {
  return (
    <button
      onClick={onClick}
      className={`px-3 py-2 ${active ? "border-b-2 border-red-500 text-zinc-100" : "text-zinc-400 hover:text-zinc-200"}`}
    >
      {label}
    </button>
  );
}
