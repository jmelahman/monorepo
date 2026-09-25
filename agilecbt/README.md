# AgileCBT

A personal, self-hosted app for living with depression and anxiety. It pairs
the rhythm of agile teams (roadmap, weekly sprints, daily standups, retros)
with tools from cognitive behavioral therapy:

- **Today**: a morning/evening check-in (mood, energy, anxiety). An optional
  AI curator then helps you pick one to three steps that fit your energy.
- **Board**: Someday → This week → Today → Done, with steps sized by energy
  cost. Steps are carried over or let go, never "overdue". Finishing one asks
  for mastery and pleasure ratings (behavioral activation).
- **Roadmap**: values → goals → steps, so every step traces back to why it
  matters.
- **Thoughts**: guided thought records with plain-language thinking traps.
- **Retro**: a weekly look back, optionally drafted by the curator, with a mood
  trend chart.

The AI curator runs on Claude Code (your Claude subscription), a local Ollama
model, or the Claude API (`APP_LLM`). The same tools are exposed over MCP at
`/mcp`. Every AI change is logged and can be undone.

> AgileCBT is a self-help tool, not a substitute for professional care. The
> "Need help now?" button lists crisis lines; edit them in Settings.

## Run

Needs Go 1.26+ and [Bun](https://bun.sh).

```sh
bun install --cwd web --frozen-lockfile && bun run --cwd web build
go build -tags embed -o agilecbt .
APP_SECRET='something long' ./agilecbt serve
```

Open <http://localhost:8080/>. See `docs/guide/` for
[configuration](docs/guide/configuration.md), [the AI curator and
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
fake Ollama (`web/tests/e2e/fake-ollama.mjs`). See `CLAUDE.md` for the full
development reference.

## License

GPL-3.0; see [LICENSE](LICENSE).
