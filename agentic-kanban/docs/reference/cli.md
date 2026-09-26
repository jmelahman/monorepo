# CLI

The `kanban` binary runs the server and the MCP server. It also has commands for working with a running server from your terminal.

```text
kanban
├── serve              Start the server
├── mcp                Run the MCP server over stdio
├── board
│   ├── list           List boards
│   ├── create         Create a board
│   ├── get            Show a board
│   ├── update         Change a board's settings
│   ├── delete         Delete a board and stop its sessions
│   ├── state          Print a board's full state as JSON
│   ├── archived       List archived tickets
│   └── archived-clear Delete every archived ticket
├── ticket
│   ├── create         Create a ticket and attach to its agent
│   ├── info           Show a ticket's details
│   ├── attach         Attach your terminal to a ticket's agent
│   ├── tasks          List, run, and stop a ticket's tasks
│   ├── update         Change a ticket's title or description
│   ├── move           Move a ticket to another column
│   ├── archive        Archive tickets
│   ├── unarchive      Restore an archived ticket
│   ├── delete         Delete an archived ticket
│   ├── done           Move a ticket to the last column and stop its session
│   ├── sync           Update a ticket's branch from its base
│   └── merge          Merge a ticket's branch into its base
├── column
│   └── archive-all    Archive every ticket in a column
├── session
│   ├── ensure         Create a ticket's session if it has none
│   ├── start          Start a session
│   ├── stop           Stop a session
│   └── restart        Restart a session
├── config
│   ├── list           List settings
│   ├── get            Print one setting
│   ├── set            Change a setting
│   └── unset          Remove a setting
└── env
    ├── list           List a board's environment variable names
    ├── set            Set environment variables
    └── unset          Remove environment variables
```

`kanban --version` prints the version.

## Common flags

Every command except `serve` and `mcp` connects to the server at `--server`, which defaults to `http://localhost:7474`. You can also set it with `$KANBAN_URL`. Commands that print a summary accept `--json` to print the full API response instead.

### Finding the board

Commands that take an optional board or ticket ID work out the board from your current directory, using the git repo you're in. If several boards use the same repo, kanban picks the one whose [project directory](/guide/monorepos) best matches where you are. If it can't decide, pass the ID or `--board`.

Commands that change or delete a board always need an explicit ID.

## `serve`

```sh
kanban serve [flags]
```

| Flag                 | Default                              | Description |
| -------------------- | ------------------------------------ | ----------- |
| `--addr`             | `:7474`                              | Address to listen on. |
| `--data-dir`         | XDG data dir                         | Where the database and worktrees are stored. Env: `KANBAN_DATA_DIR`. |
| `--worktrees-dir`    | `<data-dir>/worktrees`               | Where worktrees are created. Env: `KANBAN_WORKTREES_DIR`. |
| `--config`           | `~/.config/kanban/config.toml`       | Your user config file. Env: `KANBAN_CONFIG`. |
| `--port-range-start` | `13000`                              | First host port for forwarding task ports. |
| `--port-range-end`   | `13099`                              | Last host port for forwarding task ports. |
| `--claude-config`    | `true`                               | Mount your `~/.claude` into the bundled session image. Overrides `[devcontainer].claude_config` when set. Env: `KANBAN_CLAUDE_CONFIG`. |
| `--in-memory`        | `false`                              | Use a temporary database that's deleted on shutdown. For testing only. |

## `mcp`

```sh
kanban mcp [--server URL]
```

Runs the MCP server over stdio. It needs `kanban serve` to be running. See the [MCP reference](./mcp) for setup and the list of tools.

## `board`

### `board list`

Prints every board as an `ID SLUG NAME` table.

### `board create`

```sh
kanban board create [flags]
```

Inside a git repo, `kanban board create` needs no flags. It uses the current repo and names the board after it. Run it from a subdirectory and it also sets the [project directory](/guide/monorepos) and names the board after that subdirectory. Flags you pass override what it guesses.

This assumes the CLI and server see the same filesystem. If the server runs in Docker with different paths, pass `--repo-path` or `--mount-path` yourself.

| Flag              | Description |
| ----------------- | ----------- |
| `--name`          | Board name. Defaults to the directory name. |
| `--repo-path`     | Path to the git repo. Defaults to the current repo. |
| `--mount-path`    | Directory to mount into sessions, for boards without a repo. |
| `--project-dir`   | Subdirectory the agent works from. Requires `--repo-path`. |
| `--worktree-root` | Where to create worktrees. |
| `--base-branch`   | Branch that tickets start from. Detected from the repo by default. |
| `--branch-prefix` | Prefix for ticket branch names. |
| `--json`          | Print the created board as JSON. |

### `board get [id]`

Prints a board.

### `board update <id> [flags]`

Changes the board settings you pass. Takes the same flags as `board create`.

### `board delete <id>`

Deletes a board and stops all its sessions.

### `board state [id]`

Prints the board with all its columns, tickets, and sessions as JSON. Useful with `jq`.

### `board archived [id]`

Lists archived tickets as an `ID SLUG TITLE` table.

### `board archived-clear <id>`

Permanently deletes every archived ticket on the board.

## `ticket`

Every `ticket` command takes an optional ticket ID. Without one, it opens a list of the board's tickets for you to pick from:

- Type to filter by ID, title, column, or status. For example, `#12`, `login`, or `progress idle`.
- `↑`/`↓` move, `Enter` runs the command on the selected ticket, and `Esc` cancels.
- For `archive`, `Tab` marks several tickets and `Enter` archives all of them.
- For `unarchive` and `delete`, the list shows archived tickets.

The list needs an interactive terminal. In scripts, pass the ID.

Use `--board` to pick from a board other than the one for your current directory.

### `ticket create`

```sh
kanban ticket create [flags]
```

Run inside a repo with no flags, this opens a form for the title and description. It then creates the ticket, starts its session, and attaches your terminal to the agent. The first session start pulls or builds the container image, which can take a few minutes.

In the form, `Tab` switches fields and `Ctrl+S` creates the ticket. If more than one agent is available, a **Harness** row lets you choose one with `←`/`→`.

Passing `--title` skips the form and doesn't attach unless you add `--attach`. In scripts, `--title` is required.

| Flag            | Default                          | Description |
| --------------- | -------------------------------- | ----------- |
| `--board`       | current repo's board             | Board ID or slug. |
| `--title`       |                                  | Ticket title. Skips the form. |
| `--body`        |                                  | Markdown description. Fills in the form if there's no `--title`. |
| `--column`      | leftmost column                  | Column name or ID. |
| `--attach`      | `true` with the form, else `false` | Start the session and attach to the agent. |
| `--harness`     | board default                    | Agent to use, such as `claude` or `pi`. Needs `--attach`. |
| `--detach-keys` | `ctrl-p,ctrl-q`                  | Keys that detach from the agent. |
| `--json`        | `false`                          | Print the ticket as JSON. |

```sh
# Describe the task, then work with the agent.
kanban ticket create

# Create it but stay in your shell.
kanban ticket create --attach=false

# From a script.
kanban ticket create --board playground --title "Wire CI" --column "In Progress"
```

### `ticket info [id]`

Shows a ticket's description, session, container, branch, pull request, and ports: the same details as the **info** tab in the web UI.

In a terminal it opens a viewer where `Enter` copies the selected value. `ID` copies the plain number, `PR` copies its title and URL, and a port copies its `http://localhost` URL. Piped, it prints plain text. `--json` prints `{ticket, board, column, session, ports}`.

Copying works over SSH and inside containers, in terminals that support OSC 52. With tmux, turn on `set-clipboard`.

```sh
kanban ticket info 42 --json | jq -r .session.worktree_path
```

### `ticket attach [id]`

```sh
kanban ticket attach [id] [--shell] [--harness <id>] [--detach-keys <keys>]
```

Connects your terminal to the ticket's agent. It's the same session the web UI shows, so you can switch between the two. If the session isn't running, it's started first.

Press `Ctrl+P` then `Ctrl+Q` to detach. The agent keeps running, and attaching again restores the scrollback. Change the keys with `--detach-keys`, which takes a comma-separated list such as `ctrl-x,q`.

To change the ticket's agent, pass `--harness`, or use `←`/`→` on the **Harness** row of the ticket list. If a different agent is running, it's stopped and the new one starts. The container and worktree stay as they are, and the choice applies from then on, including in the web UI.

| Flag            | Default                   | Description |
| --------------- | ------------------------- | ----------- |
| `--shell`       | `false`                   | Open a shell in the container instead of the agent. |
| `--harness`     | the session's current one | Switch to this agent before attaching. |
| `--detach-keys` | `ctrl-p,ctrl-q`           | Keys that detach. |
| `--board`       | current repo's board      | Board to pick a ticket from. |

### `ticket tasks [id]`

```sh
kanban ticket tasks [id]
kanban ticket tasks [id] --run <label> [--detach]
kanban ticket tasks [id] --stop <label>
```

Works with the tasks in the ticket's `.vscode/tasks.json` and `.vscode/launch.json`. Tasks run in the session container.

With no flags, it opens a view listing each task with its port, status, and URL, plus the selected task's output:

```
 #42 Fix the login bug · tasks
 session #7 idle

   TASK             PORT   STATUS            URL
 ▸ Kanban Frontend  5173   running (run #5)  http://localhost:13001
   Run Tests        -      exited 0          -

 ── output · Kanban Frontend (run #5) ─────────────────────────────
   VITE v6.0.0  ready in 312 ms
```

| Key            | Action |
| -------------- | ------ |
| `↑`/`↓`        | Select a task. |
| `Enter` / `r`  | Run the task, or stop it if it's running. |
| `s`            | Stop the task. |
| `c`            | Copy the task's URL. |
| `PgUp`/`PgDn`  | Scroll the output. |
| `q` / `Esc`    | Close. Running tasks keep running. |

`--run <label>` starts the session if needed and runs the task. If the task has a [port mapping](/guide/configuration#task-ports), it prints the URL, then streams the output until the task exits. `Ctrl+C` stops the task. With `--detach`, the command returns as soon as the task is running.

`--stop <label>` stops the task. Piped, the command prints a table. `--json` prints the tasks with their URLs and last runs.

```sh
kanban ticket tasks 42 --run "Kanban Frontend"
kanban ticket tasks 42 --run "Kanban Backend" -d
kanban ticket tasks 42 --stop "Kanban Backend"
```

### `ticket update [id]`

Changes a ticket's `--title` and/or `--body`.

### `ticket move [id] --column-id <id> [--position <n>]`

Moves a ticket. Find column IDs with `kanban board state`.

### `ticket archive [id...]`

Archives tickets and stops their sessions. With `--delete`, deletes them too. If one ticket fails, the rest are still processed, and the command exits with an error.

```sh
kanban ticket archive 12 14 --delete
```

### `ticket unarchive [id]`

Restores an archived ticket.

### `ticket delete [id]`

Permanently deletes an archived ticket. To archive and delete in one step, use `ticket archive --delete`.

### `ticket done [id]`

Moves the ticket to the last column and stops its session.

### `ticket sync [id] [--strategy rebase|merge]`

Brings the base branch into the ticket's branch. Defaults to `rebase`.

### `ticket merge [id] [--strategy merge-commit|squash|rebase]`

Merges the ticket's branch into the base branch. Without `--strategy`, it uses the board's [default strategy](/guide/configuration#default-merge-strategy), or the only allowed one. Otherwise `--strategy` is required. For a merge commit, use `merge-commit`, not `merge`.

The base branch must be checked out in the repo with no uncommitted changes to tracked files.

## `column archive-all <id>`

Archives every ticket in the column. Find column IDs with `kanban board state`.

## `session`

| Command                          | Description |
| -------------------------------- | ----------- |
| `session ensure --ticket <id>`   | Creates the ticket's session if it has none. |
| `session start <id>`             | Starts a session. |
| `session stop <id>`              | Stops a session. |
| `session restart <id>`           | Restarts a session. |

These take a session ID, except `ensure`, which takes a ticket ID.

## `config`

Reads and changes [configuration](/guide/configuration), like `git config`. Keys use dots, such as `sync.allow_rebase`.

- `--global` uses your user config.
- `--local --board <id>` uses that board's `.kanban.toml`.
- Without either, `list` and `get` show the merged settings. `set` and `unset` require one.

```sh
kanban config list
kanban config get merge.default_strategy
kanban config set sync.allow_rebase false --global
kanban config set branches.prefix feat --local --board playground
kanban config set devcontainer.run_args '["--cap-add=NET_ADMIN"]' --global
kanban config set devcontainer.container_env.FOO bar --global
kanban config unset github.draft_column --global
```

Lists and tables take a JSON value. `set` and `unset` rewrite the file, so comments are lost.

## `env`

Manages a board's [environment variables](/guide/configuration#board-environment-variables). Values are secret and can't be printed.

```sh
kanban env list playground
kanban env set playground MY_API_KEY=sk-abc123 OTHER_TOKEN=xyz
kanban env unset playground MY_API_KEY
```

## Environment variables

| Variable               | Equivalent flag         |
| ---------------------- | ----------------------- |
| `KANBAN_URL`           | `--server`              |
| `KANBAN_CONFIG`        | `serve --config`        |
| `KANBAN_DATA_DIR`      | `serve --data-dir`      |
| `KANBAN_WORKTREES_DIR` | `serve --worktrees-dir` |
| `KANBAN_CLAUDE_CONFIG` | `serve --claude-config` |

A flag you pass explicitly wins over its environment variable.
