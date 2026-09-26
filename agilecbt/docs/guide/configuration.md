# Configuration

Configuration is intentionally small: a few flags on `agilecbt serve`, an
optional `config.toml`, and environment variables.

## `config.toml`

`agilecbt serve` reads up to two files, if they exist:

1. `$XDG_CONFIG_HOME/agilecbt/config.toml` (default
   `~/.config/agilecbt/config.toml`)
2. `.config/agilecbt/config.toml` in the working directory, which overrides
   the first

Environment variables override both, even when set to an empty string, and
flags override everything. The server logs each file it reads. Every key is
optional:

```toml
llm = "openai"                          # APP_LLM
base_url = "http://ollama:11434/v1"     # APP_LLM_BASE_URL
api_key = "…"                           # APP_LLM_API_KEY
model = "qwen3.8:27b"                   # APP_MODEL
reasoning_effort = "none"               # APP_LLM_REASONING_EFFORT
secret = "…"                            # APP_SECRET
data_dir = "data"                       # APP_DATA_DIR, relative to this file
crisis_resources = '''
- Call my sister: 555-0100.
- US: call or text 988.
'''                                  # APP_CRISIS_RESOURCES
```

Unknown keys are an error, so a typo doesn't silently fall back to a default.
The repository ships a `.config/agilecbt/config.toml` that points the dev
container at its Ollama service. Don't put `secret` or `api_key` in a file
you commit.

## Crisis resources

`crisis_resources` is the list of crisis lines the coach shares if a
conversation calls for it. Settings → Support shows it, but it can't be edited
in the app, so it can't be changed by accident. The built-in list covers 988 (US), Samaritans (UK & Ireland),
findahelpline.com, and emergency services. Setting `crisis_resources` replaces
it entirely, so include general lines alongside your own people and local
numbers.

Older versions let you edit this list in Settings. If you did, the server keeps
using your edited list until `crisis_resources` is set, and logs a warning at
startup asking you to move it into `config.toml`.

## Data directory

The SQLite database lives in the data directory, resolved in order:

1. `--data-dir` flag
2. `$APP_DATA_DIR`
3. `data_dir` in `config.toml`
4. `$XDG_DATA_HOME/agilecbt`
5. `~/.local/share/agilecbt`

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
| `APP_LLM` | server | AI coach: `openai` (default, any OpenAI-compatible API) or `none` |
| `APP_LLM_BASE_URL` | server | API root before `/chat/completions` (default `http://localhost:11434/v1`, a local Ollama) |
| `APP_LLM_API_KEY` | server | Bearer token for the API, e.g. an OpenRouter key |
| `APP_MODEL` | server | Model id (default `qwen3.8:27b`) |
| `APP_LLM_REASONING_EFFORT` | server | Sent as `reasoning_effort` (default `none`, thinking off); empty leaves it out |
| `APP_DATA_DIR` | server | Data directory override |
| `APP_CRISIS_RESOURCES` | server | Crisis lines the coach shares; see [Crisis resources](#crisis-resources) |
| `APP_URL` | CLI subcommands | Server base URL (an explicit `--server` flag wins) |
| `APP_BACKEND` | `web/` dev server | Backend `host:port` the Vite proxy targets |
| `APP_EVAL_JUDGE_BASE_URL` | `agilecbt eval` | OpenAI-compatible API for the rubric judge (default: the coach's) |
| `APP_EVAL_JUDGE_MODEL` | `agilecbt eval` | Judge model; rubric questions are skipped without one. See [Coach benchmarks](/guide/benchmarks) |
| `APP_EVAL_JUDGE_API_KEY` | `agilecbt eval` | Bearer token for the judge API |

See [AI coach & MCP](/guide/ai) for Ollama and OpenRouter setups.

The `claude-code`, `anthropic`, and `ollama` values of `llm` were removed.
The server refuses to start with them; use `base_url` instead (Ollama's is
`http://localhost:11434/v1`).

::: warning Reaching it from your phone
This app holds sensitive personal data. If `--addr` listens beyond localhost,
for example `:8080` in Docker or on a Tailscale address, set `APP_SECRET`. The
server logs a warning when it doesn't. Put it behind HTTPS, such as
`tailscale serve` or a reverse proxy, so the session cookie is marked `Secure`.
:::

## In-app settings

Settings opens as a dialog from the ⚙ in the header, with five tabs: Coach (the
default), AI model, Appearance, Data, and Support. The open tab is in the URL (for
example `/?settings=model`), and `/settings` opens the Coach tab.

The Coach tab's **Check-ins** setting (morning, evening, or both) sets which
check-in the Today screen offers. It's stored in the database, and can also be
set with `PATCH /api/settings`.

The **AI model** tab shows whether the coach's model is reachable. It lets you
turn the coach on or off and change the API URL, model, API key, and thinking
level. Values saved there override `config.toml` and the environment, and
take effect immediately. **Reset to config** clears them. A saved API key is
never shown again. It's only sent to the URL it was saved for, and it's left
out of exports. The same settings are available at
[`/api/llm`](/reference/api#coach-model).

The **Appearance** tab holds the look of the app. These choices are stored per
browser, not in the database:

- **Mode**: system, light, or dark.
- **Style**: *Warm* (the default: soft stone neutrals and rounded corners) or
  *Drafting* (charcoal and white neutrals, IBM Plex type, hairline rules,
  square corners, and condensed caps for headings and labels).
- **Accent**: the single colour used for buttons, focus rings, and progress:
  apricot (the default), signal red, sage, sky, or violet. Each accent has its
  own light and dark shades and works with either style. Drafting with signal
  red gives the high-contrast "engineering drawing" look.

The **Support** tab shows the crisis lines the coach shares, read-only (see
[Crisis resources](#crisis-resources)).
