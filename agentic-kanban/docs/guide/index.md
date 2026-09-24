# Introduction

Agentic Kanban is a kanban board for managing AI agent sessions. It's a single Go binary (also distributed as a Docker image and a Python wrapper) that hosts a web UI, a small REST API, and an MCP server — all backed by a local SQLite database.

## What it does

You point Agentic Kanban at a target repository. It reads that repo's existing devcontainer definition. When you create a ticket and start its session, kanban:

1. Creates a fresh git **worktree** off the chosen base branch.
2. Spawns the worktree's devcontainer in Docker — using the repo's `.devcontainer/devcontainer.json` if it has one, or the **bundled fallback devcontainer** that ships with kanban if it doesn't.
3. Starts the configured **harness** (Claude Code, pi.dev, …) inside that container, attached to a PTY you can drive from the web UI.
4. Optionally proxies dev-server ports from the container to host ports `13000–13099` so you can browse the WIP app.

When you're done with the work the agent did, you sync (rebase or merge the base branch in), then merge — all from the ticket panel.

## Concepts

**Board**
:   A long-lived workspace tied to a single git repository. Boards have configurable columns (defaults: `Backlog`, `In Progress`, `In Review`, `Done`).

**Ticket**
:   A unit of work on a board. A ticket lives in a column, has a markdown body, and (when started) owns exactly one session.

**Session**
:   The running container + worktree + harness for a ticket. Sessions have lifecycle states (`pending`, `running`, `stopped`, …) and emit hooks at state transitions.

**Harness**
:   The agent runner inside the container. The default is set in the app's settings (or `[harness].id` in [configuration](./configuration)), and a single ticket's session can use a different one via `kanban ticket create` / `kanban ticket attach`. Supports Claude Code and pi.dev today.

**Worktree**
:   A git worktree in `<data-dir>/worktrees/` (or the directory you set with `--worktrees-dir`). The session container bind-mounts the worktree, so file changes the agent makes are visible on the host. Kanban locks every worktree it creates (`git worktree lock`) because the container sees it at a different path than the host does, and an unlocked worktree would be deleted by `git worktree prune` run from inside the container. Deleting the ticket removes the worktree. To remove one by hand, run `git worktree remove -f -f <path>` or unlock it first.

**Devcontainer**
:   The container the session runs inside. Kanban reuses the target repo's `.devcontainer/devcontainer.json` when present, and otherwise falls back to a **bundled devcontainer image** (`lahmanja/kanban-devcontainer`, Ubuntu 24.04 + git, gh, node, Go) — pinned to the kanban release by digest at build time, so a fresh install works against any repo with zero setup. You can layer extra `mounts`, `runArgs`, and env vars per-repo via `.kanban.toml`.

## Where data lives

| Path                                   | What                                              |
|----------------------------------------|---------------------------------------------------|
| `$KANBAN_DATA_DIR` or XDG share dir    | SQLite database, worktrees                        |
| `$KANBAN_CONFIG` or XDG config dir     | User-level `config.toml`                          |
| `<repo>/.kanban.toml`                  | Project-level config, checked into the repo       |
| `<data-dir>/worktrees/<board>/<id>`    | Per-ticket git worktrees                          |

## Next steps

- [Install](./install) — uv, Docker, GitHub Releases, or build from source.
- [Quickstart](./quickstart) — start the server, create your first board and ticket.
- [Configuration](./configuration) — `.kanban.toml` schema and merge rules.
