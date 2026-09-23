import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useDroppable } from "@dnd-kit/core";
import { SortableContext, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { useEffect, useRef, useState } from "react";
import { api, type Column as ColumnType } from "@/api/client";
import { queryKeys } from "@/api/keys";
import { addTicketRequestStore, useScalarSelector } from "@/store";
import { useToast } from "@/toast";
import { Button } from "./Button";
import { ConfirmModal } from "./Modal";
import { Ticket } from "./Ticket";

export function Column(props: {
  column: ColumnType;
  ticketIds: number[];
  sessionIdByTicket: Record<number, number>;
  boardId: number;
}) {
  const qc = useQueryClient();
  const { push } = useToast();
  const { setNodeRef, isOver } = useDroppable({ id: `col-${props.column.id}` });
  const [adding, setAdding] = useState(false);
  const [title, setTitle] = useState("");
  const [confirmArchive, setConfirmArchive] = useState(false);
  const addInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (adding) addInputRef.current?.focus();
  }, [adding]);

  const createMut = useMutation({
    mutationFn: () => api.createTicket(props.boardId, { column_id: props.column.id, title }),
    onSuccess: () => {
      setTitle("");
      setAdding(false);
      qc.invalidateQueries({ queryKey: queryKeys.board(props.boardId) });
    },
  });

  const archiveAllMut = useMutation({
    mutationFn: async () => {
      const count = props.ticketIds.length;
      await api.archiveColumnTickets(props.column.id);
      return count;
    },
    onSuccess: (count) => {
      qc.invalidateQueries({ queryKey: queryKeys.board(props.boardId) });
      qc.invalidateQueries({ queryKey: queryKeys.archived(props.boardId) });
      setConfirmArchive(false);
      push("success", `Archived ${count} ticket${count === 1 ? "" : "s"}.`);
    },
  });

  const requested = useScalarSelector(addTicketRequestStore, (id) => id === props.column.id);
  useEffect(() => {
    if (!requested) return;
    setAdding(true);
    addTicketRequestStore.set(null);
  }, [requested]);

  const ticketCount = props.ticketIds.length;

  return (
    <div
      ref={setNodeRef}
      className={`flex h-full min-w-72 flex-1 flex-col gap-2 overflow-hidden rounded border border-border bg-surface p-2 ${isOver ? "ring-2 ring-accent-600" : ""}`}
    >
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-fg">
          {props.column.name}
        </h2>
        <div className="flex items-center gap-2">
          {ticketCount > 0 && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setConfirmArchive(true)}
              title={`Archive all ${ticketCount} ticket${ticketCount === 1 ? "" : "s"}`}
            >
              archive all
            </Button>
          )}
          <span className="text-xs text-fg-muted">{ticketCount}</span>
        </div>
      </div>
      <div className="-mx-0.5 flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-0.5 py-0.5">
        <SortableContext items={props.ticketIds} strategy={verticalListSortingStrategy}>
          {props.ticketIds.map((id) => (
            <Ticket key={id} id={id} sessionId={props.sessionIdByTicket[id] ?? null} />
          ))}
        </SortableContext>
      </div>
      {adding ? (
        <form
          data-ticket-add
          onSubmit={(e) => {
            e.preventDefault();
            if (title.trim() && !createMut.isPending) createMut.mutate();
          }}
          className="flex flex-col gap-1"
        >
          <input
            ref={addInputRef}
            className="rounded bg-surface-2 px-2 py-1 text-sm"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="ticket title"
            disabled={createMut.isPending}
          />
          <div className="flex gap-2 text-xs">
            <Button
              variant="primary"
              type="submit"
              pending={createMut.isPending}
              idleLabel="add"
              pendingLabel="adding…"
            />
            <Button
              variant="ghost"
              type="button"
              onClick={() => setAdding(false)}
              disabled={createMut.isPending}
            >
              cancel
            </Button>
          </div>
        </form>
      ) : (
        <Button
          data-ticket-add
          variant="dashed"
          className="text-xs"
          onClick={() => setAdding(true)}
        >
          + add ticket
        </Button>
      )}
      <ConfirmModal
        open={confirmArchive}
        onClose={() => setConfirmArchive(false)}
        title={`Archive all in ${props.column.name}?`}
        description={
          <>
            Archive all <span className="font-medium">{ticketCount}</span> ticket
            {ticketCount === 1 ? "" : "s"} in{" "}
            <span className="font-medium">{props.column.name}</span>? Their sessions will be
            stopped.
          </>
        }
        consequence="Tickets can be restored from the archive."
        onConfirm={() => archiveAllMut.mutate()}
        confirmLabel="archive all"
        confirmPendingLabel="archiving…"
        confirmVariant="primary"
        pending={archiveAllMut.isPending}
      />
    </div>
  );
}
