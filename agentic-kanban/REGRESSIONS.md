# Recurring regression notes

Tripwires for code that has bitten us before. Each entry states the rule and
the trap, not the war story. They earn their keep when you hit them *before*
you knew you needed them — several fire from changes that don't look risky
(adding an SSE event, a routine handler that updates a couple of columns), so
skim the headings before finishing a change and read any entry it touches.

When you fix a regression that fits an entry below, extend it. When you fix
something new and likely to recur, add a fresh entry here **and** a
`See REGRESSIONS.md: "<title>"` comment at the code it guards. This file is
deliberately kept out of the published `docs/` site — it's internal
engineering lore, not user docs.

### `ghostty-web` terminal dispose poisons the WASM heap

`Terminal.dispose()` calls `ghostty_terminal_free`, which corrupts the
shared WASM linear memory whenever the terminal previously wrote a
multi-codepoint grapheme cluster (flag emoji, skin tone, ZWJ family,
keycap). Because `init()` keeps a single page-wide Ghostty instance, the
next terminal's first `write()` traps with "Out of bounds memory access"
— see upstream issue coder/ghostty-web#141. `PtyTerminal.tsx` works
around this by setting the private `wasmTerm` field to `undefined`
before calling `dispose()`, which skips the buggy `free()` while keeping
the rest of cleanup (DOM removal, document listeners, observers). Drop
the workaround when coder/ghostty-web#142 lands and we bump the package.

### `ghostty-web` canvas doesn't fill its container

The bundled `FitAddon` reserves a hard-coded 15px scrollbar gutter (we draw
the scrollbar onto the canvas, so it's dead space) and ghostty sizes the
`<canvas>` to a whole-cell grid, so the canvas lands up to a cell short on the
right/bottom — a visible margin. `PtyTerminal.tsx` skips the addon, floors the
grid to whole cells against the full host box itself (`fitToHost`), then sets
the canvas inline `width/height` to `100%` to stretch the sub-cell remainder
away. Keep the fit in JS, not a CSS `!important` rule. ghostty rewrites the
inline canvas size only from `term.resize()` (which `fitToHost` owns) and font
changes (we never do at runtime), so re-applying the stretch inside `fitToHost`
is enough; if a package bump starts resetting it elsewhere, re-apply there too.

### `sessions` row has multiple writers

Two independent paths write to the `sessions` row: the session manager
(lifecycle columns — `status`, `container_id`, `started_at`, `stopped_at`)
and the GitHub poller (`pr_state`, `pr_number`, `pr_url`, `pr_title`).
Anything that loads the row, mutates a few fields, then writes the whole
row back will silently clobber whatever the other writer just committed.
Rules:

- Use column-scoped updates (`UpdateSessionLifecycle`, `UpdateSessionPR`).
  Don't `UpsertSession` from a path that doesn't own every column.
- Before publishing `session_updated` over SSE, refetch from the DB so the
  wire payload reflects what's persisted. HTTP handlers funnel through
  `publishSessionUpdated(ctx, sessionID)` which refetches; the poller
  refetches in `applyTransition`'s defer.

### `sessions` rows don't track container liveness

Only kanban's own lifecycle calls write `status` and `container_id`;
nothing watches the daemon. A container that dies underneath us (host
reboot, docker restart, OOM kill, a manual `docker rm`) leaves a row
claiming `idle`/`working` with a container id that no longer exists, and
every consumer of the row acts on the lie — `kanban ticket attach` skipped
the start because the row looked running, then the PTY exec failed with
"container … is not running". `session.Manager.Reconcile` is the fix: it
inspects the recorded container and runs the normal `Stop` path (clears
the id, tears down brokers and proxies) when the container is gone. Rules:

- Any path that decides "this session is running, act on its container"
  from the row alone goes through `Reconcile` first. `Ensure` and `Start`
  do; the PTY/shell WebSocket handlers reconcile after a failed attach so
  the board card flips to stopped. Don't add a new consumer that trusts
  `status` and `container_id` as-is.
- Only a definitive answer counts as evidence: the daemon says not found,
  or `State.Running` is false. An inspect *error* means "unknown" — leave
  the row alone rather than stopping a session because docker hiccuped.
- Keep it off hot reads. `/state` and the summary endpoints are polled
  constantly; a docker round-trip per session per poll is not acceptable
  there. Reconcile at the points where someone is about to act on the
  container.

### Closing a PTY broker doesn't end its process

A `sessionPTY` broker owns a hijacked `docker exec` connection with a TTY.
Closing that connection (`shutdown`) only drops kanban's end: Docker doesn't
kill a TTY exec when its client goes away, and with a TTY it doesn't even
close the process's stdin, so the process keeps running unseen until its
output buffers fill. That's fine when the whole container is about to be
stopped (`Stop` → `closeFor`), but not when one PTY is replaced inside a
running container — a harness switch that only closed the broker left the
old `claude` running next to the new `pi`, still firing its status hooks.
Rules:

- Replacing a PTY in a live container means ending its process too.
  Every brokered exec carries a unique `$KANBAN_PTY_ID`; `endPTYScript`
  finds the exec's own process (tagged, `PPid: 0`) and sends `SIGHUP`,
  then `SIGKILL`. Reuse that (`Manager.StopAgentUnless` is the model)
  rather than just calling `shutdown`.
- Take the broker out of the set under its lock *before* shutting it down,
  so a concurrent attach starts a fresh exec instead of rejoining the one
  being torn down.
- A stopped agent never reports the `idle` that ends its `working` or
  `awaiting_perm` status. Reset it column-scoped
  (`ResetSessionActivity`), not through `PATCH …/status`, which would fire
  `session.idle` hooks and an auto preview deploy.

### Per-origin SSE streams starve the WebSocket pool

Browsers cap HTTP/1.1 connections at 6 per origin and route the initial
WebSocket upgrade request through the same pool. One `EventSource` per
board (Overview subscribes to *every* loaded board) saturates the pool
once you have ~6 boards open; the next PTY WebSocket handshake then sits
queued in `readyState: CONNECTING` indefinitely — the terminal panel
shows a cursor but never receives output until the user refreshes. All
SSE subscribers funnel through a singleton `BoardEventManager` in
`web/src/api/client.ts` that keeps one stream open against
`GET /api/events?boards=…`. Don't add new code paths that open per-board
EventSources; route them through `subscribeBoard` / `useBoardSubscription`.

### A deleted board can refetch its `/state` on a loop

There is no board-level SSE (no `board_created`/`board_deleted`), so the
boards list is only refreshed by *this* client's own mutations. A long-lived
tab that loaded while board N existed keeps board N mounted in Overview after
someone else deletes it — still observing `["board", N]` and still subscribed
via the multiplexed stream. Once we moved to one shared connection
(`26713ba`), every subscribed board's events arrive reliably (the old
per-board EventSources were starved by the 6-connection cap and silently
dropped these), so each event for the dead board re-invalidates and refetches
its 404ing `GET /api/boards/N/state` — amplified ×4 by react-query retries,
each a toast + `reportRuntimeError`. The `QueryClient` in `web/src/main.tsx`
breaks the loop: 4xx responses are never retried, and a 404 on a `["board",
…]` query refetches the boards list (dropping the dead board so its Overview
node unmounts) instead of toasting. If you ever add a board-level SSE event,
prefer evicting proactively over relying on this 404 fallback.

### Build Cop boards on the same repo share one jobs cache

The build-cop poller memoizes `/actions/runs/:id/jobs` per `(repo_path,
run_id)` — completed runs are immutable, so a warm cache makes the steady
state ~zero job fetches per tick (only genuinely new runs). The trap: the
cache is keyed by **repo, not by board**, and more than one Build Cop board
can poll the same repo (e.g. an all-branches board plus a `branch = "main"`
board). If each board evicts the cache against *its own* window, the boards
clobber each other every tick: the all-branches board's non-main runs are
dropped by the main board's retain, then refetched next tick, then dropped
again. That fanout — ~150 jobs calls/tick at the incident — burned the entire
5000/hr core budget and 403'd the unrelated PR poller as collateral (shared
token). It surfaced the day a second Build Cop board was added to a repo that
already had one, and nothing in the single-board tests caught it.

Rules:

- Eviction is **per tick**, not per board: `tick` collects each board's keep
  set, unions them by repo, and calls `retainJobs` once per repo
  (`retainJobsForBoards`). `syncBoard` must *not* evict on its own.
- Skip eviction for a repo whose board errored this tick — a transient
  `listRuns` failure shouldn't drop entries the failed board would have kept.
- If you add conditional-request (ETag) support or otherwise touch the cache,
  keep the union-retain invariant: any code path that retains against a single
  board's window reintroduces the storm.

### Exec sites hardcode `/workspace` instead of the session's workspace folder

Every `docker exec` kanban issues needs a working directory, and for a long
time four of them wrote the string `/workspace` rather than asking where the
container's workspace actually is: `AttachAgent` and `AttachShell`
(`internal/api/handlers.go`), the task runner's `${workspaceFolder}`
substitution *and* exec `WorkingDir` (`internal/tasks/runner.go`), and the
`CommitMsgTemplate` in `internal/harness/harness.go`, which literally began
`cd /workspace && git diff --staged`. That was already wrong for any repo
whose `devcontainer.json` declares a custom `workspaceFolder`, and it became
wrong for every monorepo board once `boards.project_dir` moved the agent's
cwd into a subdirectory.

The fix is `sessions.workspace_folder`, written from `SpawnResult` in
`Manager.Start` and read back through `db.Session.WorkspaceDir()`, which
falls back to `/workspace` so pre-existing rows behave exactly as before.
It lives on `db.Session` specifically so `internal/api` and `internal/tasks`
can use it without importing `internal/session`.

Rules:

- A new exec site takes its `WorkingDir` from `sess.WorkspaceDir()`. Never
  the literal, never `board.ProjectDir` re-joined at call time.
- Record, don't recompute. Mounts and `WorkingDir` are frozen when the
  container is created, so a cwd recomputed from the board after someone
  edits `project_dir` can point at a directory the running container does
  not have. The column is the container's ground truth until it is
  recreated.
- Two drifts follow from that and are deliberate: host-side discovery
  (tasks, plans, `.kanban.toml`, `devcontainer.json`) reads `board.ProjectDir`
  and so follows an edit immediately, and a `.claude/settings.local.json`
  already written at the old location stays there, inert. Restarting the
  session reconciles both.
- `UpsertSession` does not persist this column (it already silently forgets
  `harness`); `UpdateSessionLifecycle` does. Adding a write through the
  wrong one looks like it works and then loses the value on the next upsert.

### `git worktree prune` from the other side of the mount orphans worktrees

A worktree's admin dir (`<repo>/.git/worktrees/<name>/gitdir`) records the
worktree path as kanban saw it when running `git worktree add`. Session
containers mount the same checkout at `/workspace`, so from inside a container
(or from a host whose paths differ from kanban's) that recorded path doesn't
exist. `git worktree prune` treats that as a deleted worktree and removes the
admin dir, which leaves the checkout's `.git` pointing at nothing. The prune
can come from the agent, from a user, or from git itself: `git gc` runs
`worktree prune` once `gc.worktreePruneExpire` has passed.

The fix is to lock every worktree kanban creates (`--lock --reason` on
`AddWorktree`/`AddWorktreeFromExisting` in `internal/git/worktree.go`). Prune
skips locked worktrees. `Manager.Ensure` also calls `git.LockWorktree` when it
reuses a worktree, which locks any worktree created before this change.

Rules:

- New code that creates a worktree goes through the `internal/git` helpers,
  or passes `--lock` itself. An unlocked worktree only breaks once something
  prunes, which can be weeks later.
- Removal needs `--force --force`. A single `--force` refuses a locked
  worktree, and the `os.RemoveAll` fallback in `Destroy` then removes the
  checkout but not its admin dir. Because the admin dir stays locked, prune
  never cleans it up, and `git branch -D` fails because the branch is still
  checked out in a registered worktree.
- Don't "fix" an orphaned worktree by pruning. Run `git worktree repair
  <path>` from the side whose path matches the recorded one.

### Piping a daemonizing helper's stdio hangs `cmd.Run()`

`exec.Cmd.Wait` waits for the process to exit **and** for every stdio pipe
Go created (any `Stdout`/`Stderr` that isn't an `*os.File`) to reach EOF.
Helpers that fork a long-lived child, such as `xclip`, `xsel` and `wl-copy`, which
stay alive to serve the selection, hand that child their stderr. The pipe
stays open until the child exits. For a clipboard helper, the child exits
only when the user copies something else. `nativeCopy` captured stderr
into a `bytes.Buffer` and ran on the TUI event loop, so pressing `c` in
`kanban ticket tasks` froze the terminal.

Rules:

- When shelling out to anything that may daemonize, send stdout/stderr to
  `nil` or an `*os.File` (see `nativeCopy`'s temp file), never a
  `bytes.Buffer`.
- Anything that runs synchronously on a tcell event loop gets a timeout
  (`exec.CommandContext`). A blocked loop can't redraw, so the user sees a
  frozen terminal with no error.

### Task stop can't use the exec's PID

`ContainerExecInspect(...).Pid` is in the **host** PID namespace; inside the
container that number is absent or belongs to something else, so signaling it
(or walking `/proc` from it) silently stops nothing. `tasks.Runner.Start` tags
each run's exec with `KANBAN_TASK_RUN=<id>` and `Stop` SIGTERMs every container
process whose `/proc/<pid>/environ` carries it, which also reaches grandchildren
reparented away from the task shell, plus every descendant of a tagged process
(catching `sudo`/`env -i` children that drop the marker). Don't reintroduce a PID-based stop, and
don't swallow the stop exec's error — both UIs trust a 204 to mean it worked.
