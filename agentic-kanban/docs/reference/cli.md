# CLI

The same `kanban` binary serves the HTTP server, runs the MCP server, and
provides noun-verb subcommands for scripting against a running server.

```text
kanban
├── serve              Start the kanban HTTP server
├── mcp                Run kanban as an MCP server over stdio
├── board              Manage kanban boards
│   ├── list           List boards as a table
│   ├── create         Create a new board
│   ├── get            Print a single board
│   ├── update         Update fields on a board
│   ├── delete         Delete a board (and destroy all its sessions)
│   ├── state          Print full board state as JSON
│   ├── archived       List archived tickets on a board
│   └── archived-clear Permanently delete every archived ticket on a board
├── ticket             Manage tickets on a board (every id is optional; omit to pick from a list)
│   ├── create         Create a ticket (interactively by default) and attach to its agent
│   ├── info           Show a ticket's details, with each value copyable to the clipboard
│   ├── attach         Attach your terminal to a ticket's agent
│   ├── update         Update title/body of a ticket
│   ├── move           Move a ticket to a different column / position
│   ├── archive        Archive tickets (--delete to also delete them)
│   ├── unarchive      Unarchive a ticket
│   ├── delete         Permanently delete (must be archived first)
│   ├── done           Move a ticket to the rightmost column and stop its session
│   ├── sync           Sync a ticket branch from base
│   └── merge          Merge a ticket branch into base
├── column             Column-level operations
│   └── archive-all    Archive every ticket in a column
├── session            Manage agent sessions
│   ├── ensure         Ensure a session exists for a ticket
│   ├── start          Start a stopped session
│   ├── stop           Stop a running session
│   └── restart        Restart a session
├── config             Read and write kanban configuration
│   ├── list           List config keys with value and source
│   ├── get            Print one config value
│   ├── set            Set a config key
│   └── unset          Remove a config key
└── env                Manage board environment variables (write-only secrets)
    ├── list           List env var key names on a board
    ├── set            Set env vars on a board
    └── unset          Remove env vars from a board
```

`kanban --version` prints the build version.

## `serve`

Starts the HTTP server.

```sh
kanban serve [flags]
```

| Flag                 | Default                                       | Description                                                                                                                                              |
| -------------------- | --------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--addr`             | `:7474`                                       | HTTP listen address.                                                                                                                                     |
| `--data-dir`         | `$KANBAN_DATA_DIR` or XDG share dir           | Override the data directory (SQLite + worktrees).                                                                                                        |
| `--worktrees-dir`    | `$KANBAN_WORKTREES_DIR` or `<data>/worktrees` | Override where new worktrees are created.                                                                                                                |
| `--config`           | `$KANBAN_CONFIG` or XDG config dir            | Override the user-level kanban config path.                                                                                                              |
| `--port-range-start` | `13000`                                       | First host port available for proxy allocation.                                                                                                          |
| `--port-range-end`   | `13099`                                       | Last host port available for proxy allocation (inclusive).                                                                                               |
| `--in-memory`        | `false`                                       | Dev/test only. Use an ephemeral in-memory SQLite database; **all data is discarded on shutdown**. The server logs `WARNING: --in-memory set` at startup. |
| `--claude-config`    | `true`                                        | Forward host Claude Code config (`~/.claude`, `~/.claude.json`) into built-in session containers. When set explicitly (`--claude-config=false`), overrides `.kanban.toml [devcontainer].claude_config`; otherwise the toml setting wins. Env: `$KANBAN_CLAUDE_CONFIG`.                  |

## `mcp`

Runs kanban as a [Model Context Protocol](https://modelcontextprotocol.io)
server over stdio. The MCP server is a thin client of the HTTP API, so a
separate `kanban serve` must be running.

```sh
kanban mcp [--server URL]
# or, equivalently:
KANBAN_URL=http://localhost:7474 kanban mcp
```

| Flag       | Default                 | Description                         |
| ---------- | ----------------------- | ----------------------------------- |
| `--server` | `http://localhost:7474` | Base URL of the kanban HTTP server. |

See the [MCP reference](./mcp) for tool definitions and Claude Desktop /
Claude Code wiring.

## Common flags

Every subcommand under `board`, `ticket`, `column`, and `session`
inherits a single `--server URL` flag (default `http://localhost:7474`,
overridden by `$KANBAN_URL` when `--server` is not explicitly set).
Read commands that return rich data accept `--json` to print the raw
API JSON instead of a one-line summary.

## `board`

### `board list`

```sh
kanban board list
```

Prints all boards as an `ID SLUG NAME` table.

### `board create`

```sh
kanban board create [flags]
```

Run inside a git repository, a bare `kanban board create` needs no flags:
`--repo-path` defaults to the repo containing the current directory
(resolved to the main working tree when run from a linked worktree) and
`--name` to that directory's name. The server then detects the base branch
and worktree root from the repo as usual. Explicit flags always win —
inference only fills in what you omit. Inference assumes the CLI and the
server share a filesystem (true for a local `kanban serve`); for dockerized
deploys keep passing `--mount-path` explicitly.

Run from a subdirectory of that repo and `--project-dir` is inferred too,
with `--name` defaulting to the subdirectory's name rather than the repo's —
see [Monorepos](/guide/monorepos). This only applies when the repo itself
was inferred: passing `--repo-path` explicitly says nothing about which
directory you happen to be standing in, so no project directory is guessed.

| Flag              | Required | Description                                                         |
| ----------------- | -------- | ------------------------------------------------------------------- |
| `--name`          | no       | Board name. Defaults to the repo directory's name. Required when only `--mount-path` is set. |
| `--repo-path`     | no       | Path to the host git repo. Defaults to the repo containing the current directory; pass it (or `--mount-path`) when running outside a git repo. |
| `--mount-path`    | no       | Mount path inside session containers. Alternative to `--repo-path`. |
| `--project-dir`   | no       | Repo-relative subdirectory the agent works from. The whole repo is still checked out and mounted. Requires `--repo-path`, and conflicts with `--mount-path`. Inferred from the current directory when the repo was. |
| `--worktree-root` | no       | Override the parent directory for new session worktrees.            |
| `--base-branch`   | no       | Branch new session worktrees fork from. Defaults to `main`. Before creating each worktree, kanban best-effort runs `git fetch origin <base-branch>` (10s timeout) and uses `origin/<base-branch>` as the start-point if it ends up strictly ahead of local; otherwise it falls back to local. |
| `--branch-prefix` | no       | Optional prefix prepended to session branch names.                  |
| `--json`          | no       | Print the full board JSON instead of a one-line summary.            |

### `board get [id]`

Prints a single board. `<id>` accepts a numeric id or a slug. When
omitted, the board is inferred from the git repo containing the current
directory — an error if zero or several boards match. The same inference
applies to `board state` and `board archived`; mutating commands
(`update`, `delete`, `archived-clear`) always require an explicit id.

When several boards share one repo path, the current directory decides
between them: kanban compares it against each board's `project_dir` and
keeps the longest match, so standing in `services/api` picks the
`services/api` board and standing at the repo root picks the board with no
`project_dir`. A board with an empty `project_dir` is the repo-wide
catch-all, so a repo with one board behaves exactly as it always has. Two
boards with the same `project_dir` stay ambiguous and still need an
explicit id.

### `board update <id> [flags]`

Patches the supplied fields. Any flag you omit is left untouched.
Same flags as `board create` (minus `--name`, which can also be passed
to rename the board).

### `board delete <id>`

Deletes a board and destroys every associated session.

### `board state [id]`

Prints a single-shot snapshot — `{board, columns, tickets, sessions,
merge_config, sync_config}` — as JSON. Useful piping into `jq`. `<id>`
optional as in `board get`.

### `board archived [id]`

Lists archived tickets on a board as an `ID SLUG TITLE` table. `<id>`
optional as in `board get`.

### `board archived-clear <id>`

Permanently deletes every archived ticket on the board. Destructive.

## `ticket`

Every `ticket` subcommand takes the ticket id as an optional positional
argument. Without one, a full-screen list of the board's tickets opens and
the subcommand acts on the ticket you pick:

- The board is inferred from the git repo containing the current directory
  (the same lookup as `board get` — an error if zero or several boards use
  that repo). `--board <id|slug>` is a persistent flag on `ticket`, so any
  subcommand can be pointed at a board you are not inside.
- Tickets are grouped by column in board order, each with its session
  status (`working`, `idle`, `stopped`, `no session`, …).
- `↑`/`↓` (or `Ctrl+P`/`Ctrl+N`, `PgUp`/`PgDn`, `Home`/`End`) move the
  highlight, and typing narrows the list — every word you type must match
  somewhere in the ticket's `#id`, title, column, or status, so `#12`,
  `login`, and `progress idle` all work.
- The header and the `Enter` key name the pending action ("Archive
  ticket … `Enter` archive"), so a list opened by `archive` can't be
  mistaken for one opened by `attach`. `Enter` runs the subcommand on the
  highlighted ticket; `Esc` cancels without touching anything.
- `archive` accepts several tickets: `Tab` marks the highlighted ticket
  (a `●` in the gutter, count in the header) and moves down, `Shift+Tab`
  does the same moving up. Marks survive filtering, and `Enter` acts on
  every marked ticket — or just the highlighted one if none are marked.
- `unarchive` and `delete` act only on archived tickets, so their lists
  show the board's archived tickets instead of its open ones.
- The list needs a real terminal, so in scripts and pipes the id is
  required — the command fails before it fetches anything rather than
  hanging.

`ticket create` is the exception: there is no ticket to pick yet, so it
prompts with a form instead.

### `ticket create`

```sh
kanban ticket create [flags]
```

Run inside a board's git repository, a bare `kanban ticket create` is the
fastest way from "I have an idea" to "an agent is working on it":

1. The board is inferred from the repo containing the current directory
   (the same lookup as `board get` — an error if zero or several boards
   use that repo). Pass `--board` to pick one explicitly.
2. A full-screen form asks for a title and an optional markdown
   description. `Tab` switches fields, `Enter` in the title jumps to the
   description, `Ctrl+S` creates the ticket, `Esc` cancels without
   creating anything. Pasting multi-line text into the description works
   (bracketed paste). `--body` pre-fills the description. When the
   server offers more than one agent harness and the ticket will be
   attached, the form has a **Harness** row between the title and the
   description. It starts on the board's default. Move to it with `Tab`,
   then press `←`/`→` to switch harnesses. `--harness` sets the starting
   value.
3. The ticket is created in the leftmost column (or `--column`), its
   session is started — a first start pulls or builds the devcontainer
   image, which can take a few minutes — and your terminal attaches to
   the agent inside the container, exactly as `ticket attach` would.

With `--title` the command is non-interactive: no form, and it only
prints the created ticket unless you also pass `--attach`. The form and
attaching both need a real terminal on stdin/stdout, so in scripts and
pipes `--title` is required and `--attach` is rejected before anything
is created.

| Flag            | Default                          | Description                                                                                       |
| --------------- | -------------------------------- | ------------------------------------------------------------------------------------------------- |
| `--board`       | board for the current repo       | Board id or slug.                                                                                 |
| `--title`       | prompted                         | Ticket title. Skips the form.                                                                     |
| `--body`        |                                  | Markdown ticket body. Pre-fills the form's description when `--title` is omitted.                 |
| `--column`      | leftmost column                  | Column name (case-insensitive) or numeric id.                                                     |
| `--attach`      | `true` when prompted, else `false` | Start the session and attach to the agent after creating. `--attach=false` opts out of the interactive default. |
| `--detach-keys` | `ctrl-p,ctrl-q`                  | Key sequence that detaches from the agent (see `ticket attach`).                                  |
| `--harness`     | board default                    | Agent harness to run in the new session (e.g. `claude`, `pi`). Needs `--attach`.                   |
| `--json`        | `false`                          | Print the full ticket JSON instead of a one-line summary.                                         |

```sh
# From inside the repo: prompt for title/description, then work with the agent.
kanban ticket create

# Same, but stay in the shell afterwards.
kanban ticket create --attach=false

# Scripted, no prompt, no attach.
kanban ticket create --board playground --title "Wire CI" --column "In Progress"

# Scripted, then hand the terminal to the agent.
kanban ticket create --title "Wire CI" --attach
```

### `ticket info [id]`

```sh
kanban ticket info [id] [--board <board>] [--json]
```

Shows everything the web UI's Info tab does for a ticket: its description
and board placement, the session working on it, that session's container,
the worktree and branch, any pull request, and the ports the session has
allocated.

On a terminal this opens a full-screen viewer, grouped into `Ticket`,
`Description`, `Session`, `Container`, `Workspace`, `Pull request`, and
`Ports` sections. `↑`/`↓` (or `Ctrl+P`/`Ctrl+N`, `j`/`k`, `PgUp`/`PgDn`,
`Home`/`End`, `g`/`G`) move between values, skipping rows that hold
nothing to copy; `Enter` (or `c` / `y`) copies the highlighted value and
says so in the footer; `Esc` (or `q` / `Ctrl+C`) closes.

A few rows copy something more useful than they show: `ID` copies the
bare number rather than `#42`, `PR` copies the title and URL together,
and a port copies its `http://localhost:<host port>` URL.

Copying writes an OSC 52 escape to your terminal *and*, when one is
installed, pipes the text through a native helper (`pbcopy`, `wl-copy`,
`xclip`, `xsel`, `clip.exe`). Neither covers every case alone — a helper
writes the clipboard of the machine kanban runs on, which is the wrong
one over ssh or from inside a container, while OSC 52 reaches the
terminal you're sitting at but is silently dropped by terminals that
don't implement it (and by tmux/screen without `set-clipboard on`).
Writing both means the text lands wherever it can; if neither works,
`--json` and the plain-text output are always available.

Piped or redirected, the same fields print as plain text. `--json` prints
them as a single object (`{ticket, board, column, session, ports}`).

| Flag      | Default                    | Description                                                            |
| --------- | -------------------------- | ---------------------------------------------------------------------- |
| `--board` | board for the current repo | Board id or slug whose tickets to list. Only used when no id is given. |
| `--json`  | `false`                    | Print the info as JSON instead of opening the viewer.                  |

```sh
# Pick a ticket from the board, then copy values out of it.
kanban ticket info

# Straight to a known ticket.
kanban ticket info 42

# Scripted.
kanban ticket info 42 --json | jq -r .session.worktree_path
```

### `ticket attach [id]`

```sh
kanban ticket attach [id] [--board <board>] [--shell] [--harness <id>] [--detach-keys <keys>]
```

Attaches the current terminal to the agent running in the ticket's
session container — the same PTY the web UI shows, so you can drive
Claude Code (or whichever harness the board uses) from the command line
and switch back to the browser later. The session is created and started
first if it isn't running — including when its container has gone away
since it was last started (a host reboot, a docker restart, an OOM kill):
kanban notices the dead container when you attach, marks the session
stopped, and starts a fresh one before connecting. Keystrokes are
forwarded as typed and the agent's terminal size follows your window.

Without an id you pick a ticket from the board's list, as described
[above](#ticket); `Enter` attaches to the highlighted one. When the
server offers more than one agent harness, the picker adds a **Harness**
row showing which harness the highlighted ticket's session uses. Press
`←`/`→` to change it before pressing `Enter`. Because the arrow keys
belong to the harness row here, move the filter cursor with
`Ctrl+B`/`Ctrl+F`.

If you pick a different harness, or pass `--harness`, and it differs from
what the session uses, the choice is saved on the session before attaching.
If the session's agent is already running under another harness, it is
stopped (sent `SIGHUP`, as when a terminal closes, then `SIGKILL` if it is
still there a few seconds later) and the new harness starts as you attach.
Anyone attached to the old agent sees the session end, and a *working* or
*awaiting permission* status it reported resets to idle. The container, the
worktree and the shell are not touched. The choice sticks: later attaches,
and the web UI, launch the same harness.

Press the detach sequence (default `ctrl-p` then `ctrl-q`, as in
`docker attach`) to return to your shell. Detaching never stops the
session: the agent keeps running and `ticket attach` reconnects with the
scrollback replayed. The command also returns when the agent process
exits. The default sequence deliberately avoids keys the agent uses
(Claude Code binds `Ctrl+]`, for instance); `--detach-keys` accepts a
comma-separated list of single characters or `ctrl-<key>` items.

| Flag            | Default                    | Description                                                                  |
| --------------- | -------------------------- | ---------------------------------------------------------------------------- |
| `--board`       | board for the current repo | Board id or slug whose tickets to list. Only used when no id is given.        |
| `--shell`       | `false`                    | Attach an interactive shell in the session container instead of the agent.   |
| `--harness`     | session's current harness  | Switch the session to this agent harness (e.g. `claude`, `pi`) before attaching. Can't be combined with `--shell`. |
| `--detach-keys` | `ctrl-p,ctrl-q`            | Key sequence that detaches; swallowed, never forwarded to the session.       |

```sh
# Reattach to a ticket you know the id of.
kanban ticket attach 42

# From inside the repo: pick a ticket from the board's list.
kanban ticket attach

# Same, for a board you're not inside.
kanban ticket attach --board playground

# Switch ticket 42's agent to pi (restarts it if running).
kanban ticket attach 42 --harness pi
```

`--server` (or `KANBAN_URL`) must point at a server whose origin check
accepts the request; the CLI sends the server's own host as `Origin`,
which matches a direct `kanban serve` and reverse proxies that preserve
the `Host` header.

### `ticket tasks [id]`

```sh
kanban ticket tasks [id] [--board <board>] [--json]
kanban ticket tasks [id] --run <label> [--detach]
kanban ticket tasks [id] --stop <label>
```

Lists, runs, and stops the tasks a ticket's worktree defines in
`.vscode/tasks.json` and `.vscode/launch.json` — the same ones the web UI's
Tasks tab shows. Tasks run inside the ticket's session container.

Without `--run` or `--stop`, on a terminal it opens a full-screen view:
the tasks on top, each with its container port (from the `[[task]]`
entries in `.kanban.toml`), the status of its latest run, and the URL its
port is proxied to; the highlighted task's live output below.

```
 #42 Fix the login bug · tasks
 session #7 idle

   TASK             PORT   STATUS            URL
 ▸ Kanban Frontend  5173   running (run #5)  http://localhost:13001
   Run Tests        -      exited 0          -

 ── output · Kanban Frontend (run #5) ─────────────────────────────
   VITE v6.0.0  ready in 312 ms
   ➜  Local:   http://localhost:5173/
```

| Key                                   | Action                                                                 |
| ------------------------------------- | ---------------------------------------------------------------------- |
| `↑`/`↓` (`Ctrl+P`/`Ctrl+N`, `k`/`j`)  | Select a task; the output pane follows its latest run.                 |
| `Enter` / `r`                         | Run the task (the same steps as `--run`), or stop it if it's running.  |
| `s`                                   | Stop the task.                                                         |
| `c` / `y`                             | Copy the task's proxied URL to the clipboard (see `ticket info`).      |
| `PgUp`/`PgDn`, `Home`/`End`           | Scroll the output back / return to the live tail.                      |
| `Esc` / `q` / `Ctrl+C`                | Close the view. Tasks started from it keep running.                    |

The list refreshes every couple of seconds, so runs started or stopped
elsewhere (the web UI, another terminal) show up. Opening the view on a
ticket with no session creates one (worktree only — the container starts
when you first run a task).

Piped or redirected, it prints the list as a table instead, and `--json`
prints `{tasks, warnings}`, where each task carries its `host_port`, `url`,
and `last_run`.

`--run <label>` does everything needed to use the task from your machine:

1. Creates and starts the ticket's session if it isn't running (a first
   start pulls or builds the devcontainer image).
2. Runs the task in the container.
3. If the task has a port in `.kanban.toml`, allocates a host port from
   the server's range (`--port-range-start`/`--port-range-end`), opens the
   proxy, and prints the URL, e.g.
   `Kanban Frontend → http://localhost:13001 (container port 5173)`. The
   proxy listens on the server's host, so the URL uses the host from
   `--server`.
4. Streams the task's output to stdout until it exits. Status lines go to
   stderr, so stdout carries only the task's output. `Ctrl+C` stops the
   task (press it again to stop waiting for it to exit). A task that exits
   non-zero makes the command fail.

With `--detach` (`-d`), the command returns once the task has started
and its port is proxied, and the task keeps running. `--stop <label>`
stops every running run of that task.

The table, `--json`, and `--stop` never create a session; on a ticket that
doesn't have one, they tell you to `--run` a task first.

| Flag           | Default                    | Description                                                            |
| -------------- | -------------------------- | ---------------------------------------------------------------------- |
| `--board`      | board for the current repo | Board id or slug whose tickets to list. Only used when no id is given. |
| `--run`        |                            | Run the task with this label and stream its output.                    |
| `--detach, -d` | `false`                    | With `--run`, return once the task has started and leave it running.   |
| `--stop`       |                            | Stop the running task with this label.                                 |
| `--json`       | `false`                    | Print the task list as JSON.                                           |

```sh
# What can this ticket run, and where is it served?
kanban ticket tasks 42

# Run the frontend in the foreground; Ctrl+C stops it.
kanban ticket tasks 42 --run "Kanban Frontend"

# Start the backend in the background, then stop it later.
kanban ticket tasks 42 --run "Kanban Backend" -d
kanban ticket tasks 42 --stop "Kanban Backend"
```

### `ticket update [id] [flags]`

| Flag      | Description                 |
| --------- | --------------------------- |
| `--title` | New title.                  |
| `--body`  | New body.                   |
| `--json`  | Print the full ticket JSON. |

### `ticket move [id] --column-id <int> [--position <int>]`

Moves a ticket. `--column-id` is numeric and required (look up column
ids via `kanban board state <id>`).

### `ticket archive [id...] [--delete]`

Archives one or more tickets, stopping any running session. With no ids
the picker opens in multi-select mode (see above).

| Flag | Default | Meaning |
|---|---|---|
| `--delete` | `false` | Permanently delete each ticket right after archiving it. |

Each ticket is handled independently: a failure on one is reported and the
rest still run, and the command exits non-zero if any failed.

```bash
kanban ticket archive 12 14 --delete   # archive and delete #12 and #14
kanban ticket archive --delete         # pick them from the board
```

### `ticket unarchive [id]`

Restores an archived ticket; its list shows the board's archived tickets.

### `ticket delete [id]`

Permanently deletes a ticket. The ticket must be archived first
(`ticket archive <id>` then `ticket delete <id>`, or both at once with
`ticket archive --delete <id>`), so its list shows the board's archived
tickets.

### `ticket done [id]`

Moves the ticket to the board's rightmost column and stops its session.

### `ticket sync [id] [--strategy rebase|merge]`

Sync a ticket branch from base. Strategy defaults to `rebase`.

### `ticket merge [id] [--strategy merge-commit|squash|rebase]`

Merge a ticket branch into base. `--strategy` is optional when the board can
resolve one on its own: it falls back to
[`merge.default_strategy`](/guide/configuration#default-merge-strategy),
then to the board's only enabled strategy. With several enabled and no default
configured, the strategy is required.

Boards can disable individual strategies via [`merge.allow_*`](/guide/configuration).
Passing a disabled one is rejected with the strategies that board does accept,
so `--strategy rebase` on a squash-only board reports
`strategy rebase is disabled for this board; enabled: squash` rather than
listing all three. Note that `merge` is a **sync** strategy — the merge
equivalent is `merge-commit`.

The source repo must have the base branch checked out with no uncommitted
changes to *tracked* files. Untracked files don't block the merge; if one
would collide with an incoming path, git refuses the merge itself.

## `column`

### `column archive-all <id>`

Archives every non-archived ticket in the column. `<id>` is the numeric
column id.

## `session`

### `session ensure --ticket <id> [--json]`

Ensures a session exists for the given ticket; creates one if absent.

### `session start <id> [--json]` / `session restart <id> [--json]`

Starts or restarts a session by numeric session id.

### `session stop <id>`

Stops a running session.

## `config`

Read and write the layered kanban config (see [Configuration](/guide/configuration)), mirroring `git config`. Keys are dotted, e.g. `sync.allow_rebase` or `github.draft_column`.

Scope flags are shared by every subcommand:

- `--global` — the user config (`~/.config/kanban/config.toml`).
- `--local --board <id-or-slug>` — that board's project `.kanban.toml`.
- Reads (`list`, `get`) default to the merged **effective** view when no scope flag is given; writes (`set`, `unset`) require `--global` or `--local`.

### `config list [--local | --global] [--board <id-or-slug>] [--json]`

Prints a `KEY  VALUE  SOURCE` table. The effective view also includes read-only runtime values (`data_dir`, ports) with source `runtime`.

### `config get <key> [--local | --global] [--board <id-or-slug>] [--json]`

Prints the bare value (so it's pipe-friendly). `--json` prints `{"key","value"}`. A single `devcontainer.container_env` entry is addressable as `devcontainer.container_env.<NAME>`.

### `config set <key> <value> (--global | --local --board <id-or-slug>)`

```sh
kanban config set sync.allow_rebase true --global
kanban config set github.draft_column "Draft" --global
kanban config set branches.prefix feat --local --board playground
# Collections take a JSON value:
kanban config set devcontainer.run_args '["--cap-add=NET_ADMIN"]' --global
kanban config set task '[{"label":"web","container_port":3000}]' --global
kanban config set devcontainer.container_env.FOO bar --global
```

Booleans accept `true`/`false`; strings pass through; arrays/maps/tables take a JSON document.

### `config unset <key> (--global | --local --board <id-or-slug>)`

Removes the key (and prunes any section it leaves empty).

## `env`

Manage per-board environment variables, injected into the board's session
containers at the next session start/restart. Values are write-only secrets:
they're encrypted at rest and no subcommand can print one — only key names
ever come back. See
[per-board environment variables](/guide/configuration#per-board-environment-variables).

### `env list <board>`

Prints the key names as a table. `<board>` is an id or slug.

### `env set <board> KEY=VALUE [KEY=VALUE...]`

```sh
kanban env set playground MY_API_KEY=sk-abc123 OTHER_TOKEN=xyz
```

Sets (or overwrites) variables. Keys must match `[A-Za-z_][A-Za-z0-9_]*`; the
`KANBAN_` prefix is reserved for server-injected variables.

### `env unset <board> KEY [KEY...]`

Removes variables by key name. Removing a missing key is a no-op.

## Environment variables

| Variable               | Used by                                       | Notes                                                           |
| ---------------------- | --------------------------------------------- | --------------------------------------------------------------- |
| `KANBAN_URL`           | `mcp`, `board`, `ticket`, `column`, `session`, `config`, `env` | Default server URL. Overridden by `--server` if explicitly set. |
| `KANBAN_CONFIG`        | `serve`                                       | User-level config path. Overridden by `--config` if set.        |
| `KANBAN_DATA_DIR`      | `serve`                                       | Data directory. Overridden by `--data-dir` if set.              |
| `KANBAN_WORKTREES_DIR` | `serve`                                       | Worktrees directory. Overridden by `--worktrees-dir` if set.    |
| `KANBAN_CLAUDE_CONFIG` | `serve`                                       | Forces built-in `claude_config` forwarding on/off (parsed as bool). Overridden by `--claude-config` if explicitly set. Either source wins over `.kanban.toml`. |
