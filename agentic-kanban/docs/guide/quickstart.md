# Quickstart

This walkthrough creates a board, adds a ticket, and starts an agent session — end to end in a few minutes.

## 1. Open the UI

With `kanban serve` running, open <http://localhost:7474/>.

You'll land on the boards list. It's empty — there's nothing here yet.

<img class="light-only" src="/quickstart-01-empty-light.png" alt="Empty boards list" />
<img class="dark-only" src="/quickstart-01-empty-dark.png" alt="Empty boards list" />

## 2. Create a board

Click **New board**. Give it a name (e.g. `playground`) and point it at a local git repository on your machine. Kanban will read that repo's `.devcontainer/devcontainer.json` to figure out how to launch sessions for tickets.

<img class="light-only" src="/quickstart-02-create-board-light.png" alt="Create board dialog" />
<img class="dark-only" src="/quickstart-02-create-board-dark.png" alt="Create board dialog" />

The board opens with the default columns: `Backlog`, `In Progress`, `In Review`, `Done`.

<img class="light-only" src="/quickstart-03-empty-board-light.png" alt="Empty board" />
<img class="dark-only" src="/quickstart-03-empty-board-dark.png" alt="Empty board" />

## 3. Create a ticket

Click **+** in the `Backlog` column. Title the ticket and write a markdown body describing what you want the agent to do.

<img class="light-only" src="/quickstart-04-create-ticket-light.png" alt="Create ticket" />
<img class="dark-only" src="/quickstart-04-create-ticket-dark.png" alt="Create ticket" />

When you save, the ticket appears as a card in `Backlog`.

## 4. Start the session

Open the ticket. The detail panel has tabs — **agent**, **terminal**, **tasks**, **info**, plus a **diff** tab once the session has a worktree. Click **Start session**. Kanban will:

1. Create a git worktree off your base branch.
2. Spawn the worktree's devcontainer.
3. Launch the configured harness (Claude Code by default) inside it, attached to a PTY.

The harness's terminal streams into the **agent** tab; the **terminal** tab gives you a regular shell inside the same container.

## 5. Review the changes

The **diff** tab shows the session's changes as a GitHub-style split diff: a
changed-files sidebar, per-file **View file** and **Viewed** toggles, and
expandable context.

You can also leave a code review for the agent. Hover a line and click the
**+** in the gutter to comment on it, or drag across the line numbers to select
a range (the selected lines highlight as you drag) and comment on the whole
span. Comments are saved per-session in your browser, so they survive tab
switches and reloads. When you're done, the **Copy review** button in the
sidebar header copies every comment — each with its file path, line range, and
the referenced code — as one block you can paste straight back into the agent
session for it to address.

## 6. Sync and merge

When the agent's done, the **Sync** menu offers to rebase or merge your base branch into the worktree (whichever you've allowed in `.kanban.toml`). The **Merge** menu sends the work back to the base branch via merge commit, squash, or rebase — again, configurable.

## What's next

- Edit `.kanban.toml` in your repo to lock in policy — see [Configuration](./configuration).
- Drive ticket creation from outside the UI — see the [REST API](/reference/api) and [CLI](/reference/cli) reference.
- Wire up Claude Desktop or Claude Code to create tickets via [MCP](/reference/mcp).
