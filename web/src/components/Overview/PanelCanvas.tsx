// biome-ignore-all lint/a11y/useSemanticElements: no native semantic for focus-tracked draggable panels and their drag handles.
// biome-ignore-all lint/a11y/noNoninteractiveTabindex: panels capture focus to track the active panel among siblings.
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { DraggableEvent } from "react-draggable";
import { Rnd } from "react-rnd";
import { queryKeys } from "@/api/keys";
import { fetchBoardStructure, useTerminalSlot } from "@/store";
import type { BoardStructure } from "@/store";
import { Button } from "@/components/Button";
import { SessionView } from "@/components/SessionView";
import { FullscreenEnterIcon, FullscreenExitIcon } from "@/icons";
import { useShortcut } from "@/keys/useShortcut";
import { NewTicketModal } from "./NewTicketModal";
import {
  findNeighborInDirection,
  nextSlot,
  slotRect,
  snapPosition,
  tileZoneFromPoint,
  type Direction,
  type Rect,
  type SlotKey,
  type SnapGuides,
} from "./tile";
import { loadPanels, writePanels, type PersistedPanel } from "./storage";

export type PanelCanvasHandle = {
  open: (boardId: number, ticketId: number) => void;
};

const DEFAULT_W = 640;
const DEFAULT_H = 480;
const CASCADE_STEP = 28;

type DragState = {
  ticketId: number;
  tileZone: SlotKey | null;
  guides: SnapGuides;
  didMove: boolean;
};

// Free-form workspace of draggable, resizable session panels. Each panel
// renders a SessionView for one ticket; multiple may be open at once.
// Layout is persisted to localStorage and re-validated against the live
// ticket set whenever a board structure refetches (covers archive/delete
// in another tab).
//
// Panels can also be tiled to canvas slots (halves, quadrants, max) via
// drag-to-edge zones or Ctrl+Shift+Alt+Arrow. While dragging, edges snap
// to canvas mid/edges and to other panels' edges within 8px. Ctrl+Alt+Arrow
// moves focus to the nearest panel in that direction.
export function PanelCanvas({
  registerHandle,
  onOpenTicketsChange,
}: {
  registerHandle: (handle: PanelCanvasHandle | null) => void;
  onOpenTicketsChange?: (ids: ReadonlySet<number>) => void;
}) {
  const [panels, setPanels] = useState<PersistedPanel[]>(loadPanels);
  const [focusedTicketId, setFocusedTicketId] = useState<number | null>(null);
  const [dragState, setDragState] = useState<DragState | null>(null);
  const [canvasSize, setCanvasSize] = useState({ w: 0, h: 0 });
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const qc = useQueryClient();

  useEffect(() => {
    writePanels(panels);
    onOpenTicketsChange?.(new Set(panels.map((p) => p.ticketId)));
  }, [panels, onOpenTicketsChange]);

  // Track canvas size so tile rects follow viewport changes (sidebar resize,
  // window resize). ResizeObserver fires synchronously on layout, so tiled
  // panels reposition without a frame of stale geometry. Measure in a layout
  // effect (before paint) so a restored maximized/tiled panel fills the canvas
  // on its first painted frame instead of briefly rendering at its stored
  // float rect while canvasSize is still {0,0}.
  useLayoutEffect(() => {
    const el = canvasRef.current;
    if (!el) return;
    const update = () => {
      const r = el.getBoundingClientRect();
      setCanvasSize({ w: r.width, h: r.height });
    };
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Filter panels whose ticket no longer exists on its board. Re-runs when
  // any board query is added, removed, or invalidated; cheap because we
  // only touch the panels array. Defers the setState via queueMicrotask so
  // we never call it synchronously from a child component's render — Panel's
  // own useQuery can fire cache notifications mid-render.
  useEffect(() => {
    const recheck = () => {
      setPanels((prev) => {
        const next = prev.filter((p) => isLiveTicket(qc, p.boardId, p.ticketId));
        if (next.length === prev.length) return prev;
        return next;
      });
    };
    recheck();
    return qc.getQueryCache().subscribe(() => {
      queueMicrotask(recheck);
    });
  }, [qc]);

  const open = useCallback((boardId: number, ticketId: number) => {
    setPanels((prev) => {
      const idx = prev.findIndex((p) => p.ticketId === ticketId);
      if (idx >= 0) {
        const next = prev.slice();
        const [moved] = next.splice(idx, 1);
        next.push(moved);
        return next;
      }
      const last = prev[prev.length - 1];
      const x = last ? Math.max(0, last.x + CASCADE_STEP) : 24;
      const y = last ? Math.max(0, last.y + CASCADE_STEP) : 24;
      return [...prev, { ticketId, boardId, x, y, width: DEFAULT_W, height: DEFAULT_H }];
    });
    setFocusedTicketId(ticketId);
  }, []);

  useEffect(() => {
    registerHandle({ open });
    return () => registerHandle(null);
  }, [open, registerHandle]);

  const close = useCallback((ticketId: number) => {
    setPanels((prev) => prev.filter((p) => p.ticketId !== ticketId));
    setFocusedTicketId((cur) => (cur === ticketId ? null : cur));
  }, []);

  const update = useCallback((ticketId: number, patch: Partial<PersistedPanel>) => {
    setPanels((prev) => prev.map((p) => (p.ticketId === ticketId ? { ...p, ...patch } : p)));
  }, []);

  const bringToFront = useCallback((ticketId: number) => {
    setPanels((prev) => {
      const idx = prev.findIndex((p) => p.ticketId === ticketId);
      if (idx < 0 || idx === prev.length - 1) return prev;
      const next = prev.slice();
      const [moved] = next.splice(idx, 1);
      next.push(moved);
      return next;
    });
  }, []);

  // Computed render rect: tile rect when tiled, otherwise the stored float rect.
  const displayRect = useCallback(
    (p: PersistedPanel): Rect => {
      if (p.tile && canvasSize.w > 0 && canvasSize.h > 0) {
        return slotRect(canvasSize.w, canvasSize.h, p.tile);
      }
      return { x: p.x, y: p.y, width: p.width, height: p.height };
    },
    [canvasSize.w, canvasSize.h],
  );

  // Apply (or clear) a tile slot. Captures the floating rect on first tile
  // so un-tile can restore it. Calling with slot=null restores from
  // floatRect when present. Always clears prevTile — that field is only
  // meaningful within a toggleMaximize sequence.
  const applyTile = useCallback((ticketId: number, slot: SlotKey | null) => {
    setPanels((prev) =>
      prev.map((p) => {
        if (p.ticketId !== ticketId) return p;
        if (slot === null) {
          const f = p.floatRect;
          if (f) {
            return {
              ...p,
              x: f.x,
              y: f.y,
              width: f.width,
              height: f.height,
              tile: null,
              floatRect: undefined,
              prevTile: undefined,
            };
          }
          return { ...p, tile: null, floatRect: undefined, prevTile: undefined };
        }
        const floatRect = p.tile
          ? p.floatRect
          : { x: p.x, y: p.y, width: p.width, height: p.height };
        return { ...p, tile: slot, floatRect, prevTile: undefined };
      }),
    );
  }, []);

  // Toggle between "max" and the panel's previous state. Maximizing from a
  // tiled slot remembers it in prevTile so the next toggle restores that
  // slot rather than dropping back to floating.
  const toggleMaximize = useCallback((ticketId: number) => {
    setPanels((prev) =>
      prev.map((p) => {
        if (p.ticketId !== ticketId) return p;
        if (p.tile === "max") {
          if (p.prevTile) {
            return { ...p, tile: p.prevTile, prevTile: undefined };
          }
          const f = p.floatRect;
          if (f) {
            return {
              ...p,
              x: f.x,
              y: f.y,
              width: f.width,
              height: f.height,
              tile: null,
              floatRect: undefined,
              prevTile: undefined,
            };
          }
          return { ...p, tile: null, floatRect: undefined, prevTile: undefined };
        }
        const floatRect = p.tile
          ? p.floatRect
          : { x: p.x, y: p.y, width: p.width, height: p.height };
        return { ...p, tile: "max", floatRect, prevTile: p.tile ?? undefined };
      }),
    );
  }, []);

  // Board id of the focused panel — drives the F shortcut target and the
  // default board for the new-ticket modal.
  const focusedBoardId =
    focusedTicketId != null
      ? (panels.find((p) => p.ticketId === focusedTicketId)?.boardId ?? null)
      : null;

  useShortcut(
    "session.fullscreen",
    () => {
      if (focusedTicketId != null) toggleMaximize(focusedTicketId);
    },
    { enabled: focusedTicketId != null },
  );

  const [newTicketOpen, setNewTicketOpen] = useState(false);
  useShortcut("ticket.create", () => setNewTicketOpen(true));

  const handleDragStart = useCallback((ticketId: number) => {
    setDragState({
      ticketId,
      tileZone: null,
      guides: { vertical: [], horizontal: [] },
      didMove: false,
    });
  }, []);

  const handleDrag = useCallback(
    (ticketId: number, d: { x: number; y: number }, e: DraggableEvent) => {
      const cv = canvasRef.current;
      if (!cv) return;
      const rect = cv.getBoundingClientRect();
      const { x: cx, y: cy } = clientPoint(e);
      const mx = cx - rect.left;
      const my = cy - rect.top;
      const panel = panels.find((p) => p.ticketId === ticketId);
      if (!panel) return;
      const tileZone = tileZoneFromPoint(mx, my, canvasSize.w, canvasSize.h);
      // Snap guides only meaningful for floating panels.
      let guides: SnapGuides = { vertical: [], horizontal: [] };
      if (!panel.tile) {
        const others = panels.filter((p) => p.ticketId !== ticketId);
        const dr = displayRect(panel);
        const res = snapPosition(
          { x: d.x, y: d.y, width: dr.width, height: dr.height, ticketId },
          others,
          canvasSize.w,
          canvasSize.h,
        );
        guides = res.guides;
      }
      setDragState({ ticketId, tileZone, guides, didMove: true });
    },
    [panels, canvasSize.w, canvasSize.h, displayRect],
  );

  const handleDragStop = useCallback(
    (ticketId: number, d: { x: number; y: number }) => {
      const ds = dragState;
      setDragState(null);
      const panel = panels.find((p) => p.ticketId === ticketId);
      if (!panel) return;
      // 0. Plain click on the drag handle (no movement) → no-op. Without this
      //    the case-2 path below would un-tile any tiled panel on a single
      //    click, which also breaks the double-click-to-maximize toggle.
      if (!ds?.didMove) return;
      // 1. Drop into a tile zone → tile.
      if (ds?.tileZone) {
        applyTile(ticketId, ds.tileZone);
        return;
      }
      // 2. Tiled panel dragged outside any zone → un-tile, restore float
      //    dimensions, place top-left at the drop point.
      if (panel.tile) {
        const f = panel.floatRect;
        const w = f?.width ?? panel.width;
        const h = f?.height ?? panel.height;
        update(ticketId, {
          tile: null,
          floatRect: undefined,
          prevTile: undefined,
          x: clampX(d.x, w, canvasSize.w),
          y: clampY(d.y, h, canvasSize.h),
          width: w,
          height: h,
        });
        return;
      }
      // 3. Floating panel → apply edge/midline/sibling snap.
      const others = panels.filter((p) => p.ticketId !== ticketId);
      const { x, y } = snapPosition(
        { x: d.x, y: d.y, width: panel.width, height: panel.height, ticketId },
        others,
        canvasSize.w,
        canvasSize.h,
      );
      update(ticketId, { x, y });
    },
    [dragState, panels, canvasSize.w, canvasSize.h, applyTile, update],
  );

  const handleResizeStop = useCallback(
    (ticketId: number, rect: Rect) => {
      // Manual resize untiles.
      update(ticketId, {
        ...rect,
        tile: null,
        floatRect: undefined,
        prevTile: undefined,
      });
    },
    [update],
  );

  // Mirror the board view's behavior of focusing the terminal on pane switch.
  // In the board view this falls out of PtyTerminal's mountTarget effect
  // because the slot DOM remounts under the new ticketId. In overview every
  // panel stays mounted, so mountTarget never changes — we focus explicitly
  // when the focused ticket (or its registered slot) changes. Subscribing to
  // the slot also handles freshly-opened panels: the slot ref fires after
  // SessionView mounts (post board-structure load), and this effect re-runs.
  const focusedSlot = useTerminalSlot(focusedTicketId);
  useEffect(() => {
    if (focusedTicketId == null) return;
    const slotEl = focusedSlot?.agent ?? focusedSlot?.shell;
    if (!slotEl) return;
    const host = slotEl.querySelector<HTMLElement>("[data-terminal]");
    host?.focus({ preventScroll: true });
  }, [focusedTicketId, focusedSlot]);

  // Ctrl+Alt+Arrow → focus neighbor; Ctrl+Shift+Alt+Arrow → re-tile focused.
  // Bound at the window in capture phase so we run before the global key
  // dispatcher (which uses Ctrl+Alt+Arrow for ticket/column navigation on
  // the board view). When we own the chord we stopImmediatePropagation so
  // those navigation actions don't also fire. Only mounted on the overview
  // route since PanelCanvas only renders here. The Ctrl+Alt prefix can't
  // collide with text input, so we don't bail when the focus is in an
  // editable element (e.g. an embedded terminal).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!e.ctrlKey || !e.altKey) return;
      const dir = arrowToDirection(e.key);
      if (!dir) return;
      e.preventDefault();
      e.stopImmediatePropagation();
      if (e.shiftKey) {
        if (focusedTicketId == null) return;
        const focused = panels.find((p) => p.ticketId === focusedTicketId);
        if (!focused) return;
        applyTile(focusedTicketId, nextSlot(focused.tile, dir));
      } else {
        const startId = focusedTicketId ?? panels[panels.length - 1]?.ticketId;
        if (startId == null) return;
        const rects = panels.map((p) => ({ ticketId: p.ticketId, ...displayRect(p) }));
        const neighborId = findNeighborInDirection(rects, startId, dir);
        if (neighborId != null) {
          setFocusedTicketId(neighborId);
          bringToFront(neighborId);
        }
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [panels, focusedTicketId, applyTile, bringToFront, displayRect]);

  return (
    <div
      ref={canvasRef}
      data-overview-canvas="true"
      className="relative h-full w-full overflow-hidden bg-surface"
    >
      {panels.length === 0 && (
        <div className="absolute inset-0 flex items-center justify-center text-sm text-fg-muted">
          Click a ticket on the left to open a session panel.
        </div>
      )}
      {panels.map((p) => {
        const dr = displayRect(p);
        return (
          <Panel
            key={p.ticketId}
            panel={p}
            rect={dr}
            canvasSize={canvasSize}
            focused={p.ticketId === focusedTicketId}
            onClose={() => close(p.ticketId)}
            onResizeStop={(r) => handleResizeStop(p.ticketId, r)}
            onBringToFront={() => bringToFront(p.ticketId)}
            onFocus={() => setFocusedTicketId(p.ticketId)}
            onDragStart={() => handleDragStart(p.ticketId)}
            onDrag={(d, e) => handleDrag(p.ticketId, d, e)}
            onDragStop={(d) => handleDragStop(p.ticketId, d)}
            onToggleMaximize={() => toggleMaximize(p.ticketId)}
          />
        );
      })}
      {dragState?.tileZone && canvasSize.w > 0 && (
        <TileZonePreview slot={dragState.tileZone} canvasW={canvasSize.w} canvasH={canvasSize.h} />
      )}
      {dragState?.guides.vertical.map((x) => (
        <div
          key={`v-${x}`}
          className="pointer-events-none absolute top-0 bottom-0 w-px bg-accent-500/70"
          style={{ left: `${x}px` }}
        />
      ))}
      {dragState?.guides.horizontal.map((y) => (
        <div
          key={`h-${y}`}
          className="pointer-events-none absolute left-0 right-0 h-px bg-accent-500/70"
          style={{ top: `${y}px` }}
        />
      ))}
      {newTicketOpen && (
        <NewTicketModal
          defaultBoardId={focusedBoardId}
          onClose={() => setNewTicketOpen(false)}
          onCreated={(ticketId, boardId) => open(boardId, ticketId)}
        />
      )}
    </div>
  );
}

function TileZonePreview({
  slot,
  canvasW,
  canvasH,
}: {
  slot: SlotKey;
  canvasW: number;
  canvasH: number;
}) {
  const r = slotRect(canvasW, canvasH, slot);
  return (
    <div
      className="pointer-events-none absolute rounded border-2 border-accent-500 bg-accent-500/15"
      style={{ left: r.x, top: r.y, width: r.width, height: r.height }}
    />
  );
}

function clientPoint(e: DraggableEvent): { x: number; y: number } {
  if ("touches" in e) {
    const t = e.touches[0] ?? ("changedTouches" in e ? e.changedTouches[0] : null);
    return { x: t?.clientX ?? 0, y: t?.clientY ?? 0 };
  }
  return { x: e.clientX, y: e.clientY };
}

function arrowToDirection(key: string): Direction | null {
  if (key === "ArrowLeft") return "L";
  if (key === "ArrowRight") return "R";
  if (key === "ArrowUp") return "U";
  if (key === "ArrowDown") return "D";
  return null;
}

function clampX(x: number, w: number, canvasW: number): number {
  if (canvasW <= 0) return x;
  return Math.max(0, Math.min(x, canvasW - w));
}

function clampY(y: number, h: number, canvasH: number): number {
  if (canvasH <= 0) return y;
  return Math.max(0, Math.min(y, canvasH - h));
}

function Panel({
  panel,
  rect,
  canvasSize,
  focused,
  onClose,
  onResizeStop,
  onBringToFront,
  onFocus,
  onDragStart,
  onDrag,
  onDragStop,
  onToggleMaximize,
}: {
  panel: PersistedPanel;
  rect: Rect;
  canvasSize: { w: number; h: number };
  focused: boolean;
  onClose: () => void;
  onResizeStop: (r: Rect) => void;
  onBringToFront: () => void;
  onFocus: () => void;
  onDragStart: () => void;
  onDrag: (d: { x: number; y: number }, e: DraggableEvent) => void;
  onDragStop: (d: { x: number; y: number }) => void;
  onToggleMaximize: () => void;
}) {
  const boardQ = useQuery({
    queryKey: queryKeys.board(panel.boardId),
    queryFn: () => fetchBoardStructure(panel.boardId),
  });
  const structure = boardQ.data;

  const rndProps = {
    position: { x: rect.x, y: rect.y },
    size: { width: rect.width, height: rect.height },
    bounds: "parent" as const,
    minWidth: 320,
    minHeight: 240,
    dragHandleClassName: "panel-drag-handle",
    onMouseDown: onBringToFront,
    onDragStart: () => onDragStart(),
    onDrag: (e: DraggableEvent, d: { x: number; y: number }) => onDrag({ x: d.x, y: d.y }, e),
    onDragStop: (_e: DraggableEvent, d: { x: number; y: number }) => onDragStop({ x: d.x, y: d.y }),
    onResizeStop: (
      _e: MouseEvent | TouchEvent,
      _dir: unknown,
      ref: HTMLElement,
      _delta: unknown,
      pos: { x: number; y: number },
    ) =>
      onResizeStop({
        width: ref.offsetWidth,
        height: ref.offsetHeight,
        x: pos.x,
        y: pos.y,
      }),
  };

  if (!structure) {
    // While the board structure loads, fill the canvas with the plain bg
    // color (darker than the surface canvas behind it). Override position/size
    // after the spread (which carries bounds, min sizes, and the drag/resize
    // handlers); once the structure resolves the panel snaps back to its real
    // rect below.
    return (
      <Rnd
        {...rndProps}
        position={{ x: 0, y: 0 }}
        size={{ width: canvasSize.w, height: canvasSize.h }}
        className="bg-bg"
      />
    );
  }

  return (
    <Rnd
      {...rndProps}
      className={`overflow-hidden rounded border bg-bg shadow-lg ${focused ? "border-accent-500" : "border-border"}`}
    >
      <div
        role="group"
        aria-label="Ticket panel"
        tabIndex={0}
        onFocus={onFocus}
        onMouseDown={onFocus}
        className="flex h-full flex-col outline-none"
        data-panel={panel.ticketId}
        data-tile={panel.tile ?? "float"}
        data-focused={focused ? "true" : "false"}
      >
        <div
          role="button"
          tabIndex={0}
          aria-label="Move panel"
          title="Drag to move, double-click to maximize"
          onDoubleClick={onToggleMaximize}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              onToggleMaximize();
            }
          }}
          className="panel-drag-handle flex h-5 cursor-move items-center bg-surface-2 px-2 text-[10px] uppercase tracking-wide text-fg-muted"
        >
          drag
        </div>
        <div className="min-h-0 flex-1">
          <SessionView
            ticketId={panel.ticketId}
            boardId={panel.boardId}
            baseBranch={structure.board.base_branch}
            mergeConfig={structure.merge_config}
            syncConfig={structure.sync_config}
            sessionIdByTicket={structure.sessionIdByTicket}
            onClose={onClose}
            shortcutsEnabled={focused}
            headerExtras={
              <Button
                variant="neutral"
                size="icon"
                onClick={onToggleMaximize}
                aria-label={panel.tile === "max" ? "Restore" : "Maximize"}
                title={panel.tile === "max" ? "Restore" : "Maximize"}
              >
                {panel.tile === "max" ? <FullscreenExitIcon /> : <FullscreenEnterIcon />}
              </Button>
            }
          />
        </div>
      </div>
    </Rnd>
  );
}

function isLiveTicket(
  qc: ReturnType<typeof useQueryClient>,
  boardId: number,
  ticketId: number,
): boolean {
  const data = qc.getQueryData<BoardStructure>(queryKeys.board(boardId));
  // If the board hasn't loaded yet, give it the benefit of the doubt — we
  // don't want to drop a panel just because its board structure hasn't
  // arrived from the network.
  if (!data) return true;
  for (const ids of Object.values(data.ticketIdsByColumn)) {
    if (ids.includes(ticketId)) return true;
  }
  return false;
}
