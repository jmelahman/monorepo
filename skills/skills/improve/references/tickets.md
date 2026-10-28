# Tickets — publish plans as kanban tickets

The last step of every planning invocation. The user's selection of findings is the go-ahead; a plan nobody selected never becomes a ticket.

The ticket is the deliverable. Each ticket gets its own worktree and agent, the ticket body is that executor's whole brief, and each ticket is judged on its own done criteria. The advisor's work ends when the tickets exist: the user starts them, and nothing here dispatches, reviews, or tracks them. `plans/` is the local staging copy the bodies were created from.

1. Preflight: `kanban board list` succeeds (it needs the kanban server, `http://localhost:7474` by default). If it fails, keep the plan files as the handoff and say why tickets were skipped.
2. Pick a board per plan from the `ID  SLUG  NAME` rows of `kanban board list`. In a monorepo there is usually one board per project: use the board of the project the plan changes, and the repo-wide board (e.g. `monorepo`) for a plan that spans projects or whose project has no board. Always pass `--board <slug>`: several boards can share one repository, and then the board can't be inferred from the working directory.
3. Per plan, non-interactively (giving `--title` is what makes it non-interactive; without it the command opens a terminal form and attaches to an agent):

   ```sh
   kanban ticket create --board <slug> --title "<short imperative title>" \
     --body "$(cat plans/NNN-<slug>.md)" --json
   ```

   - The body is the **entire plan file**, not a pointer to it. `plans/` is usually gitignored, so neither the file nor the index exists in the ticket's worktree. Before creating, reread the plan as a standalone document: nothing in it may require a file under `plans/`, and another plan is named by what it changes, not only by its number.
   - Create tickets in dependency order, so a plan that depends on another can name the prerequisite's ticket id in its body.
   - The title follows the repo's commit-subject style, since tickets tend to become squash-commit subjects; on a per-project board, drop the project prefix.
   - Do not pass `--attach` or `--harness`: creating the ticket must not start a session. The user starts each one when they want it run.
   - `--json` prints the created ticket; read `id` from it.
4. Record each ticket in the plan's Status block (`- **Ticket**: kanban board <slug>, ticket #<id>`) and in a Ticket column of the index.
5. In the final report, list ticket ids by board and repeat the index's "Files touched by more than one plan" table: tickets run in separate worktrees, so the ones that overlap should be started one after another, not together.

If a plan is revised before its ticket is started, push the new text with `kanban ticket update <id> --body "$(cat <plan file>)"`. Once a ticket is started, it belongs to its executor.
