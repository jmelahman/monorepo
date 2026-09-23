import { useCallback, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "@/api/keys";
import { fetchBoardStructure } from "@/store";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { SessionView } from "@/components/SessionView";
import { BoardTree } from "./BoardTree";
import { PanelCanvas, type PanelCanvasHandle } from "./PanelCanvas";

const SIDEBAR_WIDTH_KEY = "overview.sidebar.width";
const SIDEBAR_COLLAPSED_KEY = "overview.sidebar.collapsed";
const DEFAULT_SIDEBAR = 280;
const MIN_SIDEBAR = 200;
const MAX_SIDEBAR = 600;

function loadSidebarWidth(): number {
  try {
    const raw = localStorage.getItem(SIDEBAR_WIDTH_KEY);
    const n = raw ? Number(raw) : NaN;
    if (!Number.isFinite(n)) return DEFAULT_SIDEBAR;
    return Math.min(MAX_SIDEBAR, Math.max(MIN_SIDEBAR, n));
  } catch {
    return DEFAULT_SIDEBAR;
  }
}

function loadSidebarCollapsed(): boolean {
  try {
    return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "1";
  } catch {
    return false;
  }
}

export function Overview() {
  const isMobile = useMediaQuery("(max-width: 639px)");
  if (isMobile) return <MobileOverview />;
  return <DesktopOverview />;
}

function DesktopOverview() {
  const handleRef = useRef<PanelCanvasHandle | null>(null);
  const registerHandle = useCallback((h: PanelCanvasHandle | null) => {
    handleRef.current = h;
  }, []);
  const onOpenTicket = useCallback((boardId: number, ticketId: number) => {
    handleRef.current?.open(boardId, ticketId);
  }, []);

  const [openTicketIds, setOpenTicketIds] = useState<ReadonlySet<number>>(() => new Set());

  const [sidebarWidth, setSidebarWidth] = useState(loadSidebarWidth);
  const [resizing, setResizing] = useState(false);
  const [collapsed, setCollapsed] = useState(loadSidebarCollapsed);

  const toggleCollapsed = () => {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(SIDEBAR_COLLAPSED_KEY, next ? "1" : "0");
      } catch {
        // ignore
      }
      return next;
    });
  };

  const persistWidth = (w: number) => {
    try {
      localStorage.setItem(SIDEBAR_WIDTH_KEY, String(w));
    } catch {
      // ignore
    }
  };

  const onResizeStart = (e: React.MouseEvent) => {
    if (collapsed) return;
    e.preventDefault();
    setResizing(true);
    let latest = sidebarWidth;
    const onMove = (ev: MouseEvent) => {
      latest = Math.min(MAX_SIDEBAR, Math.max(MIN_SIDEBAR, ev.clientX));
      setSidebarWidth(latest);
    };
    const onUp = () => {
      setResizing(false);
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
      persistWidth(latest);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  };

  const onResizeKey = (e: React.KeyboardEvent) => {
    const step = e.shiftKey ? 64 : 16;
    let next = sidebarWidth;
    if (e.key === "ArrowLeft") next -= step;
    else if (e.key === "ArrowRight") next += step;
    else if (e.key === "Home") next = MIN_SIDEBAR;
    else if (e.key === "End") next = MAX_SIDEBAR;
    else return;
    e.preventDefault();
    next = Math.min(MAX_SIDEBAR, Math.max(MIN_SIDEBAR, next));
    setSidebarWidth(next);
    persistWidth(next);
  };

  return (
    <div className="flex h-full">
      {collapsed ? (
        <button
          type="button"
          onClick={toggleCollapsed}
          title="Show boards sidebar"
          aria-label="Show boards sidebar"
          aria-expanded={false}
          className="flex h-full w-6 shrink-0 items-center justify-center border-r border-border bg-bg text-fg-muted hover:bg-surface-2 hover:text-fg"
        >
          <span aria-hidden>▶</span>
        </button>
      ) : (
        <>
          <aside
            style={{ width: `${sidebarWidth}px`, flex: `0 0 ${sidebarWidth}px` }}
            className="flex h-full flex-col border-r border-border bg-bg"
          >
            <BoardTree
              onOpenTicket={onOpenTicket}
              openTicketIds={openTicketIds}
              onCollapseSidebar={toggleCollapsed}
            />
          </aside>
          {/* biome-ignore lint/a11y/useSemanticElements: HTML has no semantic resizer; role="separator" is the canonical ARIA pattern. */}
          <div
            role="separator"
            aria-orientation="vertical"
            aria-label="Resize sidebar"
            aria-valuenow={sidebarWidth}
            aria-valuemin={MIN_SIDEBAR}
            aria-valuemax={MAX_SIDEBAR}
            tabIndex={0}
            onMouseDown={onResizeStart}
            onKeyDown={onResizeKey}
            className={`w-1 cursor-col-resize hover:bg-accent-500/40 focus-visible:bg-accent-500/60 focus-visible:outline-none ${resizing ? "bg-accent-500/60" : ""}`}
          />
        </>
      )}
      <div className="min-w-0 flex-1">
        <PanelCanvas registerHandle={registerHandle} onOpenTicketsChange={setOpenTicketIds} />
      </div>
    </div>
  );
}

type MobileTicket = { boardId: number; ticketId: number };

function MobileOverview() {
  const [active, setActive] = useState<MobileTicket | null>(null);
  const openIds = active ? new Set([active.ticketId]) : new Set<number>();

  if (active) {
    return (
      <MobileSession
        boardId={active.boardId}
        ticketId={active.ticketId}
        onBack={() => setActive(null)}
      />
    );
  }
  return (
    <div className="flex h-full flex-col bg-bg">
      <BoardTree
        onOpenTicket={(boardId, ticketId) => setActive({ boardId, ticketId })}
        openTicketIds={openIds}
      />
    </div>
  );
}

function MobileSession({
  boardId,
  ticketId,
  onBack,
}: {
  boardId: number;
  ticketId: number;
  onBack: () => void;
}) {
  const boardQ = useQuery({
    queryKey: queryKeys.board(boardId),
    queryFn: () => fetchBoardStructure(boardId),
  });
  const s = boardQ.data;
  if (!s) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-fg-muted">
        Loading board…
      </div>
    );
  }
  return (
    <div className="h-full">
      <SessionView
        ticketId={ticketId}
        boardId={boardId}
        baseBranch={s.board.base_branch}
        mergeConfig={s.merge_config}
        syncConfig={s.sync_config}
        sessionIdByTicket={s.sessionIdByTicket}
        onClose={onBack}
      />
    </div>
  );
}
