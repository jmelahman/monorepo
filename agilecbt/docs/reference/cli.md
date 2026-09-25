# CLI

The `agilecbt` binary is both the server and a client for it. Client
subcommands talk to a running `agilecbt serve` over HTTP. To use a different
server, pass `--server` or set `$APP_URL`. When `$APP_SECRET` is set, it's sent
as a bearer token.

## `agilecbt serve`

Start the HTTP server, which also serves the web UI and `/mcp`.

| Flag | Default | Description |
| --- | --- | --- |
| `--addr` | `:8080` | HTTP listen address |
| `--data-dir` | (XDG) | Override the data directory |
| `--in-memory` | `false` | Ephemeral in-memory SQLite |

Environment variables such as `APP_SECRET` and `APP_LLM` are covered in
[Configuration](/guide/configuration).

## `agilecbt today`

Prints today's check-ins, the steps in Today (with carried-over steps marked),
and what's done so far.

```text
$ agilecbt today
Today, 2026-09-25
morning check-in: mood 4, energy 3, anxiety 6

Today:
  #7 Short walk [energy 1]
```

## `agilecbt goal`

| Command | Description |
| --- | --- |
| `agilecbt goal list [--status active\|resting\|done]` | List goals |
| `agilecbt goal add <title> [--why <text>] [--value <id>]` | Add a goal |

## `agilecbt step`

| Command | Description |
| --- | --- |
| `agilecbt step list [--lane week,today]` | List steps, optionally filtered by lane |
| `agilecbt step add <title> [--lane week] [--energy 1-3] [--goal <id>]` | Add a step (default lane `week`) |
| `agilecbt step move <id> <lane>` | Move a step to `someday`, `week`, `today`, `done`, or `let_go` |
| `agilecbt step done <id> [--mastery 0-10] [--pleasure 0-10]` | Complete a step, optionally with ratings |

## `agilecbt export` / `agilecbt import`

```sh
agilecbt export -o backup.json   # or to stdout without -o
agilecbt import backup.json      # into an empty instance only
```

To move to a new machine, export from the old server. Then start the new one
with a fresh `--data-dir` and import into it.

## `agilecbt version`

`agilecbt --version` prints the build version, which is filled in from
`git describe` at release time.
