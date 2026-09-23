# CLI

The binary is both the server and a client for it. Client subcommands talk to
a running `app serve` over HTTP; point them at a non-default server with
`--server` or `$APP_URL`.

## `app serve`

Start the HTTP server.

| Flag | Default | Description |
| --- | --- | --- |
| `--addr` | `:8080` | HTTP listen address |
| `--data-dir` | (XDG) | Override the data directory |
| `--in-memory` | `false` | Ephemeral in-memory SQLite |

## `app item`

| Command | Description |
| --- | --- |
| `app item list` | List items as a table |
| `app item create --title <title>` | Create an item (`--json` for the full body) |
| `app item delete <id>` | Delete an item |

## `app version`

`app --version` prints the build version (populated from `git describe` at
release time).
