import { useQuery } from "@tanstack/react-query";
import { DndContext, DragOverlay } from "@dnd-kit/core";
import { useCallback } from "react";
import { queryKeys } from "@/api/keys";
import { useTerminalOrientation } from "@/hooks/useTerminalOrientation";
import { useTicketDnd } from "@/hooks/useTicketDnd";
import { useShortcut } from "@/keys/useShortcut";
import {
  activeTicketStore,
  addTicketRequestStore,
  type BoardStructure,
  fetchBoardStructure,
  useSession,
  useTicket,
} from "@/store";
import { Column } from "./Column";
import { SessionPane } from "./SessionPane";
import { TicketDragPreview } from "./Ticket";

type Direction = "up" | "down" | "left" | "right";

function nextTicketId(
  direction: Direction,
  activeId: number | null,
  structure: BoardStructure,
): number | null {
  const { columns, ticketIdsByColumn } = structure;
  if (columns.length === 0) return activeId;
  if (activeId == null) {
    for (const c of columns) {
      const list = ticketIdsByColumn[c.id] ?? [];
      if (list.length > 0) return list[0];
    }
    return null;
  }
  const colIdx = columns.findIndex((c) => (ticketIdsByColumn[c.id] ?? []).includes(activeId));
  if (colIdx === -1) return activeId;
  const list = ticketIdsByColumn[columns[colIdx].id] ?? [];
  const pos = list.indexOf(activeId);
  if (direction === "up" || direction === "down") {
    const next = list[pos + (direction === "down" ? 1 : -1)];
    return next ?? activeId;
  }
  const delta = direction === "right" ? 1 : -1;
  for (let i = colIdx + delta; i >= 0 && i < columns.length; i += delta) {
    const candidates = ticketIdsByColumn[columns[i].id] ?? [];
    if (candidates.length === 0) continue;
    const targetIdx = Math.min(pos, candidates.length - 1);
    return candidates[targetIdx];
  }
  return activeId;
}

export function Board({ boardId }: { boardId: number }) {
  const stateQ = useQuery({
    queryKey: queryKeys.board(boardId),
    queryFn: () => fetchBoardStructure(boardId),
  });
  const orientation = useTerminalOrientation();

  const structure = stateQ.data;
  const { draggingId, sensors, onDragStart, onDragEnd, onDragCancel } = useTicketDnd(
    boardId,
    structure,
  );
  const moveSelection = useCallback(
    (direction: Direction) => {
      if (!structure) return;
      const next = nextTicketId(direction, activeTicketStore.get(), structure);
      if (next != null) activeTicketStore.set(next);
    },
    [structure],
  );
  useShortcut("ticket.prev", () => moveSelection("up"));
  useShortcut("ticket.next", () => moveSelection("down"));
  useShortcut("column.prev", () => moveSelection("left"));
  useShortcut("column.next", () => moveSelection("right"));
  useShortcut("ticket.create", () => {
    if (!structure) return;
    const { columns, ticketIdsByColumn } = structure;
    if (columns.length === 0) return;
    const activeId = activeTicketStore.get();
    const activeColIdx =
      activeId == null
        ? -1
        : columns.findIndex((c) => (ticketIdsByColumn[c.id] ?? []).includes(activeId));
    const target = columns[activeColIdx >= 0 ? activeColIdx : 0];
    addTicketRequestStore.set(target.id);
  });

  if (stateQ.isLoading) return <p className="p-4 text-sm text-fg-muted">Loading…</p>;
  if (!structure) return <p className="p-4 text-sm text-danger">No data.</p>;

  const { board, columns, ticketIdsByColumn, sessionIdByTicket, merge_config, sync_config } =
    structure;

  return (
    <div
      className={`flex h-full min-w-0 ${orientation === "horizontal" ? "flex-col" : "flex-row"}`}
    >
      <DndContext
        sensors={sensors}
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
        onDragCancel={onDragCancel}
      >
        <div data-board-area className="flex min-h-0 min-w-0 flex-1 gap-2 overflow-x-auto p-3">
          {columns.map((c) => (
            <Column
              key={c.id}
              column={c}
              ticketIds={ticketIdsByColumn[c.id] ?? []}
              sessionIdByTicket={sessionIdByTicket}
              boardId={boardId}
            />
          ))}
        </div>
        <DragOverlay>
          {draggingId != null ? (
            <DraggingPreview id={draggingId} sessionId={sessionIdByTicket[draggingId] ?? null} />
          ) : null}
        </DragOverlay>
      </DndContext>
      <SessionPane
        boardId={boardId}
        baseBranch={board.base_branch}
        mergeConfig={merge_config}
        syncConfig={sync_config}
        sessionIdByTicket={sessionIdByTicket}
        orientation={orientation}
      />
    </div>
  );
}

function DraggingPreview({ id, sessionId }: { id: number; sessionId: number | null }) {
  const ticket = useTicket(id);
  const session = useSession(sessionId);
  if (!ticket) return null;
  return (
    <TicketDragPreview
      ticket={ticket}
      session={session ?? null}
      active={activeTicketStore.get() === id}
    />
  );
}
