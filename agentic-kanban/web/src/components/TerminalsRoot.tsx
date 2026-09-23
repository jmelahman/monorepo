import { useQueryClient } from "@tanstack/react-query";
import type { QueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { sessionStore, useTerminalSlot } from "@/store";
import type { BoardStructure } from "@/store";
import { PtyTerminal } from "./PtyTerminal";

const ATTACHABLE = new Set(["idle", "working", "awaiting_perm"]);

// Renders one PtyTerminal per known session, regardless of which view is
// hosting that session's UI. Each PtyTerminal portals into the slot DOM
// node registered in `terminalSlotStore` for its ticket — set by whichever
// SessionView (board pane or overview panel) currently owns the visible
// slot. If no slot is registered, PtyTerminal stays attached to its
// off-screen container, keeping the WebSocket alive without rendering.
//
// Source of truth for which sessions to mount is React Query's per-board
// structure cache: every loaded BoardStructure contributes its session ids.
// This means the agent terminal keeps streaming even when the user closes
// a panel and re-opens it later, and switching tickets in the board view
// never tears down the websocket.
export function TerminalsRoot() {
  const sessionIds = useAllKnownSessionIds();
  return (
    <>
      {sessionIds.map((id) => (
        <SessionTerminals key={id} sessionId={id} />
      ))}
    </>
  );
}

function SessionTerminals({ sessionId }: { sessionId: number }) {
  // Read live session state per id; reacts to status transitions without
  // re-rendering the parent map.
  const session = useSessionEntity(sessionId);
  const slot = useTerminalSlot(session?.ticket_id ?? null);
  // Mirror the original Board.tsx logic: only mount the shell PTY once it's
  // been requested at least once for this session.
  const [shellEverOpened, setShellEverOpened] = useState(false);
  useEffect(() => {
    if (slot?.shell) setShellEverOpened(true);
  }, [slot?.shell]);
  if (!session || !ATTACHABLE.has(session.status)) return null;
  return (
    <>
      <PtyTerminal
        key={`agent:${session.started_at ?? 0}`}
        sessionId={session.id}
        kind="agent"
        mountTarget={slot?.agent ?? null}
      />
      {shellEverOpened && (
        <PtyTerminal
          key={`shell:${session.started_at ?? 0}`}
          sessionId={session.id}
          kind="shell"
          mountTarget={slot?.shell ?? null}
        />
      )}
    </>
  );
}

function useSessionEntity(id: number) {
  return useSyncExternalStore(
    (cb) => sessionStore.subscribe(id, cb),
    () => sessionStore.get(id),
  );
}

// Aggregates session ids across every loaded board structure in React
// Query's cache. Re-evaluates when any board query is added, removed, or
// updated.
function useAllKnownSessionIds(): number[] {
  const qc = useQueryClient();
  // Cache the latest snapshot reference so getSnapshot returns the *same*
  // array between calls when nothing has changed. useSyncExternalStore
  // requires snapshot identity stability or it emits the "Cannot update
  // a component while rendering" warning during cache notification storms.
  const cacheRef = useRef<number[]>([]);
  const subscribe = useMemo(() => (cb: () => void) => qc.getQueryCache().subscribe(cb), [qc]);
  const getSnapshot = useMemo(
    () => () => {
      const next = snapshotSessionIds(qc);
      if (sameIds(cacheRef.current, next)) return cacheRef.current;
      cacheRef.current = next;
      return next;
    },
    [qc],
  );
  return useSyncExternalStore(subscribe, getSnapshot);
}

function snapshotSessionIds(qc: QueryClient): number[] {
  const ids = new Set<number>();
  const queries = qc.getQueryCache().findAll({ queryKey: ["board"] });
  for (const q of queries) {
    const data = q.state.data as BoardStructure | undefined;
    if (!data) continue;
    for (const sid of Object.values(data.sessionIdByTicket)) ids.add(sid);
  }
  return [...ids].sort((a, b) => a - b);
}

function sameIds(a: number[], b: number[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
  return true;
}
