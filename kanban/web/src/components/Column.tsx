import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useDroppable } from "@dnd-kit/core";
import { useState } from "react";
import { api, Column as ColumnType, Session, Ticket as TicketType } from "../api/client";
import { Ticket } from "./Ticket";

export function Column(props: {
  column: ColumnType;
  tickets: TicketType[];
  sessions: Map<number, Session>;
  boardId: number;
  activeTicket: number | null;
  onSelect: (id: number) => void;
}) {
  const qc = useQueryClient();
  const { setNodeRef, isOver } = useDroppable({ id: `col-${props.column.id}` });
  const [adding, setAdding] = useState(false);
  const [title, setTitle] = useState("");

  const createMut = useMutation({
    mutationFn: () => api.createTicket(props.boardId, { column_id: props.column.id, title }),
    onSuccess: () => {
      setTitle("");
      setAdding(false);
      qc.invalidateQueries({ queryKey: ["board", props.boardId] });
    },
  });

  return (
    <div
      ref={setNodeRef}
      className={`flex w-72 shrink-0 flex-col gap-2 rounded border border-zinc-800 bg-zinc-900 p-2 ${isOver ? "ring-2 ring-red-600" : ""}`}
    >
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-zinc-300">{props.column.name}</h2>
        <span className="text-xs text-zinc-500">{props.tickets.length}</span>
      </div>
      <div className="flex flex-col gap-2">
        {props.tickets
          .sort((a, b) => a.position - b.position)
          .map((t) => (
            <Ticket
              key={t.id}
              ticket={t}
              session={props.sessions.get(t.id) ?? null}
              active={props.activeTicket === t.id}
              onSelect={() => props.onSelect(t.id)}
            />
          ))}
      </div>
      {adding ? (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (title.trim()) createMut.mutate();
          }}
          className="flex flex-col gap-1"
        >
          <input
            autoFocus
            className="rounded bg-zinc-800 px-2 py-1 text-sm"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="ticket title"
          />
          <div className="flex gap-2 text-xs">
            <button className="rounded bg-red-700 px-2 py-1" type="submit">
              add
            </button>
            <button className="text-zinc-400" type="button" onClick={() => setAdding(false)}>
              cancel
            </button>
          </div>
        </form>
      ) : (
        <button className="rounded border border-dashed border-zinc-700 py-1 text-xs text-zinc-400 hover:bg-zinc-800" onClick={() => setAdding(true)}>
          + add ticket
        </button>
      )}
    </div>
  );
}
