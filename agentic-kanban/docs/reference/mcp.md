# MCP

`kanban mcp` runs a [Model Context Protocol](https://modelcontextprotocol.io) server over stdio. It lets AI tools like Claude create and manage tickets. It talks to a running `kanban serve`, so start the server first.

## Setup

### Claude Code

```sh
claude mcp add kanban -- kanban mcp --server http://localhost:7474
```

Run `claude mcp list` to check that it's registered.

### Claude Desktop

Add this to `claude_desktop_config.json` and restart Claude Desktop:

```json
{
  "mcpServers": {
    "kanban": {
      "command": "/path/to/kanban",
      "args": ["mcp", "--server", "http://localhost:7474"]
    }
  }
}
```

`--server` defaults to `http://localhost:7474`. You can also set it with `$KANBAN_URL`.

## Tools

Read tools return JSON. Tools that change something return `"ok"` unless noted. Any `board` argument accepts a numeric ID or a slug.

### Boards

| Tool              | Arguments  | Description |
| ----------------- | ---------- | ----------- |
| `list_boards`     |            | Returns `[{ id, name, slug }]`. |
| `get_board`       | `board`    | Returns one board. |
| `board_state`     | `board`    | Returns the board, its columns, tickets, sessions, and merge and sync settings. |
| `delete_board`    | `board`    | Stops every session on the board and deletes it. |
| `list_archived`   | `board`    | Lists archived tickets. |
| `delete_archived` | `board`    | Permanently deletes every archived ticket. |

#### `create_board`

Creates a board with the columns Backlog, In Progress, Review, and Done.

| Argument           | Type   | Required | Notes |
| ------------------ | ------ | -------- | ----- |
| `name`             | string | yes      | |
| `repo_path`        | string | one of   | Path to the git repo. |
| `mount_path`       | string | one of   | Directory to mount when there's no repo. |
| `project_dir`      | string | no       | Subdirectory the agent works from. Requires `repo_path`. |
| `worktree_root`    | string | no       | Where to create worktrees. |
| `base_branch`      | string | no       | Detected from the repo if omitted. |
| `branch_prefix`    | string | no       | Prefix for ticket branch names. |
| `git_author_name`  | string | no       | Name for kanban's merge commits. |
| `git_author_email` | string | no       | Email for kanban's merge commits. |

#### `update_board`

Takes `board` plus any of the `create_board` arguments. Only the fields you pass are changed. An empty `project_dir` clears it.

### Board environment variables

Values are secret and never returned. Changes apply the next time a session starts.

| Tool              | Arguments                | Description |
| ----------------- | ------------------------ | ----------- |
| `list_board_env`  | `board`                  | Returns `{"keys": [...]}`. |
| `set_board_env`   | `board`, `vars` (object) | Sets variables from a name-to-value map. Names can't start with `KANBAN_`. Returns the updated keys. |
| `unset_board_env` | `board`, `keys` (array)  | Removes variables. Returns the updated keys. |

### Tickets

| Tool               | Arguments                                   | Description |
| ------------------ | ------------------------------------------- | ----------- |
| `create_ticket`    | `board`, `title`, `body`?, `column`?        | `column` is a name or ID. Defaults to the leftmost column. |
| `update_ticket`    | `ticket`, `title`?, `body`?                 | |
| `move_ticket`      | `ticket`, `column_id`, `position`?          | `position` is zero-based and defaults to `0`. |
| `archive_ticket`   | `ticket`                                    | |
| `unarchive_ticket` | `ticket`                                    | |
| `delete_ticket`    | `ticket`                                    | The ticket must be archived first. |
| `done_ticket`      | `ticket`                                    | Moves to the rightmost column and stops the session. |
| `sync_ticket`      | `ticket`, `strategy`?                       | `rebase` (default) or `merge`. |
| `merge_ticket`     | `ticket`, `strategy`?                       | `merge-commit`, `squash`, or `rebase`. Defaults to the board's [default strategy](/guide/configuration#default-merge-strategy). |

`?` marks an optional argument.

### Columns

| Tool                     | Arguments   | Description |
| ------------------------ | ----------- | ----------- |
| `archive_column_tickets` | `column_id` | Archives every ticket in the column and stops their sessions. |

### Sessions

| Tool              | Arguments | Description |
| ----------------- | --------- | ----------- |
| `ensure_session`  | `ticket`  | Returns the ticket's session, creating it if needed. |
| `start_session`   | `session` | |
| `stop_session`    | `session` | |
| `restart_session` | `session` | |

### Config

Reads and writes the [configuration](/guide/configuration). `scope` is `global` (your user file), `local` (a board's `.kanban.toml`, which needs `board`), or `effective` (the merged view, for reads only).

| Tool           | Arguments                                   | Description |
| -------------- | ------------------------------------------- | ----------- |
| `list_config`  | `scope`?, `board`?                          | Lists keys with their values and sources. |
| `get_config`   | `key`, `scope`?, `board`?                   | Returns `{ key, value }`. |
| `set_config`   | `key`, `value`, `scope`, `board`?           | `value` is JSON matching the key's type. |
| `unset_config` | `key`, `scope`, `board`?                    | |

## Adding tools

The tools are defined in [`internal/mcp/server.go`](https://github.com/jmelahman/agentic-kanban/blob/master/internal/mcp/server.go). Each one calls the [REST API](./api).
