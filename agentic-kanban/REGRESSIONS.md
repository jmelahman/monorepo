# Recurring regression notes

Tripwires for code that has bitten us before: what to check, not what
happened. Skim the headings before finishing a change and read any entry it
touches. Several fire from changes that don't look risky.

When you fix something likely to recur, add an entry (or extend one) **and**
a `See REGRESSIONS.md: "<title>"` comment at the code it guards. Code quotes
these titles, so don't rename one without updating its references. This file
is internal and kept out of the published `docs/` site.

### `ghostty-web` terminal dispose poisons the WASM heap

- `Terminal.dispose()` after a multi-codepoint grapheme (flag, ZWJ emoji)
  corrupts the shared WASM heap; the next terminal traps on `write()`.
- `PtyTerminal.tsx` clears the private `wasmTerm` before `dispose()` to skip
  the buggy free. Keep it until coder/ghostty-web#142 lands and we bump.

### `ghostty-web` canvas doesn't fill its container

- Don't use the bundled `FitAddon` (it reserves a 15px gutter). `fitToHost`
  sizes the grid and stretches the canvas to `100%` inline.
- Keep the fit in JS, not CSS `!important`. If a package bump resets the
  inline canvas size somewhere besides `term.resize()`, re-apply it there.

### Soft keyboard covers the terminal

- Full-screen surfaces that can hold a focused input size to `--app-height`
  (top-anchored, explicit height), never `inset-0`/`h-screen`/`100vh`.
- `mobile-keyboard.spec.ts` fakes `visualViewport`; check on a real phone.

### Soft keyboards don't type through keydown

- Android text arrives via composition/`beforeinput`, not `keydown`;
  `attachSoftKeyboardInput` (`web/src/terminalInput.ts`) handles it.
- On a ghostty-web bump, re-check that it still cancels the `keydown`s it
  handles (or hardware keys double) and that composition events stay hidden
  from it (or words send twice). Test on a real phone.

### Status hooks repeat, so push notifications key off the transition

- Hooks report the same status repeatedly, and `awaiting_perm` also fires
  for an idle reminder. Notify on the transition from the row read *before*
  the update, never on the reported status.
- `awaiting_perm → idle` is a finish too: approving a prompt doesn't always
  report `working`. Don't write tests that insert a `working` in between.
- Failure notifications come from failed starts and `Reconcile`, never from
  `Stop`, and `Reconcile` stays quiet while a user `Stop` is in flight.

### `sessions` row has multiple writers

- The session manager and the GitHub poller both write the row. Use
  column-scoped updates (`UpdateSessionLifecycle`, `UpdateSessionPR`), not
  `UpsertSession`, from any path that doesn't own every column.
- Refetch before publishing `session_updated` (`publishSessionUpdated`).

### `sessions` rows don't track container liveness

- Before acting on a session's container from the row alone, call
  `Manager.Reconcile`.
- Only "not found" or `State.Running == false` is evidence. An inspect error
  means unknown: leave the row alone.
- Keep reconciliation off hot polled reads (`/state`, summaries).

### Closing a PTY broker doesn't end its process

- Docker doesn't kill a TTY exec when the client disconnects. Replacing a
  PTY in a live container must end the process (`endPTYScript` via
  `$KANBAN_PTY_ID`; `Manager.StopAgentUnless` is the model).
- Remove the broker from the set under its lock *before* shutting it down.
- Reset a stopped agent's status with `ResetSessionActivity`, not
  `PATCH …/status` (which fires `session.idle` hooks and preview deploys).

### Per-origin SSE streams starve the WebSocket pool

- Never open a per-board `EventSource`. Use `subscribeBoard` /
  `useBoardSubscription` (one multiplexed `GET /api/events` stream).

### A deleted board can refetch its `/state` on a loop

- No board-level SSE exists, so other clients' deletions aren't seen. The
  `QueryClient` in `web/src/main.tsx` never retries 4xx and refetches the
  boards list on a `["board", …]` 404. Keep that.
- If you add `board_deleted`, evict proactively instead of relying on the 404.

### Build Cop boards on the same repo share one jobs cache

- The jobs cache is keyed by repo, not board. Evict once per tick against
  the union of every board's keep set (`retainJobsForBoards`); `syncBoard`
  must not evict on its own.
- Skip eviction for a repo whose board errored this tick.
- Test with two boards on one repo; single-board tests miss the storm.

### Exec sites hardcode `/workspace` instead of the session's workspace folder

- Exec `WorkingDir` comes from `sess.WorkspaceDir()`, never the literal
  `/workspace` or `board.ProjectDir` re-joined at call time.
- Record, don't recompute: the container's cwd is frozen at creation.
- `sessions.workspace_folder` is written by `UpdateSessionLifecycle`, not
  `UpsertSession` (which also drops `harness`).

### `git worktree prune` from the other side of the mount orphans worktrees

- Create worktrees through `internal/git` helpers (or pass `--lock`).
- Remove with `--force --force`; one `--force` refuses a locked worktree.
- Fix an orphan with `git worktree repair <path>`, never by pruning.

### Piping a daemonizing helper's stdio hangs `cmd.Run()`

- When shelling out to anything that may daemonize (`xclip`, `wl-copy`),
  send stdout/stderr to `nil` or an `*os.File`, never a `bytes.Buffer`.
- Anything synchronous on a tcell event loop gets a timeout.

### Task stop can't use the exec's PID

- Exec PIDs are in the host namespace. Stop by the `KANBAN_TASK_RUN` env tag
  (plus descendants), never by PID.
- Don't swallow the stop exec's error; UIs trust a 204.

### `task_runs` rows outlive the process watching them

- A `running` row the runner isn't watching (`live`) is an orphan: `Stop`
  closes it itself, `Reconcile` closes it on definitive evidence only.
- Session `Stop` closes remaining runs (`StopRunningTaskRuns`).
- Close runs only with `FinishTaskRun` (conditional on `running`); the first
  writer wins. Start's failure paths use `abandon`.
- `ExecState` treats `Pid == 0` as never ran, with no exit code.

### Tests inherit the caller's git environment and gitconfig

- Every package whose tests run git calls `gittest.IsolateEnv()` in
  `TestMain`. It also neuters `~/.config/git/{ignore,attributes}`.
- It clears all of `git rev-parse --local-env-vars`, not a fixed list. `git
  commit -a` points `GIT_INDEX_FILE` at the real `index.lock`, so a temp repo's
  `git add` would write to the commit in progress.
- Tests that commit set identity on the repo or with `-c`.

### Tests inherit the developer's kanban user config

- A test that reads merged config points `$KANBAN_CONFIG` (or
  `$XDG_CONFIG_HOME`) at a throwaway path first. `$KANBAN_CONFIG` wins, so
  a package that sets it can't steer the user file with `$XDG_CONFIG_HOME`.

### A push subscription is tied to one VAPID key

- `subscribe` (`web/src/push.ts`) replaces a subscription whose key doesn't
  match `/api/push/vapid-key`; `sendTestPush` renews on 404/410.
- Keep surfacing the push service's own status (`push.ErrGone` → 410)
  rather than a generic 502.

### Config writes must follow symlinks

- Project and user config files are often symlinks (undot, dotfiles). Any
  writer resolves the path with `filepath.EvalSymlinks` before renaming or
  deleting, as `kanbantoml.EditFile` does.

### Project config links must stay inside the repo

- Repo contents can plant `.kanban.toml -> /anywhere`. Project-scope paths
  must resolve under the repo: `kanbantoml.ProjectFile` for writers (422 on
  escape), `readProjectAt` for `Load`/`LoadLayers` (absent layer). The user
  config may link anywhere.
- Never write config through a predictable path: temp files use
  `os.CreateTemp`, not `<path>.tmp`, which a planted link can redirect.

### Proxying `/ws` must preserve `Host`

- Anything proxying to kanban forwards the browser's `Host` unchanged
  (`kanban web`: `newWebHandler`). Never loosen `CheckSameOrigin` instead.

### Error tickets are only symbolicated while the build embeds source maps

- Keep `build.sourcemap: true` and the maps beside their bundles in
  `web/dist`. After changing the Vite output layout or embed, file a
  frontend error and check the ticket shows `src/` paths.
- The dedup fingerprint hashes frame locations only, never function names.

### Schema changes go in the numbered migration list

- Append to `migrations` in `internal/db/migrations.go`. Never edit
  `schema.sql` (hash-pinned; don't update the hash) or `legacyMigrate`.
- The list is append-only: never reorder, edit or delete a shipped entry.
- Tests simulating a pre-versioning DB set `PRAGMA user_version = 0` and
  restore columns later migrations dropped (`sessions.mount_path`,
  `sessions.repo_path`).
- `PRAGMA foreign_keys` can't change inside a transaction; rebuilding a
  referenced table needs runner support.

### A release waits for the mirror

- `orchard.Release` refuses a rev whose subtree commits the upstream lacks, and
  the mirror action publishes them some time after a merge. The preview reports
  this as `blocked` so the Releases tab can say why before anyone confirms.
- Don't work around it by setting `ReleaseOptions.Upstream`: the mirror is the
  intended path, and a server pushing the branch itself would race it.
