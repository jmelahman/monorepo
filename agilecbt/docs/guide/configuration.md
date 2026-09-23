# Configuration

Configuration is intentionally small: a few flags on `app serve`, each with an
environment-variable fallback.

## Data directory

The SQLite database lives in the data directory, resolved in order:

1. `--data-dir` flag
2. `$APP_DATA_DIR`
3. `$XDG_DATA_HOME/app`
4. `~/.local/share/app`

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--addr` | `:8080` | HTTP listen address |
| `--data-dir` | (XDG) | Override the data directory |
| `--in-memory` | `false` | Ephemeral in-memory SQLite; all data is discarded on shutdown |

## Environment variables

| Variable | Used by | Description |
| --- | --- | --- |
| `APP_DATA_DIR` | `app serve` | Data directory override |
| `APP_URL` | CLI subcommands | Server base URL (an explicit `--server` flag wins) |
| `APP_BACKEND` | `web/` dev server | Backend `host:port` the Vite proxy targets |
