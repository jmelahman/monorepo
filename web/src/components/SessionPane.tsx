import { useCallback, useEffect, useState } from "react";
import type { MergeConfig, SyncConfig } from "@/api/client";
import { activeTicketStore, useActiveTicketId } from "@/store";
import { FullscreenEnterIcon, FullscreenExitIcon } from "@/icons";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import type { TerminalOrientation } from "@/hooks/useTerminalOrientation";
import { useShortcut } from "@/keys/useShortcut";
import { Button } from "./Button";
import { SessionView } from "./SessionView";

const MIN_WIDTH = 320;
const MAX_WIDTH = 1600;
const DEFAULT_WIDTH = 640;
const WIDTH_STORAGE_KEY = "sessionPane.width";
const MIN_HEIGHT = 200;
const MAX_HEIGHT = 1200;
const DEFAULT_HEIGHT = 360;
const HEIGHT_STORAGE_KEY = "sessionPane.height";

function loadInitialSize(key: string, fallback: number, min: number, max: number): number {
  const raw = typeof localStorage !== "undefined" ? localStorage.getItem(key) : null;
  const n = raw ? Number(raw) : NaN;
  if (!Number.isFinite(n)) return fallback;
  return Math.min(max, Math.max(min, n));
}

// Board-view session pane: a docked aside that reads the active ticket from
// the global store, owns its own edge resize handle, click-outside-to-close,
// and fullscreen toggle. Wraps SessionView, which carries the actual
// per-session content + actions.
export function SessionPane({
  boardId,
  baseBranch,
  mergeConfig,
  syncConfig,
  sessionIdByTicket,
  orientation,
}: {
  boardId: number;
  baseBranch: string;
  mergeConfig: MergeConfig;
  syncConfig: SyncConfig;
  sessionIdByTicket: Record<number, number>;
  orientation: TerminalOrientation;
}) {
  // Subscribe to selection here so picking a ticket doesn't re-render `Board`
  // (and therefore doesn't cascade through every Column / Ticket).
  const ticketId = useActiveTicketId();
  const onClose = useCallback(() => activeTicketStore.set(null), []);
  // On narrow viewports the side-by-side layout doesn't fit, so the pane takes
  // over as a full-screen overlay (same chrome as the manual fullscreen mode).
  const isMobile = useMediaQuery("(max-width: 639px)");
  const isHorizontal = orientation === "horizontal";
  const [fullscreen, setFullscreen] = useState(false);
  const tabsEnabled = ticketId != null;
  useShortcut("session.fullscreen", () => setFullscreen((v) => !v), {
    enabled: tabsEnabled,
  });

  const [width, setWidth] = useState<number>(() =>
    loadInitialSize(WIDTH_STORAGE_KEY, DEFAULT_WIDTH, MIN_WIDTH, MAX_WIDTH),
  );
  const [height, setHeight] = useState<number>(() =>
    loadInitialSize(HEIGHT_STORAGE_KEY, DEFAULT_HEIGHT, MIN_HEIGHT, MAX_HEIGHT),
  );
  const [resizing, setResizing] = useState(false);

  useEffect(() => {
    if (ticketId == null) return;
    const handler = (e: MouseEvent) => {
      const target = e.target as Element | null;
      if (!target?.closest("[data-board-area]")) return;
      // Ticket clicks switch the active ticket themselves; skip close to avoid a flicker.
      // Add-ticket clicks open the inline form in the column; closing the panel here
      // would be a surprising side effect of starting to create a ticket.
      if (target.closest("[data-ticket-card], [data-ticket-add]")) return;
      onClose();
    };
    window.addEventListener("mousedown", handler);
    return () => window.removeEventListener("mousedown", handler);
  }, [ticketId, onClose]);

  useEffect(() => {
    if (!fullscreen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      // Let Escape reach the embedded terminal when it has focus.
      const target = e.target as Element | null;
      const active = document.activeElement;
      if (target?.closest?.("[data-terminal]") || active?.closest?.("[data-terminal]")) {
        return;
      }
      setFullscreen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [fullscreen]);

  // height/width are read once as the resize start point; adding them to deps
  // would tear down and re-attach listeners on every mouse move.
  // biome-ignore lint/correctness/useExhaustiveDependencies: see above.
  useEffect(() => {
    if (!resizing) return;
    let pending: number | null = null;
    let nextSize = isHorizontal ? height : width;
    const apply = () => {
      pending = null;
      if (isHorizontal) setHeight(nextSize);
      else setWidth(nextSize);
    };
    const onMove = (e: MouseEvent) => {
      nextSize = isHorizontal
        ? Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, window.innerHeight - e.clientY))
        : Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, window.innerWidth - e.clientX));
      if (pending == null) pending = requestAnimationFrame(apply);
    };
    const onUp = () => setResizing(false);
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    const prevCursor = document.body.style.cursor;
    const prevSelect = document.body.style.userSelect;
    document.body.style.cursor = isHorizontal ? "row-resize" : "col-resize";
    document.body.style.userSelect = "none";
    return () => {
      if (pending != null) cancelAnimationFrame(pending);
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
      document.body.style.cursor = prevCursor;
      document.body.style.userSelect = prevSelect;
    };
  }, [resizing, isHorizontal]);

  useEffect(() => {
    if (resizing) return;
    localStorage.setItem(WIDTH_STORAGE_KEY, String(width));
  }, [width, resizing]);

  useEffect(() => {
    if (resizing) return;
    localStorage.setItem(HEIGHT_STORAGE_KEY, String(height));
  }, [height, resizing]);

  if (ticketId == null) return null;

  const overlay = fullscreen || isMobile;
  const paneClass = overlay
    ? "fixed inset-0 z-40 flex flex-col bg-bg"
    : isHorizontal
      ? "relative flex flex-col border-t border-border bg-bg"
      : "relative flex flex-col border-l border-border bg-bg";
  const paneStyle = overlay
    ? undefined
    : isHorizontal
      ? { height: `${height}px`, flex: `0 0 ${height}px` }
      : { width: `${width}px`, flex: `0 0 ${width}px` };

  return (
    <aside className={paneClass} style={paneStyle}>
      {!overlay && (
        // biome-ignore lint/a11y/useSemanticElements: HTML has no semantic resizer; role="separator" is the canonical ARIA pattern.
        <div
          role="separator"
          aria-orientation={isHorizontal ? "horizontal" : "vertical"}
          aria-label="Resize session pane"
          aria-valuenow={isHorizontal ? height : width}
          aria-valuemin={isHorizontal ? MIN_HEIGHT : MIN_WIDTH}
          aria-valuemax={isHorizontal ? MAX_HEIGHT : MAX_WIDTH}
          tabIndex={0}
          onMouseDown={(e) => {
            e.preventDefault();
            setResizing(true);
          }}
          onKeyDown={(e) => {
            const step = e.shiftKey ? 64 : 16;
            // Pane sits on the right (vertical) or bottom (horizontal); the
            // handle is on the leading edge, so "grow" pulls the handle away
            // from the pane: left for vertical, up for horizontal.
            const grow = isHorizontal ? "ArrowUp" : "ArrowLeft";
            const shrink = isHorizontal ? "ArrowDown" : "ArrowRight";
            const min = isHorizontal ? MIN_HEIGHT : MIN_WIDTH;
            const max = isHorizontal ? MAX_HEIGHT : MAX_WIDTH;
            const current = isHorizontal ? height : width;
            let next = current;
            if (e.key === grow) next += step;
            else if (e.key === shrink) next -= step;
            else if (e.key === "Home") next = min;
            else if (e.key === "End") next = max;
            else return;
            e.preventDefault();
            next = Math.min(max, Math.max(min, next));
            if (isHorizontal) setHeight(next);
            else setWidth(next);
          }}
          onDoubleClick={() => (isHorizontal ? setHeight(DEFAULT_HEIGHT) : setWidth(DEFAULT_WIDTH))}
          className={
            isHorizontal
              ? `absolute left-0 top-0 z-20 h-1 w-full -translate-y-1/2 cursor-row-resize hover:bg-accent-500/40 focus-visible:bg-accent-500/60 focus-visible:outline-none ${
                  resizing ? "bg-accent-500/60" : ""
                }`
              : `absolute left-0 top-0 z-20 h-full w-1 -translate-x-1/2 cursor-col-resize hover:bg-accent-500/40 focus-visible:bg-accent-500/60 focus-visible:outline-none ${
                  resizing ? "bg-accent-500/60" : ""
                }`
          }
        />
      )}
      <SessionView
        ticketId={ticketId}
        boardId={boardId}
        baseBranch={baseBranch}
        mergeConfig={mergeConfig}
        syncConfig={syncConfig}
        sessionIdByTicket={sessionIdByTicket}
        onClose={onClose}
        shortcutsEnabled
        headerExtras={
          !isMobile && (
            <Button
              variant="neutral"
              size="icon"
              onClick={() => setFullscreen((v) => !v)}
              aria-label={fullscreen ? "Exit fullscreen" : "Fullscreen"}
              title={fullscreen ? "Exit fullscreen (Esc)" : "Fullscreen"}
            >
              {fullscreen ? <FullscreenExitIcon /> : <FullscreenEnterIcon />}
            </Button>
          )
        }
      />
    </aside>
  );
}
