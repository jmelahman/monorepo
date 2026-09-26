# Quickstart

This walks through creating a board and a ticket, then handing the ticket to an agent.

## 1. Open the UI

Start the server with `kanban serve` and open <http://localhost:7474/>. You'll see an empty list of boards.

<img class="light-only" src="/quickstart-01-empty-light.png" alt="Empty boards list" />
<img class="dark-only" src="/quickstart-01-empty-dark.png" alt="Empty boards list" />

## 2. Create a board

Click **New board**. Give it a name and the path to a git repository on your machine.

<img class="light-only" src="/quickstart-02-create-board-light.png" alt="Create board dialog" />
<img class="dark-only" src="/quickstart-02-create-board-dark.png" alt="Create board dialog" />

The board opens with four columns: `Backlog`, `In Progress`, `Review`, and `Done`.

<img class="light-only" src="/quickstart-03-empty-board-light.png" alt="Empty board" />
<img class="dark-only" src="/quickstart-03-empty-board-dark.png" alt="Empty board" />

## 3. Create a ticket

Click **+** on the `Backlog` column. Give the ticket a title and describe the task in markdown. The description is what the agent works from.

<img class="light-only" src="/quickstart-04-create-ticket-light.png" alt="Create ticket" />
<img class="dark-only" src="/quickstart-04-create-ticket-dark.png" alt="Create ticket" />

## 4. Start a session

Open the ticket and click **Start session**. Kanban creates a worktree, starts its container, and launches the agent. The first start pulls or builds the container image, so it can take a few minutes.

The ticket has several tabs:

- **agent**: the agent's terminal.
- **terminal**: a plain shell in the same container.
- **tasks**: run tasks from the repo's `.vscode/tasks.json`.
- **diff**: everything the agent has changed.
- **info**: the branch, container, pull request, and ports.

## 5. Review the changes

The **diff** tab shows the branch's changes side by side, with a list of changed files.

You can leave review comments for the agent. Click the **+** next to a line, or drag across line numbers to comment on a range. When you're done, click **Copy review** and paste the result into the agent's terminal. Each comment includes its file, lines, and code, so the agent knows what you mean.

## 6. Sync and merge

**Sync** brings the base branch into the ticket's branch, by rebase or merge. **Merge** lands the ticket's branch on the base branch as a merge commit, a squash, or a rebase. You can limit which options appear in [`.kanban.toml`](./configuration).

## Next steps

- Set per-repo policy in [`.kanban.toml`](./configuration).
- Create and manage tickets from the [CLI](/reference/cli) or the [REST API](/reference/api).
- Let Claude create tickets for you over [MCP](/reference/mcp).
