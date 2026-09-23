import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { type PullProgress, type Session, subscribeBoard, type Ticket } from "@/api/client";
import { queryKeys } from "@/api/keys";
import { activeTicketStore, pullProgressStore, sessionStore, ticketStore } from "@/store";

export type StreamStatus = "open" | "error" | "closed";

// Subscribes to one board's SSE stream and routes events into the relevant
// caches. Safe to call from multiple components and across many board ids:
// all subscribers share a single underlying EventSource via the multiplexed
// /api/events?boards=… endpoint, so the browser's HTTP/1.1 per-origin
// connection pool (6 connections on Chrome/Firefox) never saturates — even
// when Overview is showing dozens of boards at once.
export function useBoardSubscription(
  boardId: number | null,
  onStatus?: (s: StreamStatus) => void,
): void {
  const qc = useQueryClient();
  useEffect(() => {
    if (boardId == null) return;
    const key = queryKeys.board(boardId);
    return subscribeBoard(boardId, {
      onEvent: (type, data) => {
        applyBoardEvent(boardId, type, data, () => qc.invalidateQueries({ queryKey: key }));
        if (
          type === "ticket_archived" ||
          type === "ticket_unarchived" ||
          type === "ticket_deleted"
        ) {
          qc.invalidateQueries({ queryKey: queryKeys.archived(boardId) });
        }
      },
      onStatus: onStatus,
    });
  }, [boardId, qc, onStatus]);
}

// Routes an SSE event to the right cache. Per-id content updates flow
// straight to the entity stores (no cascade). Structural changes — anything
// that adds, removes, or reorders ticket-ids in a column — invalidate the
// boardStructure query so it refetches and rebuilds the index.
function applyBoardEvent(
  boardId: number,
  type: string,
  data: unknown,
  invalidateStructure: () => void,
): void {
  switch (type) {
    case "ticket_updated": {
      const t = data as Ticket | null;
      if (t) ticketStore.set(t.id, t);
      return;
    }
    case "ticket_created":
    case "ticket_moved":
    case "ticket_unarchived": {
      const t = data as Ticket | null;
      if (t) ticketStore.set(t.id, t);
      invalidateStructure();
      return;
    }
    case "ticket_archived":
    case "ticket_deleted": {
      const t = data as Ticket | null;
      if (t && activeTicketStore.get() === t.id && t.board_id === boardId) {
        activeTicketStore.set(null);
      }
      invalidateStructure();
      return;
    }
    case "session_updated": {
      const s = data as Session | null;
      if (!s) return;
      const prev = sessionStore.get(s.id);
      // Filter stale events that would clobber an optimistic
      // "starting"/"stopping" with a status the server has already moved
      // past. Two known publishers send these:
      //  - Claude Code status hooks or the GitHub PR poller firing out of
      //    the dying container during a Restart's Stop phase, before
      //    manager.Stop clears container_id (manager.go:289-309). They
      //    publish session_updated{status: idle, container_id: OLD}.
      //  - The ensure endpoint publishing {status: stopped, container_id:""}
      //    immediately before startMut sets optimistic "starting"; if SSE
      //    delivers the ensure event after the optimistic flip it would
      //    revert us to "stopped".
      // Both are distinguishable from real lifecycle progress: a real
      // Manager.Start advances started_at, a real Manager.Stop advances
      // stopped_at. Stale events keep both markers where they were. Trust
      // the optimistic state until a marker actually moves.
      if (
        prev &&
        (prev.status === "starting" || prev.status === "stopping") &&
        s.status !== prev.status &&
        (s.started_at ?? 0) <= (prev.started_at ?? 0) &&
        (s.stopped_at ?? 0) <= (prev.stopped_at ?? 0)
      ) {
        return;
      }
      sessionStore.set(s.id, s);
      // Pull progress is only meaningful while the session is starting; clear
      // it on any other status so the bar doesn't linger after a stop/restart.
      if (s.status !== "starting") pullProgressStore.delete(s.id);
      // New session: ticket → session map needs rebuild.
      if (!prev) invalidateStructure();
      return;
    }
    case "session_pull_progress": {
      const p = data as PullProgress | null;
      if (!p) return;
      if (p.done) {
        pullProgressStore.delete(p.session_id);
      } else {
        pullProgressStore.set(p.session_id, p);
      }
      return;
    }
    default:
      // "ready" and any unknown event types — no state to update.
      return;
  }
}
