import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { DndContext, DragEndEvent, PointerSensor, useSensor, useSensors } from "@dnd-kit/core";
import { useState } from "react";
import { api } from "../api/client";
import { Column } from "./Column";
import { SessionPane } from "./SessionPane";

export function Board({ boardId }: { boardId: number }) {
  const qc = useQueryClient();
  const stateQ = useQuery({ queryKey: ["board", boardId], queryFn: () => api.boardState(boardId) });
  const [activeTicket, setActiveTicket] = useState<number | null>(null);
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }));

  const moveMut = useMutation({
    mutationFn: (input: { id: number; column_id: number; position: number }) =>
      api.moveTicket(input.id, { column_id: input.column_id, position: input.position }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board", boardId] }),
  });

  if (stateQ.isLoading) return <p className="p-4 text-sm text-zinc-400">Loading…</p>;
  if (!stateQ.data) return <p className="p-4 text-sm text-red-400">No data.</p>;

  const { columns, tickets, sessions } = stateQ.data;
  const sessionByTicket = new Map<number, (typeof sessions)[number]>(sessions.map((s) => [s.ticket_id, s]));

  function onDragEnd(e: DragEndEvent) {
    const ticketId = Number(e.active.id);
    const overId = e.over?.id;
    if (overId == null) return;
    const targetCol = Number(String(overId).replace(/^col-/, ""));
    if (Number.isNaN(targetCol)) return;
    const target = tickets.filter((t) => t.column_id === targetCol);
    moveMut.mutate({ id: ticketId, column_id: targetCol, position: target.length });
  }

  return (
    <div className="flex h-full">
      <DndContext sensors={sensors} onDragEnd={onDragEnd}>
        <div className="flex flex-1 gap-2 overflow-x-auto p-3">
          {columns.map((c) => (
            <Column
              key={c.id}
              column={c}
              tickets={tickets.filter((t) => t.column_id === c.id)}
              sessions={sessionByTicket}
              boardId={boardId}
              onSelect={setActiveTicket}
              activeTicket={activeTicket}
            />
          ))}
        </div>
      </DndContext>
      <SessionPane
        boardId={boardId}
        ticketId={activeTicket}
        session={activeTicket != null ? sessionByTicket.get(activeTicket) ?? null : null}
        onClose={() => setActiveTicket(null)}
      />
    </div>
  );
}
