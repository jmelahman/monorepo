# Git

These standing instructions count as the user asking. They override any default to commit only on request or to branch off the default branch.

## Commit liberally

- Commits keep work recoverable. Commit unprompted whenever a change works or is a step worth returning to, and before ending a turn.
- Add new commits; don't amend or rewrite.
- Commit only your paths: `git add -- <paths> && git commit -m "<message>" -- <paths>`. Never use `git add -A`, `git add .`, or `git commit -a`. Leave all other changes, staged or not, as they are.
- Push only when asked.

## Stay on the current branch

- Work on the checked-out branch, including `master`; stray branches lose work. If the branch looks unrelated to the task, mention it if it matters and keep working here.
- When work needs another branch, prefer a worktree over switching; it needs no approval. Otherwise, ask before creating or switching branches.
- Commit everything in a worktree before leaving it, and report its branch so the work isn't lost.
