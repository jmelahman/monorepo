# REST API

The HTTP server (default `:7474`) exposes a JSON REST API. The full route table lives in [`internal/api/api.go`](https://github.com/jmelahman/agentic-kanban/blob/master/internal/api/api.go); this page documents the endpoints most useful for scripting kanban from outside the UI.

::: warning No authentication
The server has no authentication. Bind only to `127.0.0.1` (the default container mapping does this).
:::

## Conventions

- All bodies are JSON unless noted.
- Path parameter `{id}` accepts either the numeric ID or the slug.
- Errors return `{ "error": "<message>" }` with a `4xx`/`5xx` status.
- Responses ≥1 KB are gzip-compressed when the client sends `Accept-Encoding: gzip`. Send `Accept-Encoding: identity` to opt out. SSE streams are never compressed.

## Tickets

### `POST /api/boards/{id}/tickets`

Create a ticket on the given board.

| Field       | Type    | Required | Notes                                                                                           |
|-------------|---------|----------|-------------------------------------------------------------------------------------------------|
| `title`     | string  | yes      |                                                                                                 |
| `body`      | string  | no       | Markdown ticket description.                                                                    |
| `column_id` | integer | no       | Numeric column id. Wins over `column` if both set.                                              |
| `column`    | string  | no       | Column name (case-insensitive) or numeric string. Defaults to the leftmost column when omitted. |

Returns `201` with the created `Ticket` JSON, or `400` / `404` on validation failures.

```sh
curl -fsS -X POST http://localhost:7474/api/boards/playground/tickets \
  -H 'Content-Type: application/json' \
  -d '{"title":"Investigate flaky test","column":"Backlog","body":"Repro on CI but not locally."}'
```

### `GET /api/tickets/{id}`

Get one ticket by numeric id. Unlike the board-scoped listings this needs no board context, so a caller holding only a ticket id (the CLI's `kanban ticket info`) can find the board it lives on — the response carries `board_id` and `column_id`. Returns `404` for an unknown id.

```sh
curl -fsS http://localhost:7474/api/tickets/42
```

### `GET /api/boards/{id}/archived`

List tickets that have been archived from the given board.

### `PATCH /api/tickets/{id}`

Update a ticket's `title` or `body`. Send only the fields you want to change.

### `PATCH /api/tickets/{id}/move`

Move a ticket to a different column or position. Body: `{"column_id": <id>, "position": <int>}`.

### `POST /api/tickets/{id}/archive` / `unarchive`

Move a ticket out of (or back into) the active board.

### `DELETE /api/tickets/{id}`

Permanently remove a ticket. Cleans up its session and worktree.

## Boards

### `GET /api/boards`

List all boards.

### `POST /api/boards`

Create a new board. Body fields:

| Field | Type | Notes |
| --- | --- | --- |
| `name` | string | Required. Display name; the slug is derived from this. |
| `repo_path` | string | Host path to the git repo. One of `repo_path` / `mount_path` is required. |
| `mount_path` | string | Host directory mounted into session containers, when distinct from the repo. |
| `worktree_root` | string | Parent directory for new session worktrees. Defaults to `<data_dir>/worktrees/<slug>`. |
| `base_branch` | string | Branch session worktrees fork from. When omitted, the server detects it from `repo_path` (`origin/HEAD`, falling back to the currently checked-out branch); if detection fails it defaults to `main`. Each new session worktree best-effort fetches `origin/<base_branch>` (10s timeout) and uses it as the start-point if it ends up strictly ahead of local; on failure, tie, or divergence, the local branch is used. |
| `branch_prefix` | string | Prefix prepended to session branch names. |
| `git_author_name` | string | Optional. Author name used for merge/squash commits the server creates. |
| `git_author_email` | string | Optional. Author email used for merge/squash commits. |

If both `git_author_name` and `git_author_email` are set, the server passes `-c user.name=… -c user.email=…` to `git commit` when finishing a session — needed when kanban runs in a container without a configured git identity. Leave both blank to fall back to whatever the kanban container's `git config` resolves.

### `PATCH /api/boards/{id}`

Patch any of the fields above. Pointer-style: omit a field to leave it untouched, send an empty string to clear it.

### `PATCH /api/boards/{id}/move`

Reorder boards. Body: `{"position": <int>}`, where `position` is the zero-based index the board should occupy in `GET /api/boards` after the move. The server renumbers every board so positions stay contiguous. `GET /api/boards` orders by this field, and the Overview sidebar exposes it as drag-to-reorder.

### `GET /api/boards/{id}`

Fetch a single board.

### `GET /api/boards/{id}/state`

Get the full hydrated board state — columns, tickets, sessions — in one call. The web UI uses this on load.

## Board environment variables

Per-board environment variables injected into the board's session containers at
launch — intended for secrets such as MCP server API keys. Values are
**write-only**: every response carries key names only, never values. They are
encrypted at rest (see the
[configuration guide](../guide/configuration#per-board-environment-variables)
for storage and security details) and take effect the next time a session is
started or restarted.

### `GET /api/boards/{id}/env`

Returns `{"keys": ["MY_API_KEY", …]}` — the sorted key names, always an array
(never `null`).

### `PATCH /api/boards/{id}/env`

Body: `{"set": {"KEY": "value", …}, "unset": ["OTHER_KEY", …]}`. Either field
may be omitted, but not both. Unsetting a missing key is a no-op. Returns the
updated `{"keys": […]}`.

Errors with `400` when a key doesn't match `[A-Za-z_][A-Za-z0-9_]*` or uses the
reserved `KANBAN_` prefix (case-insensitive), which is kept for
server-injected variables (`KANBAN_SESSION_ID`, `KANBAN_API_URL`).

## Live updates (SSE)

### `GET /api/boards/{id}/events`

Server-Sent Events stream of board changes. Events include:

- `ticket_created`
- `ticket_updated`
- `ticket_moved`
- `ticket_archived`
- `session_status_changed`
- `session_pull_progress` — emitted while a session is starting and its
  devcontainer image is being pulled. The payload is
  `{ session_id, image, current, total, layers, status, done }` where
  `current`/`total` are aggregate bytes summed across layers (`total` is `0`
  until the daemon reports the first byte counter), `status` is the most
  recent human-readable line from Docker (e.g. `Downloading`,
  `Extracting`), and `done` is `true` on the final terminal event.
- `task_run_*`

Useful for keeping a dashboard or another integration in sync without polling.

### `GET /api/events?boards=<id>[,<id>…]`

Multiplexed SSE stream that fans in events from N boards over a single
HTTP/1.1 connection. Each event is the same JSON envelope as the per-board
endpoint plus a `board_id` field so the client can demultiplex:

```
event: ticket_updated
data: {"type":"ticket_updated","data":{...},"board_id":42}
```

Prefer this endpoint when subscribing to more than one board — browsers cap
HTTP/1.1 connections at 6 per origin (and route WebSocket upgrades through
the same pool), so one EventSource per board can stall the next WebSocket
handshake. The web UI uses this for every board subscription.

## Sessions

Sessions are the running container + worktree + harness for a ticket. The most useful endpoints:

- `GET /api/sessions/summary` — instance-wide count of running containers across every board, for the header indicator. Returns `{ "running": <int>, "working": <int>, "awaiting_perm": <int>, "idle": <int>, "starting": <int> }`, where `running` is the sum of the four active states. Stopped and errored sessions are excluded. Computed as a single `GROUP BY status` aggregate, so it's cheap to poll.
- `POST /api/tickets/{id}/session` — ensure a session exists for the ticket. An existing row is checked against the docker daemon first: if its recorded container has died since it was started (host reboot, docker restart, OOM kill), the session is marked `stopped` with `container_id` cleared before it is returned, so a follow-up `start` brings up a fresh container instead of the client trying to attach to nothing.
- `POST /api/sessions/{id}/start` / `stop` / `restart` — `start` is a no-op for a session that is already running, except that a session whose recorded container is no longer running is treated as stopped and started again rather than returned as-is. The PTY and shell WebSockets apply the same check when attaching to the container fails, so the board's `session_updated` event reflects the real state.
- `PATCH /api/sessions/{id}/status` — used by the harness to report state changes.
- `PATCH /api/sessions/{id}/claude-session` — called by the Claude Code `SessionStart` hook with `{ "claude_session_id": "<uuid>" }` so kanban can `--resume` the same conversation on the next launch after a container/Kanban restart. Rejects non-UUID values with `400`. Clearing the stored UUID (to force a fresh conversation) is a manual `UPDATE sessions SET claude_session_id = NULL` on the DB.
- `PATCH /api/sessions/{id}/branch` — re-point the session's tracked git branch with `{ "branch_name": "<branch>" }`, returning the updated session. Use this when a session pivots onto a different branch and you want the ticket to keep surfacing that branch's GitHub events: it's a metadata-only update of `branch_name` (no git rename, no GitHub call), and it clears the cached `pr_*` fields so the stale old-branch PR drops out of the UI and the GitHub poller re-associates the ticket with the new branch's PR on its next tick. Rejects an empty or malformed branch name (whitespace, a leading `-`, or `..`) with `400`, and an unknown session with `404`. Saving the branch it already has is a no-op (the PR fields are left intact).
- `PUT /api/sessions/{id}/harness` — choose the agent harness for this session with `{ "harness": "<id>" }` (an id from `GET /api/harnesses`), returning the updated session. `""` clears the choice so the session goes back to the resolved default (user config, then the project `.kanban.toml`, then `claude`). The choice is stored in the session's `harness` field, which is left out when unset. If the session's agent is running a different harness from the one the session now resolves to (compared with the harness that agent was launched with, so a default that changed since it started counts too), that agent is stopped: attached terminals see the session end, its process in the container gets `SIGHUP` and then `SIGKILL` if it is still there about five seconds later, and a `working` or `awaiting_perm` status it reported goes back to `idle` (without firing `session.idle` hooks). The response waits for this. The next agent attach launches the new harness in the same container; the shell PTY and the container are left alone. A board `session_updated` event is published when the stored harness changes or an agent is stopped. Returns `400` for an unknown harness id and `404` for an unknown session.
- `GET /api/harnesses[?board=<id>]` — the harness registry as `[{ "id", "label" }]`. With `board`, the entry that board's sessions launch by default carries `"default": true`. Returns `400` for a non-numeric board and `404` for an unknown board.
- `GET /api/sessions/{id}/ports` — list active port proxies.
- `GET /api/sessions/{id}/pr-detail` — live snapshot of the linked GitHub PR: diff totals, rolled-up review decision, and check status (passed / failing / pending counts plus the names of failing checks). Hits the GitHub API on each call; returns `404` if the session has no PR and `502` if GitHub is unreachable. A `502` is treated as a transient upstream outage (or the operator being offline), so it is **not** filed as a ticket on the errors board. The session payload itself carries `pr_state`, `pr_number`, `pr_url`, and `pr_title` (refreshed by the GitHub poller).
- `GET /api/sessions/{id}/diff` — unified diff of the session worktree, returned as `{ "base": "<branch>", "patch": "<unified diff>" }`. The patch spans from where the session branch diverged from the board's base branch (the merge-base) to the **current working tree**, so it includes committed changes, uncommitted edits to tracked files, and brand-new untracked files the agent created. Git's ignore rules are honored — the repo's `.gitignore`, `.git/info/exclude`, and the global excludes file (`~/.config/git/ignore`) — so anything ignored stays out of the diff. Computed without touching the worktree's real git index. A session with no worktree yet returns `{ "base": "", "patch": "" }` rather than an error, and an empty `patch` means "no changes". Backs the **diff** tab in the session pane, which renders it with [`@pierre/diffs`](https://github.com/pierrecomputer/pierre/tree/main/packages/diffs) and a changed-files tree.
- `GET /api/sessions/{id}/file?path=<path>` — current working-tree contents of a single file in the session worktree (the **new** side of the diff above), returned as `{ "path": "<path>", "contents": "<text>" }`. Backs the per-file **View file** toggle in the diff tab, which swaps a file's hunks for a whole-file, syntax-highlighted blob view (like GitHub's "View file"). `path` is required and must name a file inside the worktree — it is validated to stay within the worktree (both lexically and after resolving symlinks), so `../` or a symlink pointing outside returns `404`. Returns `400` when `path` is missing, and `404` when the session has no worktree or the file doesn't exist.
- `GET /api/sessions/{id}/file-diff?path=<path>&old_path=<path>` — both sides of one changed file, returned as `{ "path": "<path>", "old_contents": "<text>", "new_contents": "<text>" }`. The **old** side is read from the same merge-base the diff above is computed against (so its lines align with the patch's hunks); the **new** side is the working tree. The diff tab uses this to rebuild a file's diff from full contents and offer GitHub-style **expand up/down** beyond the patch's few context lines. `old_path` is the pre-rename path when it differs from `path` (the diff's `prevName`) and defaults to `path`. A side absent at its ref comes back empty — an empty `old_contents` is a newly-added file, an empty `new_contents` a deleted one. `path` is validated like `/file` above; returns `400` when `path` is missing, and `404` when the session has no worktree or the path exists in neither tree.

## Previews

Preview deployments (see [Previews](../guide/previews)) are served by an
embedded [local-preview](https://github.com/jmelahman/local-preview)
orchestrator at `<sha>.<board-slug>.<preview-domain>`. When the orchestrator
failed to start these endpoints return `503`.

- `GET /api/sessions/{id}/previews` — the session branch's preview deploys, newest first. Each deploy carries `status` (`queued`/`building`/`ready`/`failed`/`evicted`), `short_sha`, and — once ready — `preview_url`, `process` (the on-demand backend's live state), `fe_process` (the same for a process-mode frontend; absent for static bundles), and `artifacts` (see below). A board that has never deployed returns `[]`. Returns `400` when the board has no git repo or the session has no branch.
- `POST /api/sessions/{id}/previews` — registers the board repo with the orchestrator (idempotent) and requests a deploy of the session branch's current tip. Idempotent per commit: re-posting an already-built tip returns the existing deploy. Responds `202` with the deploy. The target repo must be onboarded — a manifest in the deployed commit, or a server-side one named for the board slug (the build fails with a clear error otherwise).
- `GET /api/previews` — every deploy across every board, newest first: the previews dashboard's feed. Each row is a deploy as above, plus `board_id`, `board_name`, and `board_slug` identifying the board that owns it. Those three are omitted when a deploy outlives its board (renamed or deleted) — the orchestrator only ever knows the repo name.
- `POST /api/boards/{id}/previews` — deploys an arbitrary ref of a board's repo: `{ "ref": "<branch|tag|sha>" }`, defaulting to the board's `base_branch` when omitted or empty. Registers the repo with the orchestrator first (idempotent). Responds `202` with the deploy. Returns `404` for an unknown board, and `400` when the board has no git repo, has no base branch to fall back on, or the ref doesn't resolve. The session endpoint above only ever deploys that session's branch; this one is what the dashboard's **deploy** dialog posts to.
- `POST /api/previews/{id}/stop` — stops the supervised processes backing a deploy without removing it. The deploy stays listed and cold-starts again on the next request to its URL. Processes are shared per artifact hash, so sibling deploys built to the same output stop with it. Responds `204`; `404` for an unknown deploy.
- `DELETE /api/previews/{id}` — removes a deploy and reclaims the artifacts and process state no surviving deploy still references. Previews are content-addressed per side, so a half shared with another deploy is kept. Responds `204`; `404` for an unknown deploy. Deploying the same commit again rebuilds it.
- `GET /api/previews/storage` — how much disk the orchestrator occupies, measured by walking its data dir on each call (so treat it as a user-triggered read, not a poll). Returns totals — `total_bytes`, `artifacts_bytes`, `state_bytes`, `logs_bytes`, `mirror_bytes`, `tmp_bytes`, `db_bytes` — plus `repos`, one row per repo with the same category fields, `total_bytes`, `deploys`, `evicted_deploys`, and the owning board's `board_id`/`board_name`/`board_slug` (omitted when a repo outlives its board).
- `GET /api/previews/retention` — the retention policy: `{ "max_deploys_per_repo", "max_age_days" }`. Either limit at `0` means unlimited; both at `0` (the default) disables eviction.
- `PUT /api/previews/retention` — replaces the policy, returning it. Negative limits are `400`. It takes effect on the next sweep — hourly, or immediately via the endpoint below — so tightening limits never surprise-evicts on save.
- `POST /api/previews/gc` — runs one retention sweep now. Responds with `{ "policy", "evicted": [{ "id", "repo", "short_sha", "branch" }], "freed_bytes" }`. Eviction reclaims a deploy's build output and keeps its row as history; redeploying the commit rebuilds it. Never evicted: queued and building deploys, and each repo's newest ready deploy. With retention disabled the sweep still collects stale staging leftovers.
- `GET /api/previews/{id}/logs` — plain-text snapshot of the deploy's build logs: frontend, backend, and one section per declared artifact.
- `GET /api/previews/{id}/artifacts/{artifact}/{file}` — downloads one file from a ready deploy's `[artifacts.<name>]` section, served as `application/octet-stream` with an `attachment` disposition. `{artifact}` is the manifest's artifact name, `{file}` its **base** name (`bin/preview-linux-amd64` downloads as `preview-linux-amd64`), which is why base names must be unique within an artifact. Artifacts are content-addressed and immutable, so responses are cached for a year. Returns `404` for an unknown deploy, artifact, or file — and for a deploy that isn't ready yet.

A ready deploy's `artifacts` is a list of `{ "name", "hash", "files": [{ "name", "size" }] }` — per-commit build outputs the manifest publishes for download (a CLI binary, say) instead of running. Deploys whose manifest declares no `[artifacts.*]` sections omit the field.

Deploys also fire automatically when a session reports idle (the agent finished a work burst), gated on the board being onboarded — see [Previews](../guide/previews).

## Config

A generic read/write surface over the layered kanban config (see [Configuration](../guide/configuration)). Mirrors `git config`: `global` scope targets the user config (`~/.config/kanban/config.toml`), `local` scope targets a board's project `.kanban.toml`. Keys are dotted, e.g. `sync.allow_rebase` or `github.draft_column`. The older `GET`/`PATCH /api/settings` endpoint that backs the web Settings dialog still exists and edits the same files through the same writer.

### `GET /api/config`

Query params:

- `scope` — `effective` (default), `local`, or `global`.
- `board` — board id or slug; selects the local layer for an `effective` view, and is **required** for `local`.

Returns `{ "scope", "board", "entries": [ { "key", "value", "source", "writable" } ] }`.

- `effective` lists **every** config key (set or not) with its merged value and `source` (`local`, `global`, or `default`), then read-only **runtime** settings (`data_dir`, `worktrees_dir`, `port_range_start`, `port_range_end`) with `source: "runtime"`, `writable: false` — these come from flags/env, not TOML.
- `local` / `global` list only the keys actually set in that single file.

### `PATCH /api/config`

Body: `{ "scope", "board", "set": { "<key>": <value> }, "unset": ["<key>"] }`.

- `scope` is `local` or `global`; `board` (id or slug) is required for `local`.
- `set` values are native JSON typed per key: bool/string for scalars, an array for `devcontainer.run_args`/`mounts`, an object for `devcontainer.container_env`, an array of objects for `task`/`buildcop.boards`. A single map entry is addressable as `devcontainer.container_env.<NAME>`.
- `unset` removes keys and prunes any section it leaves empty.
- Returns the refreshed view for the same scope.

Errors: `400` for an unknown key, a type mismatch, a JSON `null` in `set` (use `unset` to clear), an unknown `harness.id`, an invalid `buildcop.interval`, an invalid `scope`, or `local` without a `board`; `409` if `worktrees.root` is locked by `--worktrees-dir` / `$KANBAN_WORKTREES_DIR`; `422` if the chosen board has no on-disk repo (a `mount_path`-only board, or a `repo_path` that no longer exists).

Writes go through a single atomic, per-file lock-serialized writer, but rewrite the whole TOML file — comments and key ordering are **not** preserved. Array values (`devcontainer.run_args`/`mounts`) append across layers rather than override (see [merge semantics](../guide/configuration#merge-semantics)).

## Filesystem

- `GET /api/fs/check?path=<path>` — returns `{ "state": "git" | "not_git" | "unknown" }` for the given path as seen from inside the kanban server. `git` means the path is a directory containing `.git`; `not_git` means it exists but isn't a repo; `unknown` means it isn't visible to the kanban container (it may still be a valid host path that dockerd can mount when a session spawns — kanban just can't `stat` it). Used by the UI to badge the board-settings icon when a board's mount path looks like a git repo but no `repo_path` is configured.

## Health & metadata

- `GET /api/health` — returns `200 OK` with `{"status":"ok"}` when both the SQLite database and the Docker daemon are reachable. Returns `503 Service Unavailable` with `{"db":"..."}` or `{"docker":"..."}` otherwise. Used by the container `HEALTHCHECK` directive in the published image so orchestrators can detect a wedged process.
- `GET /api/version` — returns `{ "version": "<semver-or-sha>" }`.
- `GET /metrics` — Prometheus exposition format. Lists Go runtime/process metrics plus kanban-specific series: `kanban_http_requests_total`, `kanban_http_request_duration_seconds`, `kanban_db_query_duration_seconds`, `kanban_db_query_errors_total`, `kanban_github_api_requests_total`, `kanban_github_api_request_duration_seconds`, `kanban_github_rate_limit_remaining`, `kanban_github_rate_limit_limit`, and `kanban_github_rate_limit_reset_timestamp_seconds`. See [observability](../guide/observability) for the bundled Prometheus service that scrapes this endpoint.
- `GET /prometheus/...` — reverse-proxy to the Prometheus UI when `KANBAN_PROMETHEUS_URL` is set. Used by the bundled `prometheus` compose service; returns 503 in dev runs without Prometheus.

## Frontend error reporting

- `POST /api/errors` — used internally by the web UI to file frontend exceptions
  as tickets when error reporting is enabled. The body is
  `{ message, stack, source, url, user_agent, meta }`; `meta` is an optional
  string map merged into the ticket body. Sources used today:
  `boundary` (React render errors), `react-query` (query/mutation 5xx),
  `window` (uncaught exceptions), `unhandledrejection` (rejected promises with
  no `.catch`), and `longtask` (main-thread blocks ≥2s reported by the
  PerformanceObserver — useful for catching UI freezes that throw nothing).
  Always returns 204. Every accepted report is also written to the server's
  stdout logger (one `client error: source=… url=… message=…` line plus the
  stack), so frontend errors show up in `docker logs` even when
  `[errors] enabled` is false; ticket creation is the only path gated by that
  flag.
