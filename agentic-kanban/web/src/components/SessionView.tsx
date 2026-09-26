import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  type ReactNode,
  Suspense,
  lazy,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  api,
  formatApiError,
  type MergeConfig,
  PR_STATE_COLOR,
  type PRState,
  type PullProgress,
  type SyncConfig,
} from "@/api/client";
import { queryKeys } from "@/api/keys";
import { useSessionDiff } from "@/hooks/useSessionDiff";
import { sessionStore, setTerminalSlot, usePullProgress, useSession, useTicket } from "@/store";
import { useToast } from "@/toast";
import { useClickOutside } from "@/hooks/useClickOutside";
import { useShortcut } from "@/keys/useShortcut";
import { readSessionTab, writeSessionTab } from "@/storage";
import {
  ArchiveIcon,
  CheckIcon,
  MergeIcon,
  PlayIcon,
  PlusIcon,
  PullIcon,
  RestartIcon,
  StopIcon,
  XIcon,
} from "@/icons";
import { ActionButton } from "./ActionButton";
import { Button } from "./Button";
import { InfoPanel } from "./InfoPanel";
import { PlansPanel } from "./PlansPanel";
import { PreviewsPanel } from "./PreviewsPanel";
import { Tab } from "./Tab";
import { TasksPanel } from "./TasksPanel";

// DiffPanel pulls in @pierre/diffs + Shiki; lazy-load it so that weight only
// lands when the diff tab is first opened, not in the initial bundle. The
// importer is shared so SessionView can prefetch the chunk in the background
// (see the effect below) — by the time the user opens the diff tab the JS is
// already parsed, not downloading behind the Suspense fallback.
const importDiffPanel = () => import("./DiffPanel");
const DiffPanel = lazy(() => importDiffPanel().then((m) => ({ default: m.DiffPanel })));

const TAB_ORDER = ["agent", "shell", "diff", "tasks", "previews", "plans", "info"] as const;
type TabId = (typeof TAB_ORDER)[number];

function loadInitialTab(sessionId: number | null): TabId {
  if (sessionId == null) return "agent";
  const raw = readSessionTab(sessionId);
  return (TAB_ORDER as readonly string[]).includes(raw ?? "") ? (raw as TabId) : "agent";
}

type MergeStrategy = "merge-commit" | "squash" | "rebase";

const MERGE_STRATEGY_LABELS: Record<MergeStrategy, string> = {
  "merge-commit": "create a merge commit",
  squash: "squash and merge",
  rebase: "rebase and merge",
};

function enabledMergeStrategies(cfg: MergeConfig): MergeStrategy[] {
  const out: MergeStrategy[] = [];
  if (cfg.allow_merge_commit) out.push("merge-commit");
  if (cfg.allow_squash) out.push("squash");
  if (cfg.allow_rebase) out.push("rebase");
  // The board's default leads the menu: it's what the CLI and MCP pick when
  // no strategy is named, so the UI shouldn't disagree about what's usual.
  const def = cfg.default_strategy as MergeStrategy;
  if (out.includes(def)) return [def, ...out.filter((s) => s !== def)];
  return out;
}

type SyncStrategy = "rebase" | "merge";

const SYNC_STRATEGY_LABELS: Record<SyncStrategy, string> = {
  rebase: "rebase from",
  merge: "merge from",
};

function enabledSyncStrategies(cfg: SyncConfig): SyncStrategy[] {
  const out: SyncStrategy[] = [];
  if (cfg.allow_rebase) out.push("rebase");
  if (cfg.allow_merge) out.push("merge");
  return out;
}

export type SessionViewProps = {
  ticketId: number;
  boardId: number;
  baseBranch: string;
  mergeConfig: MergeConfig;
  syncConfig: SyncConfig;
  sessionIdByTicket: Record<number, number>;
  onClose: () => void;
  // When false, this view's keyboard shortcuts are disabled. Multiple
  // SessionViews can be visible at once (overview canvas) — only the focused
  // one should react to start/stop/tab-cycling shortcuts.
  shortcutsEnabled?: boolean;
  // Trailing controls injected into the header (e.g. a fullscreen toggle in
  // the board pane). Rendered after the close button slot.
  headerExtras?: ReactNode;
};

// Inner content of the session pane: the action header, tab bar, and tab
// body. Owns its own ticket via props (no activeTicketStore dependency) and
// publishes its terminal slot DOM nodes into the global terminalSlotStore so
// the central TerminalsRoot can portal the right PtyTerminal in.
export function SessionView({
  ticketId,
  boardId,
  baseBranch,
  mergeConfig,
  syncConfig,
  sessionIdByTicket,
  onClose,
  shortcutsEnabled = true,
  headerExtras,
}: SessionViewProps) {
  const sessionId = sessionIdByTicket[ticketId] ?? null;
  const session = useSession(sessionId) ?? null;
  const ticket = useTicket(ticketId);
  const pullProgress = usePullProgress(sessionId);
  const qc = useQueryClient();
  const toast = useToast();
  const [tab, setTab] = useState<TabId>(() => loadInitialTab(sessionId));
  const tabsEnabled = shortcutsEnabled;

  // The Plans tab only appears when the session's plans directory has at
  // least one .md file. The poll catches files created mid-session (e.g.
  // via Claude Code's /plan mode) without a manual refresh.
  const plansQ = useQuery({
    queryKey: queryKeys.sessionPlans(sessionId ?? 0),
    queryFn: () => api.listSessionPlans(sessionId as number),
    enabled: sessionId != null,
    refetchInterval: 5000,
  });
  const hasPlans = (plansQ.data?.length ?? 0) > 0;

  // The Diff tab needs a worktree to diff; it appears once the session has one.
  const hasWorktree = !!session?.worktree_path;

  // Visible tabs follow `TAB_ORDER` minus any conditionally-hidden ones.
  // Shortcuts and the bounce-on-hidden effect both consult this so a
  // hidden tab is treated as if it never existed.
  const visibleTabs = useMemo<readonly TabId[]>(
    () => TAB_ORDER.filter((t) => (t !== "plans" || hasPlans) && (t !== "diff" || hasWorktree)),
    [hasPlans, hasWorktree],
  );

  // Keep the diff warm in the background while the user is on another tab, so
  // opening the diff tab renders from cache instead of a cold fetch. Gated on
  // `shortcutsEnabled` (the focused view) so the multi-panel overview doesn't
  // fire a `git diff` subprocess per open panel — only the ticket in focus
  // warms. While the diff tab is open DiffPanel owns the (faster) poll, so this
  // backs off to avoid a duplicate request racing the same cache entry.
  useSessionDiff(sessionId ?? 0, {
    enabled: shortcutsEnabled && hasWorktree && sessionId != null && tab !== "diff",
    active: false,
  });

  // Prefetch the (heavy) DiffPanel chunk once a worktree exists on the focused
  // view, so the diff tab doesn't sit behind the Suspense fallback while the JS
  // downloads on first open. Fire-and-forget; lazy() dedupes the import.
  useEffect(() => {
    if (shortcutsEnabled && hasWorktree) void importDiffPanel();
  }, [shortcutsEnabled, hasWorktree]);

  useShortcut(
    "tab.next",
    () =>
      setTab((t) => {
        const i = visibleTabs.indexOf(t);
        return visibleTabs[(i + 1) % visibleTabs.length];
      }),
    { enabled: tabsEnabled },
  );
  useShortcut(
    "tab.prev",
    () =>
      setTab((t) => {
        const i = visibleTabs.indexOf(t);
        return visibleTabs[(i - 1 + visibleTabs.length) % visibleTabs.length];
      }),
    { enabled: tabsEnabled },
  );

  // If the persisted tab points at a hidden one (e.g. user was on plans
  // and then deleted every .md file), fall back to a sensible default.
  useEffect(() => {
    if (!visibleTabs.includes(tab)) setTab(visibleTabs[0] ?? "agent");
  }, [tab, visibleTabs]);
  const [syncMenuOpen, setSyncMenuOpen] = useState(false);
  const syncMenuRef = useRef<HTMLDivElement | null>(null);
  const [mergeMenuOpen, setMergeMenuOpen] = useState(false);
  const mergeMenuRef = useRef<HTMLDivElement | null>(null);

  const [compact, setCompact] = useState(false);
  const headerRef = useRef<HTMLDivElement | null>(null);
  const headerGhostRef = useRef<HTMLDivElement | null>(null);
  const autoStartedRef = useRef<number | null>(null);

  // Reload the persisted tab when the sessionId changes — each session has
  // its own tab memory under sessionPane.tab.{sessionId}.
  useEffect(() => {
    setTab(loadInitialTab(sessionId));
  }, [sessionId]);

  useEffect(() => {
    if (sessionId == null) return;
    writeSessionTab(sessionId, tab);
  }, [tab, sessionId]);

  useClickOutside(syncMenuRef, () => setSyncMenuOpen(false), {
    enabled: syncMenuOpen,
  });
  useClickOutside(mergeMenuRef, () => setMergeMenuOpen(false), {
    enabled: mergeMenuOpen,
  });

  const boardKey = queryKeys.board(boardId);

  // Optimistic status updates flow straight into the session entity store —
  // only `Ticket`s for this session re-render. Returns the snapshot to roll
  // back on error.
  const optimisticStatus = (id: number, status: string) => {
    const prev = sessionStore.get(id);
    if (!prev) return { prev: null };
    sessionStore.set(id, { ...prev, status });
    return { prev };
  };
  const rollbackStatus = (id: number, prev: typeof session) => {
    if (prev) sessionStore.set(id, prev);
  };

  // Defensive `onSuccess`: the bus drops events when its 16-slot per-subscriber
  // channel fills up, which a burst of `session_pull_progress` can cause
  // mid-start. Without this, a dropped `session_updated("idle")` would leave
  // the entry stuck at the optimistic "starting" forever — only a page refresh
  // (which empties the store and reseeds from the server) would fix it. Apply
  // the response only when the store is still on our optimistic "starting" so
  // we don't clobber a newer SSE update (e.g. "working" from the agent hook).
  const reconcileFromStartResponse = (sess: typeof session) => {
    if (!sess) return;
    const cur = sessionStore.get(sess.id);
    if (!cur || cur.status === "starting") sessionStore.set(sess.id, sess);
  };

  const startMut = useMutation({
    mutationFn: (id: number) => api.startSession(id),
    onMutate: (id) => optimisticStatus(id, "starting"),
    onSuccess: (sess) => reconcileFromStartResponse(sess),
    onError: (_err, id, ctx) => rollbackStatus(id, ctx?.prev ?? null),
  });
  // No onError reset of autoStartedRef: doing so combined with `ensureMut` in
  // the auto-start effect's deps produces an infinite POST loop on persistent
  // errors (every error rerenders the mutation object, the effect re-runs,
  // sees a null ref, fires again). The error banner below + the manual
  // "create session" button cover retry; ticket switching resets via the
  // `ref !== ticketId` check.
  const ensureMut = useMutation({
    mutationFn: () => api.ensureSession(ticketId),
    onSuccess: (created) => {
      sessionStore.set(created.id, created);
      qc.invalidateQueries({ queryKey: boardKey });
      startMut.mutate(created.id);
    },
  });
  useEffect(() => {
    if (session) return;
    if (autoStartedRef.current === ticketId) return;
    autoStartedRef.current = ticketId;
    ensureMut.mutate();
  }, [ticketId, session, ensureMut]);

  // Active recovery for stuck "starting". The layered defenses (SSE filter,
  // reconcileFromStartResponse, fetchBoardStructure self-heal) each cover a
  // failure mode but compose only if a refetch is *ever* triggered after the
  // server's lifecycle advance. In Overview the structure query goes idle
  // after ensureMut.onSuccess's one-shot invalidate — nothing forces another
  // refetch, so if the {idle} SSE drops and startMut.onSuccess didn't fire
  // (observer detached, response error, etc.) we stay pinned at "starting"
  // until the user navigates away. Poll the structure every few seconds
  // while stuck; fetchBoardStructure's lifecycleAdvanced check flips the
  // entry the moment the server has actually moved forward, and is a no-op
  // otherwise.
  // Depend on `boardId` (stable primitive), not `boardKey` — queryKeys.board
  // returns a fresh array each call, so listing boardKey in deps would tear
  // down and rebuild the interval every render and it would never tick.
  useEffect(() => {
    if (session?.status !== "starting" || sessionId == null) return;
    const t = setInterval(() => {
      if (sessionStore.get(sessionId)?.status === "starting") {
        qc.invalidateQueries({ queryKey: queryKeys.board(boardId) });
      }
    }, 5_000);
    return () => clearInterval(t);
  }, [session?.status, sessionId, qc, boardId]);
  const stopMut = useMutation({
    mutationFn: () => api.stopSession(session!.id),
    onMutate: () => (session ? optimisticStatus(session.id, "stopping") : { prev: null }),
    onError: (_err, _vars, ctx) => {
      if (session) rollbackStatus(session.id, ctx?.prev ?? null);
    },
  });
  const restartMut = useMutation({
    mutationFn: () => api.restartSession(session!.id),
    onMutate: () => (session ? optimisticStatus(session.id, "starting") : { prev: null }),
    onSuccess: (sess) => {
      reconcileFromStartResponse(sess);
      toast.push("success", "container restarted");
    },
    onError: (_err, _vars, ctx) => {
      if (session) rollbackStatus(session.id, ctx?.prev ?? null);
    },
  });
  const archiveMut = useMutation({
    mutationFn: (id: number) => api.archiveTicket(id),
    onSuccess: (_data, archivedId) => {
      qc.invalidateQueries({ queryKey: boardKey });
      // The pane may have switched to a different ticket while the request was
      // in flight; only close if we're still viewing the ticket that was archived.
      if (archivedId === ticketId) onClose();
    },
  });
  useShortcut("ticket.archive", () => archiveMut.mutate(ticketId), { enabled: shortcutsEnabled });
  const syncMut = useMutation({
    mutationFn: (strategy: SyncStrategy) => api.syncTicket(ticketId, strategy),
    onSuccess: (_data, strategy) => {
      setSyncMenuOpen(false);
      toast.push("success", `${strategy} from ${baseBranch} succeeded`);
      qc.invalidateQueries({ queryKey: boardKey });
    },
    onError: () => {
      setSyncMenuOpen(false);
    },
  });
  const mergeMut = useMutation({
    mutationFn: (strategy: MergeStrategy) => api.mergeTicket(ticketId, strategy),
    onSuccess: (_data, strategy) => {
      setMergeMenuOpen(false);
      toast.push("success", `${strategy} into ${baseBranch} succeeded`);
      qc.invalidateQueries({ queryKey: boardKey });
    },
    onError: () => {
      setMergeMenuOpen(false);
    },
  });
  const doneMut = useMutation({
    mutationFn: (id: number) => api.doneTicket(id),
    onSuccess: (_data, doneId) => {
      qc.invalidateQueries({ queryKey: boardKey });
      // The pane may have switched to a different ticket while the request was
      // in flight; only close if we're still viewing the ticket that was finished.
      if (doneId === ticketId) onClose();
    },
  });

  // Per-ticket transient UI state. Switching tickets resets these so that
  // an in-flight action on the previous ticket doesn't leave the new
  // ticket's button stuck in its spinner state. Mutation handles are stable
  // for the lifetime of this component; listing them as deps would re-fire
  // on every render.
  // biome-ignore lint/correctness/useExhaustiveDependencies: see above.
  useEffect(() => {
    setSyncMenuOpen(false);
    setMergeMenuOpen(false);
    startMut.reset();
    ensureMut.reset();
    stopMut.reset();
    restartMut.reset();
    archiveMut.reset();
    syncMut.reset();
    mergeMut.reset();
    doneMut.reset();
  }, [ticketId]);

  // ticketId isn't referenced directly, but switching tickets remounts the
  // header DOM, so we re-measure to recompute the compact threshold.
  // biome-ignore lint/correctness/useExhaustiveDependencies: see above.
  useLayoutEffect(() => {
    const header = headerRef.current;
    const ghost = headerGhostRef.current;
    if (!header || !ghost) return;
    let raf: number | null = null;
    const measure = () => {
      raf = null;
      const h = headerRef.current;
      const g = headerGhostRef.current;
      if (!h || !g) return;
      setCompact(g.scrollWidth > h.clientWidth);
    };
    const schedule = () => {
      if (raf == null) raf = requestAnimationFrame(measure);
    };
    measure();
    const ro = new ResizeObserver(schedule);
    ro.observe(header);
    ro.observe(ghost);
    return () => {
      ro.disconnect();
      if (raf != null) cancelAnimationFrame(raf);
    };
  }, [ticketId]);

  // Register agent/shell slot DOM into the global terminalSlotStore so
  // TerminalsRoot can portal the matching PtyTerminal in. Re-runs when the
  // ticket id changes so a Board-view SessionView that swaps tickets
  // re-keys the slot. Cleans up on unmount or ticket change.
  const onAgentSlot = useCallback(
    (el: HTMLDivElement | null) => setTerminalSlot(ticketId, "agent", el),
    [ticketId],
  );
  const onShellSlot = useCallback(
    (el: HTMLDivElement | null) => setTerminalSlot(ticketId, "shell", el),
    [ticketId],
  );
  useEffect(() => {
    return () => {
      // Defensive cleanup if the ref callback didn't fire null (e.g. rapid
      // unmount during a portal swap).
      setTerminalSlot(ticketId, "agent", null);
      setTerminalSlot(ticketId, "shell", null);
    };
  }, [ticketId]);

  const mergeStrategies = enabledMergeStrategies(mergeConfig);
  const syncStrategies = enabledSyncStrategies(syncConfig);
  const status = session?.status;
  const isRunning = status && !["stopped", "error", "stopping"].includes(status);
  const canStart = session && !isRunning && status !== "starting";

  const renderHeaderContent = ({
    compact,
    interactive,
  }: {
    compact: boolean;
    interactive: boolean;
  }): ReactNode => (
    <>
      <span className="min-w-0 truncate font-medium">{ticket?.title}</span>
      <span className="whitespace-nowrap text-fg-muted">#{ticketId}</span>
      {session?.pr_number != null && session.pr_url && (
        <a
          href={session.pr_url}
          target="_blank"
          rel="noreferrer"
          className={`hover:underline ${
            session.pr_state
              ? (PR_STATE_COLOR[session.pr_state as PRState] ?? "text-fg-muted")
              : "text-fg-muted"
          }`}
          title={session.pr_state ? `PR ${session.pr_state}` : "pull request"}
        >
          #{session.pr_number}
        </a>
      )}
      <div className="ml-auto flex gap-2">
        {!session && (
          <ActionButton
            compact={compact}
            variant="primary"
            pending={ensureMut.isPending}
            onClick={() => ensureMut.mutate()}
            icon={<PlusIcon />}
            idleLabel="create session"
            pendingLabel="creating session…"
            ariaLabel="create session"
          />
        )}
        {canStart && (
          <ActionButton
            compact={compact}
            variant="primary"
            pending={startMut.isPending || status === "starting"}
            onClick={() => startMut.mutate(session.id)}
            icon={<PlayIcon />}
            idleLabel="start"
            pendingLabel="starting…"
            ariaLabel="start"
          />
        )}
        {session && isRunning && (
          <ActionButton
            compact={compact}
            variant="secondary"
            pending={stopMut.isPending || status === "stopping"}
            onClick={() => stopMut.mutate()}
            icon={<StopIcon />}
            idleLabel="stop"
            pendingLabel="stopping…"
            ariaLabel="stop"
          />
        )}
        {session && isRunning && (
          <ActionButton
            compact={compact}
            variant="neutral"
            pending={restartMut.isPending}
            onClick={() => restartMut.mutate()}
            icon={<RestartIcon />}
            idleLabel="restart container"
            pendingLabel="restarting…"
            ariaLabel="restart container"
            title="restart container"
          />
        )}
        {session && syncStrategies.length === 1 && (
          <ActionButton
            compact={compact}
            variant="neutral"
            pending={syncMut.isPending}
            onClick={() => syncMut.mutate(syncStrategies[0])}
            icon={<PullIcon />}
            idleLabel={`${SYNC_STRATEGY_LABELS[syncStrategies[0]]} ${baseBranch}`}
            pendingLabel="syncing…"
            ariaLabel={`${SYNC_STRATEGY_LABELS[syncStrategies[0]]} ${baseBranch}`}
            title={`update from ${baseBranch}`}
          />
        )}
        {session && syncStrategies.length > 1 && (
          <div className="relative" ref={interactive ? syncMenuRef : undefined}>
            <ActionButton
              compact={compact}
              variant="neutral"
              pending={syncMut.isPending}
              onClick={() => setSyncMenuOpen((v) => !v)}
              icon={<PullIcon />}
              idleLabel="sync ▾"
              pendingLabel="syncing…"
              ariaLabel="sync"
              title={`update from ${baseBranch}`}
              compactTitle={`update from ${baseBranch}`}
            />
            {interactive && syncMenuOpen && (
              <div className="absolute right-0 top-full z-(--z-raised) mt-1 w-56 rounded border border-border bg-surface p-1 text-xs shadow-lg">
                {syncStrategies.map((s) => (
                  <Button
                    key={s}
                    variant="ghost"
                    className="block w-full text-left"
                    onClick={() => syncMut.mutate(s)}
                  >
                    {SYNC_STRATEGY_LABELS[s]} <span className="font-mono">{baseBranch}</span>
                  </Button>
                ))}
              </div>
            )}
          </div>
        )}
        {session && mergeStrategies.length === 1 && (
          <ActionButton
            compact={compact}
            variant="neutral"
            pending={mergeMut.isPending}
            onClick={() => mergeMut.mutate(mergeStrategies[0])}
            icon={<MergeIcon />}
            idleLabel={MERGE_STRATEGY_LABELS[mergeStrategies[0]]}
            pendingLabel="merging…"
            ariaLabel={MERGE_STRATEGY_LABELS[mergeStrategies[0]]}
            title={`integrate into ${baseBranch}`}
          />
        )}
        {session && mergeStrategies.length > 1 && (
          <div className="relative" ref={interactive ? mergeMenuRef : undefined}>
            <ActionButton
              compact={compact}
              variant="neutral"
              pending={mergeMut.isPending}
              onClick={() => setMergeMenuOpen((v) => !v)}
              icon={<MergeIcon />}
              idleLabel="merge ▾"
              pendingLabel="merging…"
              ariaLabel="merge"
              title={`integrate into ${baseBranch}`}
              compactTitle={`integrate into ${baseBranch}`}
            />
            {interactive && mergeMenuOpen && (
              <div className="absolute right-0 top-full z-(--z-raised) mt-1 w-64 rounded border border-border bg-surface p-1 text-xs shadow-lg">
                {mergeStrategies.map((s) => (
                  <Button
                    key={s}
                    variant="ghost"
                    className="block w-full text-left"
                    onClick={() => mergeMut.mutate(s)}
                  >
                    {MERGE_STRATEGY_LABELS[s]}
                    {s === mergeConfig.default_strategy && (
                      <span className="ml-1 text-fg-muted">(default)</span>
                    )}
                  </Button>
                ))}
              </div>
            )}
          </div>
        )}
        <ActionButton
          compact={compact}
          variant="neutral"
          pending={archiveMut.isPending}
          onClick={() => archiveMut.mutate(ticketId)}
          icon={<ArchiveIcon />}
          idleLabel="archive"
          pendingLabel="archiving…"
          ariaLabel="archive"
        />
        <ActionButton
          compact={compact}
          variant="primary"
          pending={doneMut.isPending}
          onClick={() => doneMut.mutate(ticketId)}
          icon={<CheckIcon />}
          idleLabel="done"
          pendingLabel="finishing…"
          ariaLabel="done"
          title="mark as done"
          compactTitle="mark as done"
        />
        {headerExtras}
        <Button
          variant="neutral"
          size="icon"
          onClick={onClose}
          aria-label="Close session"
          title="Close session"
        >
          <XIcon />
        </Button>
      </div>
    </>
  );

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div
        ref={headerRef}
        className="flex items-center gap-2 border-b border-border px-3 py-2 text-sm"
      >
        {renderHeaderContent({ compact, interactive: true })}
      </div>
      <div
        ref={headerGhostRef}
        aria-hidden
        className="pointer-events-none invisible absolute left-0 top-0 flex items-center gap-2 whitespace-nowrap px-3 py-2 text-sm"
      >
        {renderHeaderContent({ compact: false, interactive: false })}
      </div>
      {status === "starting" && <PullProgressBanner progress={pullProgress ?? null} />}
      {!session && ensureMut.isError && (
        <div className="border-b border-border bg-danger/10 px-3 py-2 text-xs text-danger">
          <div className="font-medium">Couldn't create session</div>
          <div className="mt-1 whitespace-pre-wrap break-words font-mono">
            {formatApiError(ensureMut.error)}
          </div>
        </div>
      )}
      <div className="flex border-b border-border text-sm">
        <Tab active={tab === "agent"} onClick={() => setTab("agent")} label="agent" />
        <Tab active={tab === "shell"} onClick={() => setTab("shell")} label="terminal" />
        {hasWorktree && <Tab active={tab === "diff"} onClick={() => setTab("diff")} label="diff" />}
        <Tab active={tab === "tasks"} onClick={() => setTab("tasks")} label="tasks" />
        <Tab active={tab === "previews"} onClick={() => setTab("previews")} label="previews" />
        {hasPlans && <Tab active={tab === "plans"} onClick={() => setTab("plans")} label="plans" />}
        <Tab active={tab === "info"} onClick={() => setTab("info")} label="info" />
      </div>
      <div className="min-h-0 flex-1 bg-bg">
        {tab === "agent" && (
          <div className="h-full">
            {session && isRunning ? (
              <div ref={onAgentSlot} className="h-full w-full" />
            ) : (
              <p className="p-4 text-sm text-fg-muted">Start the session to attach the agent.</p>
            )}
          </div>
        )}
        {tab === "shell" && (
          <div className="h-full">
            {session && isRunning ? (
              <div ref={onShellSlot} className="h-full w-full" />
            ) : (
              <p className="p-4 text-sm text-fg-muted">Start the session to attach a shell.</p>
            )}
          </div>
        )}
        {tab === "diff" && session && (
          <Suspense fallback={<p className="p-4 text-sm text-fg-muted">Loading diff viewer…</p>}>
            <DiffPanel session={session} />
          </Suspense>
        )}
        {tab === "tasks" && session && <TasksPanel session={session} boardId={boardId} />}
        {tab === "previews" && session && <PreviewsPanel session={session} />}
        {tab === "plans" && session && <PlansPanel session={session} />}
        {tab === "info" &&
          (session ? (
            <InfoPanel session={session} />
          ) : (
            <p className="p-4 text-sm text-fg-muted">Create a session to see its info.</p>
          ))}
      </div>
    </div>
  );
}

// PullProgressBanner shows the live image pull state under the header while a
// session is starting. Falls back to an indeterminate "starting…" line when no
// progress events have arrived yet (image cached, or first event still in
// flight) so the bar isn't blank for the first ~200ms.
function PullProgressBanner({ progress }: { progress: PullProgress | null }) {
  const hasBytes = progress != null && progress.total > 0;
  const pct = hasBytes
    ? Math.min(100, Math.max(0, Math.round((progress.current / progress.total) * 100)))
    : 0;
  const status = progress?.status?.toLowerCase() ?? "starting";
  return (
    <div className="border-b border-border bg-surface px-3 py-1.5 text-xs">
      <div className="flex items-center justify-between gap-2 text-fg-muted">
        <span className="truncate">{hasBytes ? `pulling image: ${status}` : "starting…"}</span>
        <span className="shrink-0 font-mono">
          {hasBytes
            ? `${formatBytes(progress.current)} / ${formatBytes(progress.total)} (${pct}%)`
            : ""}
        </span>
      </div>
      <div className="mt-1 h-1 w-full overflow-hidden rounded bg-border">
        {hasBytes ? (
          <div
            className="h-full origin-left bg-accent-500 transition-transform duration-200 ease-(--ease-out)"
            style={{ transform: `scaleX(${pct / 100})` }}
          />
        ) : (
          <div className="h-full w-1/3 animate-pulse bg-accent-500/60" />
        )}
      </div>
    </div>
  );
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v >= 100 ? 0 : v >= 10 ? 1 : 2)} ${units[i]}`;
}
