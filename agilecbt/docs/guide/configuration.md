# Configuration

Configuration is intentionally small: a few flags on `agilecbt serve` plus
environment variables.

## Data directory

The SQLite database lives in the data directory, resolved in order:

1. `--data-dir` flag
2. `$APP_DATA_DIR`
3. `$XDG_DATA_HOME/agilecbt`
4. `~/.local/share/agilecbt`

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--addr` | `:8080` | HTTP listen address |
| `--data-dir` | (XDG) | Override the data directory |
| `--in-memory` | `false` | Ephemeral in-memory SQLite; all data is discarded on shutdown |

## Environment variables

| Variable | Used by | Description |
| --- | --- | --- |
| `APP_SECRET` | server, CLI | Shared secret. When set, the web UI asks for it and API/MCP clients send it as `Authorization: Bearer`. Unset means no auth |
| `APP_LLM` | server | AI curator backend: `claude-code` (default), `ollama`, `anthropic`, or `none` |
| `APP_MODEL` | server | Model override for the chosen backend |
| `OLLAMA_HOST` | server | Ollama base URL (default `http://localhost:11434`; a bare `host:port` works) |
| `ANTHROPIC_API_KEY` | server | API key for `APP_LLM=anthropic` |
| `APP_SELF_URL` | server | How the `claude` subprocess reaches this server's `/mcp`. Defaults to loopback on the `--addr` port |
| `APP_DATA_DIR` | server | Data directory override |
| `APP_URL` | CLI subcommands | Server base URL (an explicit `--server` flag wins) |
| `APP_BACKEND` | `web/` dev server | Backend `host:port` the Vite proxy targets |

See [AI curator & MCP](/guide/ai) for how each `APP_LLM` backend works and
what it needs.

::: warning Reaching it from your phone
This app holds sensitive personal data. If `--addr` listens beyond localhost,
for example `:8080` in Docker or on a Tailscale address, set `APP_SECRET`. The
server logs a warning when it doesn't. Put it behind HTTPS, such as
`tailscale serve` or a reverse proxy, so the session cookie is marked `Secure`.
:::

## In-app settings

The Settings page (or `PATCH /api/settings`) stores the following in the
database:

- **Check-ins**: morning, evening, or both. This sets which check-in the Today
  screen offers.
- **Crisis resources**: the text behind the "Need help now?" button. The
  curator also sees it. The default lists 988 (US) and findahelpline.com.

Settings also has an **Appearance** section. These choices are stored per
browser, not in the database:

- **Mode**: system, light, or dark.
- **Style**: *Warm* (the default: soft stone neutrals and rounded corners) or
  *Drafting* (charcoal and white neutrals, IBM Plex type, hairline rules,
  square corners, and condensed caps for headings and labels).
- **Accent**: the single colour used for buttons, focus rings, and progress:
  apricot (the default), signal red, sage, sky, or violet. Each accent has its
  own light and dark shades and works with either style. Drafting with signal
  red gives the high-contrast "engineering drawing" look.
