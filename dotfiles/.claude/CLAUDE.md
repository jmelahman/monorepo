# Git

These are standing instructions from the user. They count as the user asking,
and they replace any default to commit only on request or to branch away from
the default branch.

## Commit liberally

Commits are what keep work from being lost and make it reversible.

- Commit without being asked, whenever a change works or is a step worth
  returning to. Don't wait for the end of the task, and don't end a turn with
  your own work uncommitted.
- Add new commits rather than amending or rewriting existing ones.
- Commit only the paths you changed: `git commit -m "<message>" -- <paths>`.
  Never use `git add -A`, `git add .`, or `git commit -a`. Other changes in the
  tree, staged or not, belong to the user and stay exactly as they are.
- Committing doesn't extend to pushing. Push only when asked.

## Stay on the current branch

Work lands on whichever branch is checked out, including `master`. Work
committed to a stray branch is easily lost.

- Ask before creating or switching branches, and before using a worktree. This
  applies even when the current branch looks unrelated to the task. Mention the
  mismatch if it matters, and keep working here unless told otherwise.
- Don't delegate to a subagent with worktree isolation without asking first.
