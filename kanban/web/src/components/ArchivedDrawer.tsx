import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { api, Ticket } from "../api/client";
import { useToast } from "../toast";

export function ArchivedDrawer({ boardId, onClose }: { boardId: number; onClose: () => void }) {
  const qc = useQueryClient();
  const { push } = useToast();
  const archivedQ = useQuery({
    queryKey: ["archived", boardId],
    queryFn: () => api.listArchivedTickets(boardId),
  });

  const deleteMut = useMutation({
    mutationFn: (id: number) => api.deleteTicket(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["archived", boardId] });
      qc.invalidateQueries({ queryKey: ["board", boardId] });
      push("success", "Ticket and its resources deleted.");
    },
  });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-40 flex" role="dialog" aria-modal="true">
      <div className="flex-1 bg-black/50" onClick={onClose} />
      <aside className="flex w-[480px] flex-col border-l border-zinc-800 bg-zinc-950">
        <header className="flex items-center justify-between border-b border-zinc-800 px-4 py-2">
          <h2 className="text-sm font-semibold">Archived tickets</h2>
          <button className="text-zinc-400 hover:text-zinc-100" onClick={onClose} aria-label="Close">
            ✕
          </button>
        </header>
        <div className="flex-1 overflow-y-auto p-3">
          {archivedQ.isLoading && <p className="text-sm text-zinc-400">Loading…</p>}
          {archivedQ.data && archivedQ.data.length === 0 && (
            <p className="text-sm text-zinc-400">No archived tickets.</p>
          )}
          <ul className="flex flex-col gap-2">
            {(archivedQ.data ?? []).map((t) => (
              <ArchivedRow
                key={t.id}
                ticket={t}
                pending={deleteMut.isPending && deleteMut.variables === t.id}
                onDelete={() => {
                  if (
                    window.confirm(
                      `Permanently delete "${t.title}"?\n\nThis stops the container, removes the worktree, and deletes the branch.`,
                    )
                  ) {
                    deleteMut.mutate(t.id);
                  }
                }}
              />
            ))}
          </ul>
        </div>
      </aside>
    </div>
  );
}

function ArchivedRow({
  ticket,
  pending,
  onDelete,
}: {
  ticket: Ticket;
  pending: boolean;
  onDelete: () => void;
}) {
  const archivedAt = ticket.archived_at ? new Date(ticket.archived_at * 1000).toLocaleString() : "";
  return (
    <li className="rounded border border-zinc-800 bg-zinc-900 p-2 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="font-medium">{ticket.title}</div>
          <div className="truncate font-mono text-xs text-zinc-500">{ticket.slug}</div>
          {archivedAt && <div className="text-xs text-zinc-500">archived {archivedAt}</div>}
        </div>
        <button
          className="rounded bg-red-900/60 px-2 py-1 text-xs text-red-100 hover:bg-red-800 disabled:opacity-50"
          onClick={onDelete}
          disabled={pending}
        >
          {pending ? "deleting…" : "delete"}
        </button>
      </div>
    </li>
  );
}
