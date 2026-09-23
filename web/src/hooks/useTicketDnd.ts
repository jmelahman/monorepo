import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  type DragEndEvent,
  type DragStartEvent,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import { useState } from "react";
import { api } from "@/api/client";
import { queryKeys } from "@/api/keys";
import { type BoardStructure, ticketStore } from "@/store";

// moveTicketIdInStructure returns a copy of structure with movedId removed from
// its current column and reinserted into targetCol at insertIndex. Pure; drives
// the optimistic cache update applied before the move mutation round-trips.
export function moveTicketIdInStructure(
  structure: BoardStructure,
  movedId: number,
  targetCol: number,
  insertIndex: number,
): BoardStructure {
  const ticketIdsByColumn: Record<number, number[]> = {};
  for (const [colKey, ids] of Object.entries(structure.ticketIdsByColumn)) {
    ticketIdsByColumn[Number(colKey)] = ids.filter((id) => id !== movedId);
  }
  const target = ticketIdsByColumn[targetCol] ?? [];
  target.splice(insertIndex, 0, movedId);
  ticketIdsByColumn[targetCol] = target;
  return { ...structure, ticketIdsByColumn };
}

// useTicketDnd encapsulates the ticket drag-and-drop wiring shared by the Board
// column view and the Overview board tree: drag state, the pointer sensor,
// optimistic cache reordering, and the move mutation. Callers render their own
// DndContext and DragOverlay and read draggingId for the overlay; this hook
// supplies the sensors and handlers to spread onto DndContext.
export function useTicketDnd(boardId: number, structure: BoardStructure | undefined) {
  const qc = useQueryClient();
  const [draggingId, setDraggingId] = useState<number | null>(null);
  // 5px activation distance lets click-to-open still fire on ticket buttons.
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }));

  const moveMut = useMutation({
    mutationFn: (input: { id: number; column_id: number; position: number }) =>
      api.moveTicket(input.id, { column_id: input.column_id, position: input.position }),
    onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.board(boardId) }),
  });

  function onDragStart(e: DragStartEvent) {
    setDraggingId(Number(e.active.id));
  }

  function onDragEnd(e: DragEndEvent) {
    setDraggingId(null);
    if (!structure) return;
    const ticketId = Number(e.active.id);
    const overId = e.over?.id;
    if (overId == null) return;
    const moved = ticketStore.get(ticketId);
    if (!moved) return;

    // over.id is either `col-N` (column droppable) or a ticket id (sortable item).
    const { ticketIdsByColumn } = structure;
    let targetCol: number;
    let insertIndex: number;
    if (typeof overId === "string" && overId.startsWith("col-")) {
      targetCol = Number(overId.slice(4));
      if (Number.isNaN(targetCol)) return;
      const list = ticketIdsByColumn[targetCol] ?? [];
      insertIndex = list.filter((id) => id !== ticketId).length;
    } else {
      const overTicketId = Number(overId);
      const overTicket = ticketStore.get(overTicketId);
      if (!overTicket) return;
      targetCol = overTicket.column_id;
      const targetList = (ticketIdsByColumn[targetCol] ?? []).filter((id) => id !== ticketId);
      const idx = targetList.indexOf(overTicketId);
      if (idx < 0) {
        insertIndex = targetList.length;
      } else if (moved.column_id === targetCol && moved.position < overTicket.position) {
        // Dragging downward within the same column: drop after the over ticket.
        insertIndex = idx + 1;
      } else {
        insertIndex = idx;
      }
    }

    if (moved.column_id === targetCol) {
      const sourceList = ticketIdsByColumn[targetCol] ?? [];
      if (sourceList.indexOf(ticketId) === insertIndex) return;
    }

    qc.setQueryData<BoardStructure>(queryKeys.board(boardId), (old) =>
      old ? moveTicketIdInStructure(old, ticketId, targetCol, insertIndex) : old,
    );
    moveMut.mutate({ id: ticketId, column_id: targetCol, position: insertIndex });
  }

  const onDragCancel = () => setDraggingId(null);

  return { draggingId, sensors, onDragStart, onDragEnd, onDragCancel };
}
