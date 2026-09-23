import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Ticket } from "@/api/client";
import { queryKeys } from "@/api/keys";
import { useToast } from "@/toast";
import { Button } from "./Button";
import { ConfirmModal, Drawer } from "./Modal";

export function ArchivedDrawer({
  open,
  boardId,
  onClose,
}: {
  open: boolean;
  boardId: number;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const { push } = useToast();
  const [confirmTicket, setConfirmTicket] = useState<Ticket | null>(null);
  const [confirmDeleteAll, setConfirmDeleteAll] = useState(false);
  const archivedQ = useQuery({
    queryKey: queryKeys.archived(boardId),
    queryFn: () => api.listArchivedTickets(boardId),
    enabled: open,
  });

  const invalidateBoard = () => {
    qc.invalidateQueries({ queryKey: queryKeys.archived(boardId) });
    qc.invalidateQueries({ queryKey: queryKeys.board(boardId) });
  };

  const deleteMut = useMutation({
    mutationFn: (id: number) => api.deleteTicket(id),
    onSuccess: () => {
      invalidateBoard();
      push("success", "Ticket and its resources deleted.");
      setConfirmTicket(null);
    },
  });

  const deleteAllMut = useMutation({
    mutationFn: async () => {
      const count = archivedQ.data?.length ?? 0;
      await api.deleteAllArchived(boardId);
      return count;
    },
    onSuccess: (count) => {
      invalidateBoard();
      push("success", `Deleted ${count} archived ticket${count === 1 ? "" : "s"}.`);
      setConfirmDeleteAll(false);
    },
  });

  const unarchiveMut = useMutation({
    mutationFn: (id: number) => api.unarchiveTicket(id),
    onSuccess: () => {
      invalidateBoard();
      push("success", "Ticket unarchived.");
    },
  });

  const confirmPending =
    confirmTicket !== null && deleteMut.isPending && deleteMut.variables === confirmTicket.id;
  const archivedCount = archivedQ.data?.length ?? 0;

  return (
    <>
      <Drawer open={open} onClose={onClose} title="Archived tickets">
        {archivedCount > 0 && (
          <div className="flex items-center justify-between border-b border-border px-3 py-2">
            <span className="text-xs text-fg-muted">{archivedCount} archived</span>
            <Button variant="danger" size="sm" onClick={() => setConfirmDeleteAll(true)}>
              delete all
            </Button>
          </div>
        )}
        <div className="flex-1 overflow-y-auto p-3">
          {archivedQ.isLoading && <p className="text-sm text-fg-muted">Loading…</p>}
          {archivedQ.data && archivedQ.data.length === 0 && (
            <p className="text-sm text-fg-muted">No archived tickets.</p>
          )}
          <ul className="flex flex-col gap-2">
            {(archivedQ.data ?? []).map((t) => (
              <ArchivedRow
                key={t.id}
                ticket={t}
                deletePending={deleteMut.isPending && deleteMut.variables === t.id}
                unarchivePending={unarchiveMut.isPending && unarchiveMut.variables === t.id}
                onUnarchive={() => unarchiveMut.mutate(t.id)}
                onDelete={() => setConfirmTicket(t)}
              />
            ))}
          </ul>
        </div>
      </Drawer>
      <ConfirmModal
        open={confirmTicket !== null}
        onClose={() => setConfirmTicket(null)}
        title="Permanently delete ticket?"
        description={
          <>
            Permanently delete <span className="font-medium">"{confirmTicket?.title}"</span>?
          </>
        }
        consequence="This stops the container, removes the worktree, and deletes the branch."
        onConfirm={() => confirmTicket && deleteMut.mutate(confirmTicket.id)}
        confirmLabel="delete"
        confirmPendingLabel="deleting…"
        pending={confirmPending}
      />
      <ConfirmModal
        open={confirmDeleteAll}
        onClose={() => setConfirmDeleteAll(false)}
        title="Permanently delete all archived?"
        description={
          <>
            Permanently delete all <span className="font-medium">{archivedCount}</span> archived
            ticket
            {archivedCount === 1 ? "" : "s"}?
          </>
        }
        consequence="This stops their containers, removes worktrees, and deletes branches. Cannot be undone."
        onConfirm={() => deleteAllMut.mutate()}
        confirmLabel="delete all"
        confirmPendingLabel="deleting…"
        pending={deleteAllMut.isPending}
      />
    </>
  );
}

function ArchivedRow({
  ticket,
  deletePending,
  unarchivePending,
  onDelete,
  onUnarchive,
}: {
  ticket: Ticket;
  deletePending: boolean;
  unarchivePending: boolean;
  onDelete: () => void;
  onUnarchive: () => void;
}) {
  const archivedAt = ticket.archived_at ? new Date(ticket.archived_at * 1000).toLocaleString() : "";
  const busy = deletePending || unarchivePending;
  return (
    <li className="rounded border border-border bg-surface p-2 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="font-medium">{ticket.title}</div>
          <div className="truncate font-mono text-xs text-fg-muted">{ticket.slug}</div>
          {archivedAt && <div className="text-xs text-fg-muted">archived {archivedAt}</div>}
        </div>
        <div className="flex shrink-0 gap-1">
          <Button
            variant="neutral"
            size="sm"
            onClick={onUnarchive}
            disabled={busy}
            pending={unarchivePending}
            idleLabel="unarchive"
            pendingLabel="unarchiving…"
          />
          <Button
            variant="danger"
            size="sm"
            onClick={onDelete}
            disabled={busy}
            pending={deletePending}
            idleLabel="delete"
            pendingLabel="deleting…"
          />
        </div>
      </div>
    </li>
  );
}
