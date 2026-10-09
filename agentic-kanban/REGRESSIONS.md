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

### Soft keyboard covers the terminal

`height: 100%` and `fixed inset-0` follow the *layout* viewport. On iOS
Safari (and Android browsers that ignore `interactive-widget=resizes-content`)
the soft keyboard only shrinks the *visual* viewport, so anything sized that
way keeps its full height and the keyboard hides its bottom — for the agent
terminal that is the prompt and the permission dialogs. `useVisualViewportHeight`
publishes the visual viewport height as `--app-height`; `#root` and the
`SessionPane` overlay size to it, and `PtyTerminal`'s ResizeObserver refits
the grid so the TUI redraws above the keyboard. New full-screen surfaces that
can hold a focused input must use `--app-height` (top-anchored, explicit
height), never `inset-0`/`h-screen`/`100vh`. Headless Chromium has no soft
keyboard: `mobile-keyboard.spec.ts` fakes `visualViewport`, so changes here
still need a check on a real phone.

### Soft keyboards don't type through keydown

ghostty-web types from `keydown` and looks keys up by `KeyboardEvent.code`.
Android keyboards report every key as keyCode 229 with an empty `code` and
deliver text through composition and `beforeinput` events; ghostty cancels
`beforeinput` and only forwards a composition once it ends. On a phone that
meant a word appeared after the next space at best, and the space, Enter and
Backspace never reached the PTY. `attachSoftKeyboardInput`
(`web/src/terminalInput.ts`) handles those events in the capture phase on the
terminal host and streams the word being composed as "erase what changed,
type the rest". It relies on ghostty cancelling the `keydown`s it does handle
(so no `beforeinput` follows and hardware keys aren't doubled) and on hiding
the composition events from ghostty (or the word is sent twice) — re-check
both when bumping ghostty-web. Headless Chromium has no soft keyboard and
synthetic events only approximate one, so changes here need a real phone.

### Status hooks repeat, so push notifications key off the transition

The agent hooks `PATCH /api/sessions/{id}/status` more often than the
status changes: `working` on every prompt and after every tool call, `idle`
on every stop, and Claude's `Notification` hook reports `awaiting_perm` both
for a real permission prompt *and* when a finished agent has sat idle for a
minute. Sending a Web Push per report buzzes the phone constantly, and the
idle reminder follows every "finished" with a bogus "waiting for permission".
`updateSessionStatus` therefore compares against the row it read *before* the
update: `working → awaiting_perm` is a prompt, and `working → idle` or
`awaiting_perm → idle` is a finish.

The second finish case matters. Nothing reports `working` when the user
approves a prompt; only the `PostToolUse` hook does, once the tool has run,
and worktrees whose `settings.local.json` predates that hook (or is
hand-written) never send it. Gating "finished" on `working` alone silently
drops it for every turn that needed a permission. A test that inserts its
own `working` report between the prompt and the stop hides this.

Likewise the manager's failure notifier fires from failed starts and from
`Reconcile` finding a dead container, never from `Stop` itself, and
`Reconcile` stays quiet while a user's `Stop` for that session is in flight
(the container is removed before the row is updated). New triggers must
compare against the previous status rather than react to the reported one.

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

### `task_runs` rows outlive the process watching them

A run leaves `running` only when something records it. Normally that's the
output goroutine `tasks.Runner.Start` spawns, which marks the run exited
at EOF. That goroutine lives in the server process, and Docker can't
re-attach to an exec. So after a server restart (a restarted devcontainer,
for example), every run it was watching stays `running` forever. `Stop`
used to just run the stop script and wait for the goroutine to notice. For
an orphan it killed a process nobody was watching, or found nothing after
a container restart, and returned 204 with the row unchanged. Rules:

- The runner tracks the runs it watches (`live`). Every other `running`
  row is an orphan: `Stop` marks it `stopped` itself, and `Reconcile` (run
  on `GET /api/sessions/{id}/task-runs`) closes it once there's definitive
  evidence. That means the session has no container, the run has no exec,
  or `ExecState` reports the exec gone or finished. An inspect error is not
  evidence. Only orphans cost a docker call, which keeps the 2s-polled list
  cheap.
- Session `Stop` removes the container, so it marks the session's
  remaining runs `stopped` (`StopRunningTaskRuns`).
- If the stop script fails because the container is definitively gone,
  `Stop` still closes the orphan instead of returning 500.
- Close runs only with `FinishTaskRun` (conditional on `status='running'`).
  The goroutine, `Stop`, reconciliation and session stop race each other,
  and the first writer's exit code has to stand. Start's failure paths use
  `abandon`, which outlives the cancelled request context.
- A created-but-never-started exec inspects as not running with exit code
  0. `ExecState` treats `Pid == 0` as "never ran", with no exit code.

### Tests inherit the caller's git environment and gitconfig

Git hands its hooks `GIT_INDEX_FILE` and similar variables as repo-relative
paths. `go test` run from a pre-commit hook then points every git command at
the wrong index. The developer's `~/.gitconfig` leaks in too. A global
`core.hooksPath` runs a prek shim on every test `git commit`, and the shim
fails outside a configured repo. A global `commit.gpgsign` breaks commits the
same way. Both fail dozens of unrelated tests at once, on one machine only.

Hiding the gitconfig does not hide `~/.config/git/ignore` or
`~/.config/git/attributes`: git reads those by default whenever
`core.excludesFile` and `core.attributesFile` are unset. A global ignore of
`**/.claude/settings.local.json` made `git add -A` skip the session's agent
settings, so every merge test's "session files" commit had nothing to commit.
`IsolateEnv` pins both keys to the null device through `GIT_CONFIG_COUNT`.

Rule: every package whose tests run git (directly, or through the server
under test) calls `gittest.IsolateEnv()` first in its `TestMain`. Tests that
commit set their own identity on the repo or with `-c`, never through the
global config.

### Tests inherit the developer's kanban user config

`kanbantoml.Load` layers the user file (`~/.config/kanban/config.toml`) over
the project's `.kanban.toml`, and the user file wins. A test that writes a
`.kanban.toml` and reads the merged result passes in CI, where there is no
user file, and fails on a machine whose owner set the same section, such as a
`[merge]` that leaves one strategy enabled.

Rule: a test that reads merged config points `$KANBAN_CONFIG` (or
`$XDG_CONFIG_HOME`) at a throwaway path first. `cmd/server` does it for the
whole package in `TestMain`. `$KANBAN_CONFIG` takes precedence, so a package
that sets it there can't also steer the user file through `$XDG_CONFIG_HOME`.

### A push subscription is tied to one VAPID key

A browser's push subscription only accepts messages signed with the VAPID
key it was created for, and the browser keeps it across reloads. Put a
different backend behind the same address (dev server vs. compose, a
recreated or `--in-memory` database) and the browser still holds the old
subscription; re-registering it as-is makes every send fail (FCM answers
403, other services 404/410, which also prunes the row — the "502 then 404"
on the test button). `subscribe` in `web/src/push.ts` compares
`sub.options.applicationServerKey` with `/api/push/vapid-key` on every load
and replaces a mismatched subscription; `sendTestPush` renews on 404/410.
The server reports the push service's own answer (`push.ErrGone` → 410)
rather than a generic 502 — keep that detail, it is the only way to tell
these cases apart from a phone.

### Proxying `/ws` must preserve `Host`

The PTY and shell WebSockets only upgrade when `Origin` matches `Host`
(`session.CheckSameOrigin`), which is what keeps another site's page from
opening a shell in a session container. A reverse proxy that rewrites `Host`
to the upstream's address (the default for `httputil.ReverseProxy.SetURL` and
for nginx `proxy_pass`) makes every terminal fail with a 403 while the rest
of the UI, which is plain same-origin HTTP, keeps working.

Rule: anything proxying to kanban forwards the browser's `Host` unchanged.
`kanban web` does this in `newWebHandler`. Never fix a rejected upgrade by
loosening `CheckSameOrigin`.

### Error tickets are only symbolicated while the build embeds source maps

`internal/errreport` rewrites browser stack frames to `src/…:line:col` by
reading `assets/<bundle>.js.map` out of the embedded `web/dist`
(`web.DistFS()`). Nothing fails when a map is missing — the frame just stays
minified — so symbolication disappears silently if `build.sourcemap` in
`web/vite.config.ts` becomes `false`, a build step strips `*.map` from
`web/dist`, or the bundles move out from under the URL path the browser
reports (serving the app under a path prefix).

Rule: keep `sourcemap: true` and the maps beside their bundles in `web/dist`.
After touching the Vite output layout or the embed, file a frontend error
against an embedded build and check the ticket shows `src/` paths. The dedup
fingerprint hashes only each browser frame's location, never its minified
function name, which changes per build.
