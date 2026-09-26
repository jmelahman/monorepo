# Introduction

Agentic Kanban is a kanban board where each ticket is worked on by an AI agent. It runs as a single binary that serves a web UI, a REST API, and an MCP server, and it stores everything in a local SQLite database.

## How it works

You create a board for a git repository. When you start a session on one of its tickets, kanban:

1. Creates a git worktree on a new branch off the board's base branch.
2. Starts a container for that worktree. It uses the repo's `.devcontainer/devcontainer.json` if there is one, and a bundled image if not.
3. Launches the agent (Claude Code by default) inside the container. You interact with it from the ticket in your browser.
4. Forwards dev-server ports from the container to host ports 13000-13099 so you can open the app the agent is building.

When the work looks good, you sync the branch with its base and merge it, both from the ticket.

## Concepts

**Board**
:   A workspace for one git repository. Its columns default to `Backlog`, `In Progress`, `Review`, and `Done`.

**Ticket**
:   A unit of work with a title and a markdown description. A started ticket has one session.

**Session**
:   The container, worktree, and agent working on a ticket.

**Harness**
:   The agent program that runs in the session, such as Claude Code or pi.dev. Set the default in the app's settings. You can override it for a single ticket.

**Worktree**
:   The ticket's checkout of the repo, stored under `<data-dir>/worktrees/`. It's mounted into the container, so the agent's edits show up on your machine too. Deleting the ticket deletes the worktree.

**Devcontainer**
:   The container a session runs in. You can add mounts, `docker run` arguments, and environment variables per repo in [`.kanban.toml`](./configuration).

::: tip Removing a worktree by hand
Kanban locks its worktrees so that a `git worktree prune` run inside a container can't delete them. To remove one yourself, run `git worktree remove -f -f <path>`.
:::

## Where data lives

| Path                                | Contents                              |
| ----------------------------------- | ------------------------------------- |
| `$KANBAN_DATA_DIR` or XDG data dir  | SQLite database and worktrees         |
| `$KANBAN_CONFIG` or XDG config dir  | Your user-level `config.toml`         |
| `<repo>/.kanban.toml`               | Project config, checked into the repo |
| `<data-dir>/worktrees/<board>/<id>` | Per-ticket worktrees                  |

## Next steps

- [Install](./install) kanban.
- Follow the [Quickstart](./quickstart) to create your first board and ticket.
- Read [Configuration](./configuration) to set per-repo policy.
