import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  closestCenter,
  closestCorners,
  DndContext,
  type DragEndEvent,
  DragOverlay,
  PointerSensor,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { api, type Board } from "@/api/client";
import { queryKeys } from "@/api/keys";
import { useBoardSubscription } from "@/hooks/useBoardSubscription";
import { useTicketDnd } from "@/hooks/useTicketDnd";
import { fetchBoardStructure, sessionStore, useSession, useTicket } from "@/store";
import type { BoardStructure } from "@/store";
import { STATUS_BG, STATUS_BG_NONE } from "@/components/Ticket";
import { Button } from "@/components/Button";
import { ActivityIcon, FoldIcon, ListIcon, PanelIcon } from "@/icons";
import {
  loadCollapsedBoards,
  loadOpenOnly,
  loadTreeFilter,
  TREE_FILTERS,
  type TreeFilter,
  writeCollapsedBoards,
  writeOpenOnly,
  writeTreeFilter,
} from "./storage";

export type OpenTicketFn = (boardId: number, ticketId: number) => void;

const TREE_FILTER_LABEL: Record<TreeFilter, string> = {
  all: "Showing everything",
  tickets: "Hiding empty columns and boards",
  running: "Showing only running tickets",
};

// Statuses that count as "running" for the sidebar filter. Mirrors the
// `running` total in SessionSummary: stopped, stopping and error are excluded.
const RUNNING_STATUSES: ReadonlySet<string> = new Set([
  "working",
  "awaiting_perm",
  "idle",
  "starting",
]);

// Ticket ids, across every loaded board, whose session is running. Subscribes
// to each session, but the snapshot is a joined id string so the tree only
// re-renders when the running set itself changes — not on every status flip
// between e.g. working and idle.
function useRunningTicketIds(
  structures: readonly (BoardStructure | undefined)[],
): ReadonlySet<number> {
  const sessionEntries = structures.flatMap((st) =>
    st ? Object.entries(st.sessionIdByTicket) : [],
  );
  const sessionKey = sessionEntries.map(([, sid]) => sid).join(",");
  const subscribe = useCallback(
    (cb: () => void) => {
      const unsubs = sessionKey
        ? sessionKey.split(",").map((sid) => sessionStore.subscribe(Number(sid), cb))
        : [];
      return () => {
        for (const unsub of unsubs) unsub();
      };
    },
    [sessionKey],
  );
  const key = useSyncExternalStore(subscribe, () =>
    sessionEntries
      .filter(([, sid]) => RUNNING_STATUSES.has(sessionStore.get(sid)?.status ?? ""))
      .map(([ticketId]) => ticketId)
      .join(","),
  );
  return useMemo(() => new Set(key ? key.split(",").map(Number) : []), [key]);
}

export function BoardTree({
  onOpenTicket,
  openTicketIds,
  onCollapseSidebar,
  hasPanels = false,
}: {
  onOpenTicket: OpenTicketFn;
  openTicketIds: ReadonlySet<number>;
  onCollapseSidebar?: () => void;
  // True when tickets open as panels beside the tree (desktop). Enables the
  // "open only" filter, which is meaningless when a ticket replaces the tree.
  hasPanels?: boolean;
}) {
  const qc = useQueryClient();
  const boardsQ = useQuery({ queryKey: queryKeys.boards, queryFn: api.listBoards });
  const boards = boardsQ.data ?? [];
  const structures = useQueries({
    queries: boards.map((b) => ({
      queryKey: queryKeys.board(b.id),
      queryFn: () => fetchBoardStructure(b.id),
    })),
  });

  const boardSensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  );

  const moveBoardMut = useMutation({
    mutationFn: (input: { id: number; position: number }) =>
      api.moveBoard(input.id, input.position),
    onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.boards }),
  });

  function onBoardDragEnd(e: DragEndEvent) {
    const activeId = Number(e.active.id);
    const overId = e.over?.id;
    if (overId == null) return;
    const overNum = Number(overId);
    if (activeId === overNum) return;
    const oldIdx = boards.findIndex((b) => b.id === activeId);
    const newIdx = boards.findIndex((b) => b.id === overNum);
    if (oldIdx < 0 || newIdx < 0) return;
    qc.setQueryData<Board[]>(queryKeys.boards, (old) => {
      if (!old) return old;
      const next = [...old];
      const [moved] = next.splice(oldIdx, 1);
      next.splice(newIdx, 0, moved);
      return next;
    });
    moveBoardMut.mutate({ id: activeId, position: newIdx });
  }

  const [collapsed, setCollapsed] = useState<Set<number>>(loadCollapsedBoards);
  const toggle = (boardId: number) =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(boardId)) next.delete(boardId);
      else next.add(boardId);
      writeCollapsedBoards(next);
      return next;
    });

  const [treeFilter, setTreeFilter] = useState<TreeFilter>(loadTreeFilter);
  const nextTreeFilter = TREE_FILTERS[(TREE_FILTERS.indexOf(treeFilter) + 1) % TREE_FILTERS.length];
  const cycleTreeFilter = () => {
    writeTreeFilter(nextTreeFilter);
    setTreeFilter(nextTreeFilter);
  };
  const [openOnlyState, setOpenOnlyState] = useState(loadOpenOnly);
  const openOnly = hasPanels && openOnlyState;
  const setOpenOnly = (on: boolean) => {
    writeOpenOnly(on);
    setOpenOnlyState(on);
  };

  // The tree filter and the open filter stack: "running" + open means a
  // ticket must be both. "tickets" adds no predicate of its own — it only
  // switches on the compact layout (empty columns and boards dropped) that
  // every other filter implies.
  const runningIds = useRunningTicketIds(structures.map((q) => q.data));
  const runningOnly = treeFilter === "running";
  const filtering = treeFilter !== "all" || openOnly;
  const matches = (ticketId: number) =>
    (!runningOnly || runningIds.has(ticketId)) && (!openOnly || openTicketIds.has(ticketId));
  const boardMatches = (st: BoardStructure) =>
    Object.values(st.ticketIdsByColumn).some((ids) => ids.some(matches));
  // A board whose structure hasn't loaded (or failed to) can't be judged, so
  // it stays listed rather than vanishing; the empty message likewise waits
  // until every board has answered, so a persisted filter doesn't flash it.
  const boardHidden = (st: BoardStructure | undefined) =>
    filtering && st != null && !boardMatches(st);
  const toggleCls = (on: boolean) =>
    `rounded p-1 ${
      on
        ? "bg-accent-500/15 text-accent-500 ring-1 ring-inset ring-accent-500/40"
        : "text-fg-muted hover:bg-surface-2 hover:text-fg"
    }`;

  const header = (
    <div className="sticky top-0 z-(--z-raised) flex items-center border-b border-border bg-bg px-3 py-2">
      <h2 className="text-xs font-semibold uppercase tracking-wide text-fg-muted">Boards</h2>
      <button
        type="button"
        onClick={cycleTreeFilter}
        title={`${TREE_FILTER_LABEL[treeFilter]} — click for ${TREE_FILTER_LABEL[nextTreeFilter].toLowerCase()}`}
        aria-label={`Ticket filter: ${TREE_FILTER_LABEL[treeFilter].toLowerCase()}`}
        data-tree-filter={treeFilter}
        className={`ml-auto ${toggleCls(treeFilter !== "all")}`}
      >
        {treeFilter === "running" ? (
          <ActivityIcon size={12} />
        ) : treeFilter === "tickets" ? (
          <FoldIcon size={12} />
        ) : (
          <ListIcon size={12} />
        )}
      </button>
      {hasPanels && (
        <button
          type="button"
          onClick={() => setOpenOnly(!openOnly)}
          title={openOnly ? "Show all tickets" : "Show only tickets open in a panel"}
          aria-label="Filter by open"
          aria-pressed={openOnly}
          className={`ml-1 ${toggleCls(openOnly)}`}
        >
          <PanelIcon size={12} />
        </button>
      )}
      {onCollapseSidebar && (
        <button
          type="button"
          onClick={onCollapseSidebar}
          title="Hide boards sidebar"
          aria-label="Hide boards sidebar"
          aria-expanded={true}
          className="ml-1 rounded px-1 text-fg-muted hover:bg-surface-2 hover:text-fg"
        >
          <span aria-hidden>◀</span>
        </button>
      )}
    </div>
  );

  if (boardsQ.isLoading) {
    return (
      <div className="flex h-full flex-col">
        {header}
        <p className="p-3 text-sm text-fg-muted">Loading boards…</p>
      </div>
    );
  }
  if (boards.length === 0) {
    return (
      <div className="flex h-full flex-col">
        {header}
        <p className="p-3 text-sm text-fg-muted">No boards yet. Create one from the Board view.</p>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col overflow-y-auto">
      {header}
      <DndContext
        sensors={boardSensors}
        collisionDetection={closestCenter}
        onDragEnd={onBoardDragEnd}
      >
        <SortableContext items={boards.map((b) => b.id)} strategy={verticalListSortingStrategy}>
          {boards.map((b, i) => (
            <BoardNode
              key={b.id}
              boardId={b.id}
              boardName={b.name}
              structure={structures[i]?.data}
              collapsed={collapsed.has(b.id)}
              matches={filtering ? matches : null}
              hidden={boardHidden(structures[i]?.data)}
              onToggle={() => toggle(b.id)}
              onOpenTicket={onOpenTicket}
              openTicketIds={openTicketIds}
            />
          ))}
        </SortableContext>
      </DndContext>
      {structures.every((q) => boardHidden(q.data)) && (
        <p className="p-3 text-sm text-fg-muted">No tickets match the current filter.</p>
      )}
    </div>
  );
}

function BoardNode({
  boardId,
  boardName,
  structure,
  collapsed,
  matches,
  hidden,
  onToggle,
  onOpenTicket,
  openTicketIds,
}: {
  boardId: number;
  boardName: string;
  structure: BoardStructure | undefined;
  collapsed: boolean;
  // Ticket predicate while a sidebar filter is on; null when unfiltered.
  matches: ((ticketId: number) => boolean) | null;
  hidden: boolean;
  onToggle: () => void;
  onOpenTicket: OpenTicketFn;
  openTicketIds: ReadonlySet<number>;
}) {
  // Subscribe to live updates for *every* board in the tree so status dots
  // animate in real time. See useBoardSubscription doc for the connection
  // limit caveat.
  useBoardSubscription(boardId);

  const {
    setNodeRef: setSortableRef,
    attributes: sortableAttrs,
    listeners: sortableListeners,
    transform: sortableTransform,
    transition: sortableTransition,
    isDragging: isBoardDragging,
  } = useSortable({ id: boardId });
  const sortableStyle: React.CSSProperties = {
    transform: CSS.Translate.toString(sortableTransform),
    transition: sortableTransition,
    opacity: isBoardDragging ? 0.4 : 1,
  };

  const qc = useQueryClient();
  const [addingColumnId, setAddingColumnId] = useState<number | null>(null);
  const [title, setTitle] = useState("");

  const createMut = useMutation({
    mutationFn: (columnId: number) => api.createTicket(boardId, { column_id: columnId, title }),
    onSuccess: () => {
      setTitle("");
      setAddingColumnId(null);
      qc.invalidateQueries({ queryKey: queryKeys.board(boardId) });
    },
  });

  const { draggingId, sensors, onDragStart, onDragEnd, onDragCancel } = useTicketDnd(
    boardId,
    structure,
  );

  const visibleIds = (ids: number[]) => (matches ? ids.filter(matches) : ids);

  const totalTickets = structure
    ? Object.values(structure.ticketIdsByColumn).reduce((n, ids) => n + visibleIds(ids).length, 0)
    : 0;

  const draggingSessionId =
    draggingId != null && structure ? (structure.sessionIdByTicket[draggingId] ?? null) : null;

  // Filtered-out boards render nothing but stay mounted: the subscription
  // above is what brings a board back when one of its sessions starts. A
  // board with an add-ticket form open stays put so the draft isn't lost
  // when its last matching ticket drops out mid-typing.
  if (hidden && addingColumnId == null) return null;

  return (
    <div
      ref={setSortableRef}
      style={sortableStyle}
      className="group/board border-b border-border"
      data-board-node={boardId}
    >
      <div className="flex w-full items-center hover:bg-surface-2">
        <button
          type="button"
          {...sortableAttrs}
          {...sortableListeners}
          title="Drag to reorder board"
          aria-label={`Reorder ${boardName}`}
          className="touch-none cursor-grab px-1 py-2 text-xs leading-none text-fg-muted opacity-0 group-hover/board:opacity-100 focus:opacity-100"
        >
          ⋮⋮
        </button>
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={!collapsed}
          className="flex flex-1 items-center gap-2 px-1 py-2 pr-3 text-left text-sm"
        >
          <span
            className={`inline-block w-3 text-xs text-fg-muted transition-transform ${collapsed ? "" : "rotate-90"}`}
          >
            ▶
          </span>
          <span className="font-medium">{boardName}</span>
          <span className="ml-auto text-xs text-fg-muted">{totalTickets}</span>
        </button>
      </div>
      {!collapsed && structure && (
        <DndContext
          sensors={sensors}
          collisionDetection={closestCorners}
          onDragStart={onDragStart}
          onDragEnd={onDragEnd}
          onDragCancel={onDragCancel}
        >
          <div className="pb-1">
            {structure.columns.map((c) => {
              const ids = visibleIds(structure.ticketIdsByColumn[c.id] ?? []);
              const isAdding = addingColumnId === c.id;
              // Filtered view stays compact: drop columns with no matches.
              if (matches && ids.length === 0 && !isAdding) return null;
              return (
                <ColumnSection
                  key={c.id}
                  columnId={c.id}
                  columnName={c.name}
                  ticketIds={ids}
                  sessionIdByTicket={structure.sessionIdByTicket}
                  boardId={boardId}
                  onOpenTicket={onOpenTicket}
                  openTicketIds={openTicketIds}
                  isAdding={isAdding}
                  onStartAdding={() => {
                    setAddingColumnId(isAdding ? null : c.id);
                    setTitle("");
                  }}
                  onCancelAdding={() => {
                    setAddingColumnId(null);
                    setTitle("");
                  }}
                  title={title}
                  onTitleChange={setTitle}
                  onSubmitAdd={() => {
                    if (title.trim() && !createMut.isPending) createMut.mutate(c.id);
                  }}
                  pendingAdd={createMut.isPending}
                />
              );
            })}
          </div>
          <DragOverlay>
            {draggingId != null ? (
              <TicketRowPreview ticketId={draggingId} sessionId={draggingSessionId} />
            ) : null}
          </DragOverlay>
        </DndContext>
      )}
    </div>
  );
}

function ColumnSection({
  columnId,
  columnName,
  ticketIds,
  sessionIdByTicket,
  boardId,
  onOpenTicket,
  openTicketIds,
  isAdding,
  onStartAdding,
  onCancelAdding,
  title,
  onTitleChange,
  onSubmitAdd,
  pendingAdd,
}: {
  columnId: number;
  columnName: string;
  ticketIds: number[];
  sessionIdByTicket: Record<number, number>;
  boardId: number;
  onOpenTicket: OpenTicketFn;
  openTicketIds: ReadonlySet<number>;
  isAdding: boolean;
  onStartAdding: () => void;
  onCancelAdding: () => void;
  title: string;
  onTitleChange: (s: string) => void;
  onSubmitAdd: () => void;
  pendingAdd: boolean;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: `col-${columnId}` });
  const addInputRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (isAdding) addInputRef.current?.focus();
  }, [isAdding]);
  return (
    <div
      ref={setNodeRef}
      data-tree-column={columnId}
      className={`px-3 py-1 ${isOver ? "bg-accent-500/10" : ""}`}
    >
      <div className="mb-1 flex items-center justify-between gap-2">
        <h3 className="text-[10px] font-semibold uppercase tracking-wide text-fg-muted">
          {columnName}
        </h3>
        <Button
          variant="ghost"
          size="sm"
          type="button"
          className="leading-none"
          title={`Add ticket to ${columnName}`}
          aria-label={`Add ticket to ${columnName}`}
          onClick={onStartAdding}
        >
          +
        </Button>
      </div>
      {isAdding && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            onSubmitAdd();
          }}
          className="mb-1 flex flex-col gap-1"
        >
          <input
            ref={addInputRef}
            className="rounded bg-surface-2 px-2 py-1 text-sm"
            value={title}
            onChange={(e) => onTitleChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") onCancelAdding();
            }}
            placeholder="ticket title"
            disabled={pendingAdd}
          />
          <div className="flex gap-2 text-xs">
            <Button
              variant="primary"
              size="sm"
              type="submit"
              pending={pendingAdd}
              idleLabel="add"
              pendingLabel="adding…"
            />
            <Button
              variant="ghost"
              size="sm"
              type="button"
              onClick={onCancelAdding}
              disabled={pendingAdd}
            >
              cancel
            </Button>
          </div>
        </form>
      )}
      {ticketIds.length > 0 && (
        <SortableContext items={ticketIds} strategy={verticalListSortingStrategy}>
          <ul className="flex flex-col gap-0.5">
            {ticketIds.map((tid) => (
              <TicketRow
                key={tid}
                ticketId={tid}
                boardId={boardId}
                sessionId={sessionIdByTicket[tid] ?? null}
                onOpenTicket={onOpenTicket}
                active={openTicketIds.has(tid)}
              />
            ))}
          </ul>
        </SortableContext>
      )}
    </div>
  );
}

function TicketRow({
  ticketId,
  boardId,
  sessionId,
  onOpenTicket,
  active,
}: {
  ticketId: number;
  boardId: number;
  sessionId: number | null;
  onOpenTicket: OpenTicketFn;
  active: boolean;
}) {
  const ticket = useTicket(ticketId);
  const session = useSession(sessionId);
  const { attributes, listeners, setNodeRef, isDragging, transform, transition } = useSortable({
    id: ticketId,
  });
  const style: React.CSSProperties = {
    transform: CSS.Translate.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  };
  if (!ticket) return null;
  const status = session?.status ?? "";
  const dotClass = status ? (STATUS_BG[status] ?? STATUS_BG_NONE) : STATUS_BG_NONE;
  const activeCls = active
    ? "bg-accent-500/15 ring-1 ring-inset ring-accent-500/40"
    : "hover:bg-surface-2";
  return (
    <li ref={setNodeRef} style={style} data-tree-ticket={ticketId}>
      <button
        type="button"
        onClick={() => onOpenTicket(boardId, ticketId)}
        {...attributes}
        {...listeners}
        className={`flex w-full touch-none items-center gap-2 rounded px-2 py-1 text-left text-sm ${activeCls}`}
        title={ticket.title}
        aria-current={active ? "true" : undefined}
        data-session-status={status || "none"}
      >
        <span className={`inline-block h-2 w-2 shrink-0 rounded-full ${dotClass}`} />
        <span className="truncate text-fg-muted">#{ticket.id}</span>
        <span className="truncate">{ticket.title}</span>
      </button>
    </li>
  );
}

function TicketRowPreview({ ticketId, sessionId }: { ticketId: number; sessionId: number | null }) {
  const ticket = useTicket(ticketId);
  const session = useSession(sessionId);
  if (!ticket) return null;
  const status = session?.status ?? "";
  const dotClass = status ? (STATUS_BG[status] ?? STATUS_BG_NONE) : STATUS_BG_NONE;
  return (
    <div className="flex items-center gap-2 rounded bg-surface-2 px-2 py-1 text-sm shadow-2xl ring-1 ring-border">
      <span className={`inline-block h-2 w-2 shrink-0 rounded-full ${dotClass}`} />
      <span className="truncate text-fg-muted">#{ticket.id}</span>
      <span className="truncate">{ticket.title}</span>
    </div>
  );
}
