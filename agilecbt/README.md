# AgileCBT

A personal, self-hosted app for living with depression and anxiety. It pairs
the rhythm of agile teams (roadmap, weekly sprints, daily standups, retros)
with tools from cognitive behavioral therapy:

- **Today**: a morning/evening check-in (mood, energy, anxiety). An optional
  AI coach then helps you pick one to three steps that fit your energy.
- **Board**: Someday → This week → Today → Done, with steps sized by energy
  cost. Steps are carried over or let go, never "overdue". Finishing one asks
  for mastery and pleasure ratings (behavioral activation).
- **Roadmap**: values → goals → steps, so every step traces back to why it
  matters.
- **Thoughts**: guided thought records with plain-language thinking traps.
- **Retro**: a weekly look back, optionally drafted by the coach, with a mood
  trend chart.

The AI coach runs on any OpenAI-compatible API: a local Ollama model by
default, or a hosted one through OpenRouter. The same tools are exposed over
MCP at `/mcp`, so Claude Code and Claude Desktop can use them too. Every AI
change is logged and can be undone.

> AgileCBT is a self-help tool, not a substitute for professional care. The
> coach shares crisis lines if a conversation calls for it; they're also in
> Settings → Support. Set your own with `crisis_resources` in `config.toml`.

## Run

Needs Go 1.26+ and [Bun](https://bun.sh).

```sh
bun install --cwd web --frozen-lockfile && bun run --cwd web build
go build -tags embed -o agilecbt .
APP_SECRET='something long' ./agilecbt serve
```

Open <http://localhost:8080/>. See `docs/guide/` for
[configuration](docs/guide/configuration.md), [the AI coach and
MCP](docs/guide/ai.md), and phone access, and `docs/reference/` for the
[REST API](docs/reference/api.md) and [CLI](docs/reference/cli.md).

## Develop

```sh
# Backend on :8080 (wgo restarts on save; plain `go run` works too)
wgo run . serve --in-memory

# Frontend on :5173, proxying /api to the backend
cd web && bun install && bun run dev
```

Tests: `go test ./...` and `cd web && bun run typecheck && bun run check &&
bun run test:e2e`. The E2E suite boots the real backend against a scripted
fake OpenAI-compatible API (`web/tests/e2e/fake-llm.mjs`). See `CLAUDE.md` for the full
development reference.

## License

GPL-3.0; see [LICENSE](LICENSE).
