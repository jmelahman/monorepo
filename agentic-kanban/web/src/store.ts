import { useCallback, useSyncExternalStore } from "react";
import { api } from "@/api/client";
import type {
  Board,
  Column,
  MergeConfig,
  PullProgress,
  Session,
  SyncConfig,
  Ticket,
} from "@/api/client";
import { clearSessionTab } from "@/storage";

// Per-id subscriptions: a render that reads `useEntity(store, id)` only
// re-runs when that one id's value changes. By construction, updating ticket 5
// notifies only ticket-5 subscribers — no cascade through Board/Column.
export class EntityStore<T> {
  private values = new Map<number, T>();
  private idListeners = new Map<number, Set<() => void>>();

  get = (id: number): T | undefined => this.values.get(id);

  set = (id: number, value: T): void => {
    if (this.values.get(id) === value) return;
    this.values.set(id, value);
    const listeners = this.idListeners.get(id);
    if (listeners) for (const l of listeners) l();
  };

  delete = (id: number): void => {
    if (!this.values.has(id)) return;
    this.values.delete(id);
    const listeners = this.idListeners.get(id);
    if (listeners) for (const l of listeners) l();
  };

  subscribe = (id: number, cb: () => void): (() => void) => {
    let set = this.idListeners.get(id);
    if (!set) {
      set = new Set();
      this.idListeners.set(id, set);
    }
    set.add(cb);
    return () => {
      set!.delete(cb);
    };
  };

  // Reconciliation hook: walk every (id, value) pair so callers can drop
  // entries that no longer belong (e.g. tickets removed from a board).
  forEach = (cb: (value: T, id: number) => void): void => {
    this.values.forEach(cb);
  };
}

// A single value with subscribe/select. `useScalarSelector` is the hot path:
// each subscriber renders only when its derived selection changes (`Object.is`).
export class ScalarStore<T> {
  private value: T;
  private listeners = new Set<() => void>();

  constructor(initial: T) {
    this.value = initial;
  }

  get = (): T => this.value;

  set = (next: T | ((prev: T) => T)): void => {
    const v = typeof next === "function" ? (next as (p: T) => T)(this.value) : next;
    if (Object.is(v, this.value)) return;
    this.value = v;
    for (const l of this.listeners) l();
  };

  subscribe = (cb: () => void): (() => void) => {
    this.listeners.add(cb);
    return () => {
      this.listeners.delete(cb);
    };
  };
}

export function useEntity<T>(store: EntityStore<T>, id: number): T | undefined {
  const subscribe = useCallback((cb: () => void) => store.subscribe(id, cb), [store, id]);
  const getSnapshot = useCallback(() => store.get(id), [store, id]);
  return useSyncExternalStore(subscribe, getSnapshot);
}

export function useScalarStore<T>(store: ScalarStore<T>): T {
  return useSyncExternalStore(store.subscribe, store.get);
}

export function useScalarSelector<T, S>(store: ScalarStore<T>, select: (v: T) => S): S {
  const getSnapshot = useCallback(() => select(store.get()), [store, select]);
  return useSyncExternalStore(store.subscribe, getSnapshot);
}

// Singletons. Tickets and sessions live here; the boardStructure query in
// React Query holds only ids and column/session shape so structural changes
// (column moves, archives, creates) refetch independently of content edits.
export const ticketStore = new EntityStore<Ticket>();
export const sessionStore = new EntityStore<Session>();
// Transient per-session image-pull progress. Populated by SSE events while a
// session is starting; cleared once the pull completes or the session
// transitions out of "starting".
export const pullProgressStore = new EntityStore<PullProgress>();
export const activeTicketStore = new ScalarStore<number | null>(null);
// Bumped by the "new ticket" shortcut to ask a Column to open its add form.
// Holds the target column id; Columns reset it to null after reacting.
export const addTicketRequestStore = new ScalarStore<number | null>(null);

// Per-ticket DOM slots that the global TerminalsRoot portals PtyTerminals
// into. Keyed by ticket id so the single PtyTerminal that exists per session
// lands in whichever SessionView currently owns the visible slot for that
// ticket. Board view and Overview view are mutually exclusive at the
// top level, so a ticket has at most one live slot at any moment.
export type TerminalSlot = {
  agent: HTMLDivElement | null;
  shell: HTMLDivElement | null;
};
export const terminalSlotStore = new EntityStore<TerminalSlot>();

export function useTerminalSlot(ticketId: number | null | undefined): TerminalSlot | undefined {
  return useEntity(terminalSlotStore, ticketId ?? -1);
}

// Set or clear one kind of slot for a ticket without clobbering the other.
// Drops the entry once both kinds are null so we don't keep empty records.
export function setTerminalSlot(
  ticketId: number,
  kind: "agent" | "shell",
  el: HTMLDivElement | null,
): void {
  const prev = terminalSlotStore.get(ticketId);
  const next: TerminalSlot = {
    agent: prev?.agent ?? null,
    shell: prev?.shell ?? null,
    [kind]: el,
  };
  if (next.agent == null && next.shell == null) {
    terminalSlotStore.delete(ticketId);
    return;
  }
  terminalSlotStore.set(ticketId, next);
}

export function useTicket(id: number): Ticket | undefined {
  return useEntity(ticketStore, id);
}

export function useSession(id: number | null | undefined): Session | undefined {
  return useEntity(sessionStore, id ?? -1);
}

export function usePullProgress(id: number | null | undefined): PullProgress | undefined {
  return useEntity(pullProgressStore, id ?? -1);
}

export function useActiveTicketId(): number | null {
  return useScalarStore(activeTicketStore);
}

export function useIsActiveTicket(id: number): boolean {
  return useScalarSelector(activeTicketStore, (v) => v === id);
}

// BoardStructure is the *shape* of a board: which columns exist and which
// ticket-ids live in each, in render order. The actual ticket and session
// content lives in `ticketStore` / `sessionStore` so per-id subscribers
// re-render in isolation. Keep this object purely structural — anything that
// changes per content edit (title, status, body, …) must NOT live here.
export type BoardStructure = {
  board: Board;
  columns: Column[];
  ticketIdsByColumn: Record<number, number[]>;
  sessionIdByTicket: Record<number, number>;
  merge_config: MergeConfig;
  sync_config: SyncConfig;
};

// fetchBoardStructure: fetch the server snapshot, fan content out into the
// entity stores, return the structural index. Reconciliation: any ticket
// previously in `ticketStore` for this board that isn't in the new snapshot
// (archived / deleted in another tab) is dropped, and so is its session.
export async function fetchBoardStructure(boardId: number): Promise<BoardStructure> {
  const data = await api.boardState(boardId);

  const newTicketIds = new Set(data.tickets.map((t) => t.id));
  for (const t of data.tickets) ticketStore.set(t.id, t);
  ticketStore.forEach((t, id) => {
    if (t.board_id === boardId && !newTicketIds.has(id)) ticketStore.delete(id);
  });

  const newSessionIds = new Set(data.sessions.map((s) => s.id));
  // Reconciling two writers: optimistic UI that's ahead of the server (e.g.
  // startMut.onMutate set "starting" while the snapshot still shows
  // "stopped"), and the authoritative server state that the store can fall
  // behind on (SSE dropped by a full 16-slot per-subscriber buffer during a
  // pull-progress burst, SessionView unmounted before startMut.onSuccess
  // fired so the response-based reconcile never ran, or the user was
  // off-board while the transition happened).
  //
  // started_at / stopped_at / container_id only advance when Manager.Start
  // or Manager.Stop commits a lifecycle change, and Claude-hook status
  // updates (working/idle/awaiting_perm) leave them untouched. So matching
  // markers mean the snapshot represents the same lifecycle generation we
  // already have — trust the store, including any optimistic status.
  // Differing markers mean the server moved forward and we missed it —
  // trust the snapshot so the store self-heals without a page reload.
  for (const s of data.sessions) {
    const existing = sessionStore.get(s.id);
    if (!existing) {
      sessionStore.set(s.id, s);
      continue;
    }
    const lifecycleAdvanced =
      (s.started_at ?? 0) !== (existing.started_at ?? 0) ||
      (s.stopped_at ?? 0) !== (existing.stopped_at ?? 0) ||
      (s.container_id ?? "") !== (existing.container_id ?? "");
    // Manager.Start's failure paths (manager.go:144,191) flip status to
    // "error" via UpdateSessionStatus alone — no lifecycle column advance.
    // Without this branch we'd ignore a server-side error and keep an
    // optimistic "starting" pinned indefinitely.
    const serverFailedTransient =
      (existing.status === "starting" || existing.status === "stopping") && s.status === "error";
    if (lifecycleAdvanced || serverFailedTransient) {
      sessionStore.set(s.id, s);
      continue;
    }
    // Lifecycle in sync; refresh PR fields (written by the GitHub poller, a
    // separate writer the snapshot is always authoritative for).
    if (
      existing.pr_state !== s.pr_state ||
      existing.pr_number !== s.pr_number ||
      existing.pr_url !== s.pr_url ||
      existing.pr_title !== s.pr_title
    ) {
      sessionStore.set(s.id, {
        ...existing,
        pr_state: s.pr_state,
        pr_number: s.pr_number,
        pr_url: s.pr_url,
        pr_title: s.pr_title,
      });
    }
  }
  // Symmetric to the seed guard above: only drop sessions whose ticket has
  // actually disappeared. The ticket reconciliation immediately above is
  // authoritative, so a missing ticket reliably means archive/delete and the
  // session is orphan. A session whose ticket still exists but is absent from
  // *this* snapshot is the stale-snapshot race — e.g. a refetch that started
  // before ensureSession committed lands after we set the new session locally.
  // Trust local state and let SSE drive any real deletion.
  sessionStore.forEach((s, id) => {
    if (newSessionIds.has(id)) return;
    if (!ticketStore.get(s.ticket_id)) {
      sessionStore.delete(id);
      // Drop the session's remembered-tab key so it doesn't outlive the
      // session (archive/delete). One key per session id, so without this
      // they accumulate in localStorage forever.
      clearSessionTab(id);
    }
  });

  const ticketIdsByColumn: Record<number, number[]> = {};
  for (const c of data.columns) ticketIdsByColumn[c.id] = [];
  const sorted = [...data.tickets].sort((a, b) => a.position - b.position);
  for (const t of sorted) {
    const arr = ticketIdsByColumn[t.column_id];
    if (arr) arr.push(t.id);
  }

  const sessionIdByTicket: Record<number, number> = {};
  for (const s of data.sessions) sessionIdByTicket[s.ticket_id] = s.id;

  return {
    board: data.board,
    columns: data.columns,
    ticketIdsByColumn,
    sessionIdByTicket,
    merge_config: data.merge_config,
    sync_config: data.sync_config,
  };
}
