# Agentic Kanban

[![Test status](https://github.com/jmelahman/agentic-kanban/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/agentic-kanban/actions)
[![Deploy Status](https://github.com/jmelahman/agentic-kanban/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/agentic-kanban/actions)
[![Docs](https://github.com/jmelahman/agentic-kanban/actions/workflows/docs.yml/badge.svg)](https://jmelahman.github.io/agentic-kanban/)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/agentic-kanban.svg)](https://pkg.go.dev/github.com/jmelahman/agentic-kanban)
[![PyPI](https://img.shields.io/pypi/v/agentic-kanban.svg)](https://pypi.org/project/agentic-kanban/)

A kanban board for managing AI agent sessions.

<p align="center">
  <picture align="center">
    <source media="(prefers-color-scheme: dark)" srcset="https://github.com/jmelahman/agentic-kanban/blob/master/assets/demo_dark.png">
    <source media="(prefers-color-scheme: light)" srcset="https://github.com/jmelahman/agentic-kanban/blob/master/assets/demo_light.png">
    <img src="https://github.com/jmelahman/agentic-kanban/blob/master/assets/demo_light.png">
  </picture>
</p>

Each ticket is bound to an agent session (Claude Code, pi.dev) running inside its own git worktree, executed in the target repository's existing devcontainer. The active harness is selected globally in the app's settings.

## Features

**Agent sessions**

- Every ticket runs an AI agent in its own git worktree, inside a Docker container — no contention, no rebasing pain.
- Reuses the target repo's `.devcontainer/devcontainer.json`, or falls back to a bundled Ubuntu devcontainer (git, gh, node, Go) so any repo works with zero setup.
- Pluggable harnesses (Claude Code, pi.dev), switchable globally in settings.
- Drive the agent from a full terminal in the browser, plus a second plain shell — both backed by a WASM terminal emulator.
- Claude Code conversations resume automatically across container and server restarts.

**Boards & tickets**

- Kanban boards tied to a git repo, with configurable columns and drag-and-drop tickets with markdown bodies.
- Sync (rebase/merge from base) and merge (merge-commit / squash / rebase) straight from the ticket panel.
- Optional AI-generated merge commit messages and per-board commit identity.
- Archive, unarchive, and delete tickets; archived drawer with bulk actions.
- Multi-board overview with tiling, resizable session panels for working several tickets at once.

**Diff, tasks & ports**

- Built-in diff viewer with a changed-files tree, syntax highlighting, whole-file view, and expandable context.
- Tasks tab discovers `.vscode/tasks.json` tasks, runs them, streams live output, and stops them.
- Per-task port forwarding proxies in-container dev servers to host ports `13000–13099` and surfaces them on the ticket.
- Plans tab renders the `.md` plans the agent writes (e.g. Claude Code's `/plan` mode).

**GitHub integration**

- Auto-moves tickets when their linked PR or issue changes state, with configurable column mappings.
- Live PR detail: diff totals, rolled-up review decision, and CI check status.
- **Build Cop** polls GitHub Actions and files tickets for jobs whose failure rate crosses a threshold, on its own auto-managed board (`Failing` / `Investigating` / `Fixed` / `Won't fix`).

**Interfaces & ops**

- Web UI with light / dark / high-contrast themes, accent colors, and remappable keyboard shortcuts.
- REST API with Server-Sent Events for live updates, an MCP server for Claude Desktop / Claude Code, and noun-verb CLI subcommands.
- Ships as a single self-contained ~20 MB Go binary — also distributed as a Docker image and a PyPI wrapper — backed by local SQLite.
- Prometheus `/metrics` and a `/health` endpoint; project + user `.kanban.toml` config that merge.

## Install

```sh
uv tool install agentic-kanban
```

This installs the binary to `~/.local/bin/kanban`.
Make sure to have that on your `PATH`.

## Run

```sh
kanban serve
```

The server listens on `:7474`. Open <http://localhost:7474/>.

Or with Docker:

```sh
SOURCE=$HOME/code
docker run -d --name kanban \
  --restart unless-stopped \
  -p 127.0.0.1:7474:7474 \
  -p 13000-13099:13000-13099 \
  -v ${DOCKER_SOCK_PATH:-/var/run/docker.sock}:/var/run/docker.sock \
  -v $HOME/.claude:$HOME/.claude \
  -v $HOME/.claude.json:$HOME/.claude.json \
  -v $HOME/.pi/agent:$HOME/.pi/agent \
  -v $HOME/.local/share/kanban:$HOME/.local/share/kanban \
  -v $HOME/.gitconfig:$HOME/.gitconfig:ro \
  -v $HOME/.config/git:$HOME/.config/git:ro \
  -v $SOURCE:$SOURCE \
  -e HOME=$HOME \
  -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
  -e KANBAN_DATA_DIR=$HOME/.local/share/kanban \
  -e KANBAN_HOST_DOCKER_SOCK=${DOCKER_SOCK_PATH:-/var/run/docker.sock} \
  -e GH_TOKEN=$(gh auth token) \
  lahmanja/kanban:latest
```

## 📖 Documentation

Full documentation lives at **<https://jmelahman.github.io/agentic-kanban/>**:

- [Quickstart](https://jmelahman.github.io/agentic-kanban/guide/quickstart) — your first board and ticket in five minutes.
- [Configuration](https://jmelahman.github.io/agentic-kanban/guide/configuration) — `.kanban.toml` schema and merge rules.
- [REST API](https://jmelahman.github.io/agentic-kanban/reference/api) — endpoints exposed by the running server.
- [CLI](https://jmelahman.github.io/agentic-kanban/reference/cli) — `serve`, `mcp`, `board list`, `ticket create`.
- [MCP](https://jmelahman.github.io/agentic-kanban/reference/mcp) — Claude Desktop / Claude Code integration.
