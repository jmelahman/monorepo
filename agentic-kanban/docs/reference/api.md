# REST API

The server exposes a JSON API on port 7474. This page covers the endpoints that are useful for scripting. The complete route list is in [`internal/api/api.go`](https://github.com/jmelahman/agentic-kanban/blob/master/internal/api/api.go).

::: warning No authentication
The API has no authentication. Only expose it on `127.0.0.1`.
:::

## Conventions

- Request and response bodies are JSON.
- `{id}` in a board path accepts the numeric ID or the slug.
- Errors return `{ "error": "<message>" }` with a 4xx or 5xx status.
- Responses of 1 KB or more are gzipped when the client sends `Accept-Encoding: gzip`.

## Tickets

### `POST /api/boards/{id}/tickets`

Creates a ticket. Returns `201` with the ticket.

| Field       | Type    | Required | Notes                                                                    |
| ----------- | ------- | -------- | ------------------------------------------------------------------------ |
| `title`     | string  | yes      |                                                                          |
| `body`      | string  | no       | Markdown description.                                                    |
| `column_id` | integer | no       | Takes priority over `column`.                                            |
| `column`    | string  | no       | Column name (case-insensitive) or ID. Defaults to the leftmost column.   |

```sh
curl -fsS -X POST http://localhost:7474/api/boards/playground/tickets \
  -H 'Content-Type: application/json' \
  -d '{"title":"Investigate flaky test","body":"Fails on CI but not locally."}'
```

### `GET /api/tickets/{id}`

Returns one ticket, including its `board_id` and `column_id`.

### `PATCH /api/tickets/{id}`

Updates `title` and/or `body`.

### `PATCH /api/tickets/{id}/move`

Moves a ticket. Body: `{"column_id": <id>, "position": <int>}`.

### `POST /api/tickets/{id}/archive`, `POST /api/tickets/{id}/unarchive`

Archives or restores a ticket.

### `DELETE /api/tickets/{id}`

Deletes a ticket along with its session and worktree.

### `GET /api/boards/{id}/archived`

Lists a board's archived tickets.

## Boards

### `GET /api/boards`

Lists boards in display order.

### `GET /api/boards/{id}`

Returns one board.

### `GET /api/boards/{id}/state`

Returns the board with all its columns, tickets, and sessions in one response.

### `POST /api/boards`

Creates a board with the default columns.

| Field              | Type   | Notes                                                                                                   |
| ------------------ | ------ | ------------------------------------------------------------------------------------------------------- |
| `name`             | string | Required. The slug is derived from it.                                                                  |
| `repo_path`        | string | Path to the git repo. Either this or `mount_path` is required.                                          |
| `mount_path`       | string | Directory to mount into sessions when there's no repo.                                                  |
| `project_dir`      | string | Subdirectory the agent works from. Requires `repo_path` and no `mount_path`. See [Monorepos](/guide/monorepos). |
| `worktree_root`    | string | Where to create worktrees. Defaults to `<data-dir>/worktrees/<slug>`.                                   |
| `base_branch`      | string | Branch that tickets start from. Detected from the repo if omitted, falling back to `main`.              |
| `branch_prefix`    | string | Prefix for ticket branch names.                                                                         |
| `git_author_name`  | string | Name for kanban's merge commits.                                                                        |
| `git_author_email` | string | Email for kanban's merge commits. Only used when both author fields are set.                            |

Before creating a worktree, kanban fetches `origin/<base_branch>` and starts from it if it's ahead of the local branch.

### `PATCH /api/boards/{id}`

Updates any of the fields above. Omitted fields are unchanged. An empty string clears a field.

### `PATCH /api/boards/{id}/move`

Reorders boards. Body: `{"position": <int>}`, the board's new zero-based index.

### `GET /api/boards/{id}/env`

Returns the board's environment variable names as `{"keys": [...]}`. Values are never returned.

### `PATCH /api/boards/{id}/env`

Sets and removes board environment variables. Body: `{"set": {"KEY": "value"}, "unset": ["OTHER_KEY"]}`. Returns the updated key list. Keys must match `[A-Za-z_][A-Za-z0-9_]*` and can't start with `KANBAN_`. Changes apply the next time a session starts. See [Board environment variables](/guide/configuration#board-environment-variables).

## Live updates

### `GET /api/events?boards=<id>,<id>`

A Server-Sent Events stream of changes on one or more boards. Each event includes the `board_id` it belongs to:

```
event: ticket_updated
data: {"type":"ticket_updated","data":{...},"board_id":42}
```

Event types include `ticket_created`, `ticket_updated`, `ticket_moved`, `ticket_archived`, `session_status_changed`, `session_pull_progress`, and `task_run_*`.

`session_pull_progress` reports image download progress while a session starts: `{ session_id, image, current, total, layers, status, done }`, where `current` and `total` are bytes.

### `GET /api/boards/{id}/events`

The same stream for a single board, without `board_id`. When watching several boards, use `/api/events` instead. Browsers allow only six connections per host, so one stream per board can block other requests.

## Sessions

### `POST /api/tickets/{id}/session`

Returns the ticket's session, creating it if needed. If the session's container has stopped since it started (after a reboot, for example), the session is marked `stopped`.

### `POST /api/sessions/{id}/start`, `stop`, `restart`

Starts, stops, or restarts a session. Starting a running session does nothing.

### `GET /api/sessions/summary`

Counts active sessions across all boards: `{ "running", "working", "awaiting_perm", "idle", "starting" }`. `running` is the total of the other four.

### `PUT /api/sessions/{id}/harness`

Sets the session's agent. Body: `{ "harness": "<id>" }`, using an ID from `/api/harnesses`. An empty string goes back to the default. If a different agent is running, it's stopped, and the new one starts the next time someone attaches. The container and shell keep running.

### `GET /api/harnesses?board=<id>`

Lists available agents as `[{ "id", "label" }]`. With `board`, the board's default is marked `"default": true`.

### `PATCH /api/sessions/{id}/branch`

Changes which branch the session tracks. Body: `{ "branch_name": "<branch>" }`. Use it when the agent moves to a different branch. This only updates kanban's record: it doesn't rename anything in git. The ticket's linked pull request is cleared and found again for the new branch.

### `GET /api/sessions/{id}/diff`

Returns `{ "base", "patch" }`: a unified diff from where the branch left its base to the current working tree. It includes commits, uncommitted edits, and new files, and respects `.gitignore`. An empty `patch` means no changes.

### `GET /api/sessions/{id}/file?path=<path>`

Returns `{ "path", "contents" }` for a file in the worktree. Paths outside the worktree return `404`.

### `GET /api/sessions/{id}/file-diff?path=<path>&old_path=<path>`

Returns both versions of a changed file: `{ "path", "old_contents", "new_contents" }`. The old version comes from the same base as `/diff`. Pass `old_path` for renamed files. An empty side means the file was added or deleted.

### `GET /api/sessions/{id}/pr-detail`

Returns the linked pull request's diff size, review status, and check results, fetched live from GitHub. Returns `404` if there's no pull request and `502` if GitHub can't be reached.

### `GET /api/sessions/{id}/ports`

Lists the session's forwarded ports.

### Used by agent hooks

- `PATCH /api/sessions/{id}/status` reports the agent's status.
- `PATCH /api/sessions/{id}/claude-session` saves the Claude Code conversation ID so it can resume. Body: `{ "claude_session_id": "<uuid>" }`.

## Previews

See [Previews](/guide/previews). These endpoints return `503` if previews couldn't start.

A deploy has a `status` (`queued`, `building`, `ready`, `failed`, or `evicted`) and a `short_sha`. Once ready, it also has `preview_url`, the backend's `process` state, and any `artifacts`, each listed as `{ "name", "hash", "files": [{ "name", "size" }] }`.

| Endpoint                                          | Description |
| ------------------------------------------------- | ----------- |
| `GET /api/sessions/{id}/previews`                 | Deploys of the session's branch, newest first. |
| `POST /api/sessions/{id}/previews`                | Deploys the branch's latest commit. Returns `202`. Returns the existing deploy if that commit is already built. |
| `POST /api/boards/{id}/previews`                  | Deploys any ref. Body: `{ "ref": "<branch, tag, or sha>" }`. Defaults to the base branch. Returns `202`. |
| `GET /api/previews`                               | All deploys on every board, with `board_id`, `board_name`, and `board_slug`. |
| `POST /api/previews/{id}/stop`                    | Stops the deploy's backend. It starts again on the next request. |
| `DELETE /api/previews/{id}`                       | Deletes a deploy and its files. |
| `GET /api/previews/{id}/logs`                     | Build logs as plain text. |
| `GET /api/previews/{id}/artifacts/{artifact}/{file}` | Downloads an artifact file by its name. |
| `GET /api/previews/storage`                       | Disk usage by category and by repo. Slow on large installs, so don't poll it. |
| `GET /api/previews/retention`                     | Retention limits: `{ "max_deploys_per_repo", "max_age_days" }`. `0` means no limit. |
| `PUT /api/previews/retention`                     | Updates the limits. They're applied at the next hourly cleanup. |
| `POST /api/previews/gc`                           | Runs cleanup now. Returns what was removed and `freed_bytes`. |

## Config

Reads and writes the [configuration](/guide/configuration) files. `global` is your user config and `local` is a board's `.kanban.toml`. Keys use dots, such as `sync.allow_rebase`.

### `GET /api/config?scope=<scope>&board=<id>`

`scope` is `effective` (the default), `local`, or `global`. `board` is required for `local`.

Returns `{ "scope", "board", "entries": [{ "key", "value", "source", "writable" }] }`. The `effective` view lists every key with its merged value and where it came from (`local`, `global`, or `default`). It also lists server settings such as `data_dir` with source `runtime`, which can't be changed here. `local` and `global` list only the keys set in that file.

### `PATCH /api/config`

Body: `{ "scope", "board", "set": { "<key>": <value> }, "unset": ["<key>"] }`.

Values are JSON matching the key's type: an array for `devcontainer.mounts`, an object for `devcontainer.container_env`, and so on. You can set a single variable with `devcontainer.container_env.<NAME>`. Returns the updated view.

Errors:

- `400` for an unknown key, a wrong type, or a missing `board` on a `local` write.
- `409` when `worktrees.root` is fixed by `--worktrees-dir`.
- `422` when the board has no repo on disk.

Writes rewrite the whole file, dropping comments.

## Server

| Endpoint                     | Description |
| ---------------------------- | ----------- |
| `GET /api/health`            | `200 {"status":"ok"}` when the database and Docker are reachable, `503` otherwise. |
| `GET /api/version`           | `{ "version": "..." }` |
| `GET /api/fs/check?path=<p>` | `{ "state": "git" \| "not_git" \| "unknown" }`. `unknown` means kanban can't see the path, though Docker might still be able to mount it. |
| `GET /metrics`               | Prometheus metrics. See [Observability](/guide/observability). |
| `GET /prometheus/`           | Proxies the Prometheus UI when `KANBAN_PROMETHEUS_URL` is set. |
| `POST /api/errors`           | Used by the web UI to report frontend errors. |
