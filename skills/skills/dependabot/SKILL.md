---
name: dependabot
description: >
  Triages and lands a GitHub repository's open Dependabot PRs: merges green
  ones in conflict-free waves, fixes known mechanical CI failures, separates
  real regressions from breakage already on the default branch, and checks the
  default branch's workflows afterwards. Use when asked to merge, clean up,
  clear out, or land Dependabot PRs or dependency bumps.
model: sonnet
metadata:
  author: jmelahman
  version: "3.0"
---

# Dependabot PRs

Don't count on branch protection to stop a bad merge: many repositories have
no required checks, and a red PR merges as readily as a green one. **This
skill is the gate.** Merge only what the script puts under `MERGE NOW`.

If the repository's `CLAUDE.md` or `AGENTS.md` has notes on dependency
updates (a fix that is safe to push, a workflow to check after merging),
they add to the rules below.

## Rules

- Merge with the command the script prints after `MERGE WITH`. It picks a
  merge method the repository allows. Don't approve and don't pass `--auto`.
- Do without asking: merge `MERGE NOW` PRs, comment `@dependabot rebase`.
- Ask first, once, with `AskUserQuestion`, everything in one question set:
  merging an `ASK FIRST` PR, pushing any commit (to a PR branch or to the
  default branch), closing a PR.
- Never merge a PR that is red, pending or conflicting.

## 1. Triage

```bash
${CLAUDE_SKILL_DIR}/scripts/triage.py
```

Run it from the repository's checkout; it asks `gh` about the current
repository. Read-only. It prints one row per PR, then the plan. Follow the
plan as printed; don't re-derive it.

| Bucket | Meaning | Do |
|---|---|---|
| `MERGE NOW` | green patch, minor or digest-only bump (same tag, new sha256) | step 2 |
| `NEXT WAVE` | green, but shares a file with a `MERGE NOW` PR | step 2 |
| `ASK FIRST` | green, but major, 0.x minor, or no version found | ask, then step 2 |
| `RED` | check failed; the row names the hook, if it was a prek or pre-commit hook | step 3 |
| `CONFLICT` | does not merge cleanly | step 4 |
| `PENDING` | check still running | `gh pr checks <pr> --watch`, then rerun the script |

For an `ASK FIRST` PR, give the user one line per bump: the version change
and any breaking change from the release notes in the PR body. If the jump is
`unknown`, get the change from `gh pr diff <pr>`.

## 2. Merge

1. Merge each `MERGE NOW` PR with the `MERGE WITH` command.
2. Comment `@dependabot rebase` on each `NEXT WAVE` PR.
3. `gh pr checks <pr> --watch` on those, then rerun the script.
4. Repeat until `MERGE NOW` and `NEXT WAVE` are both `-`.

## 3. Red

Get the failing check's output. Don't read the raw run log.

```bash
${CLAUDE_SKILL_DIR}/scripts/triage.py --log <pr>
```

Then pick the first that applies:

1. The log shows a network, registry or rate-limit error: rerun once with
   `gh run rerun <runId> --failed` (`<runId>` is in the URL printed by
   `gh pr checks <pr>`). If it fails the same way again, treat it as real.
2. Otherwise, run the same check on a checkout of the default branch: the
   hook (`prek run <hook-id> --all-files`, or `pre-commit run <hook-id>
   --all-files`), or the step's command from the workflow file.

   - It passes: the bump broke it. Leave the PR open and report the check and
     the error. Don't edit source code to make it pass.
   - It fails the same way: the PR didn't cause it. If it is a dependency
     audit (`bun audit`, `npm audit`, `pnpm audit`) and the ecosystem has a
     fix command (`bun audit fix`, `npm audit fix`), run it in the directory
     the output names and check the diff touches the lockfile only. Then,
     after asking, commit it with the command as the subject (e.g.
     `` `bun audit fix` ``), push to the default branch, and comment
     `@dependabot rebase` on the red PRs. For any other check, leave the PR
     open and report it.

## 4. Conflict

- Branch has only Dependabot's commits: comment `@dependabot rebase`.
- Branch has a fix we pushed: rebase it onto the default branch yourself,
  regenerate the lockfile with the package manager (`bun install`,
  `npm install`, `go mod tidy`, ...) in the bumped directory instead of
  hand-merging it, and push with `--force-with-lease`.

## Pushing a fix

The main checkout often has work in progress, so don't use it. Create a
worktree with `EnterWorktree` (not under `/tmp`), then:

```bash
gh pr checkout <pr>
# make the change
# run the repository's PR checks on the change, e.g.
#   prek run --from-ref origin/<default-branch> --to-ref HEAD
git commit -am "<what changed>" && git push
```

The checks must pass before pushing.

## 5. Finish

Wait for the runs on the last merged commit. Run this with a 10-minute
timeout:

```bash
${CLAUDE_SKILL_DIR}/scripts/triage.py --base --wait
```

It prints one row per workflow on the default branch's last commit: its
result and run id. `not triggered` is fine; many workflows only run when
certain files change.

If a row is `failure`:

1. Rerun it once: `gh run rerun <run id> --failed`, then run the command
   above again.
2. Still `failure`: get the output with
   `${CLAUDE_SKILL_DIR}/scripts/triage.py --log base` and report it. Don't
   investigate, fix or revert anything.

Then report:

```
Merged (N): #A, #B
Fixed: <commit or PR> — <what>
Left for you (N): #C — <specific reason>
<default branch>: <workflow> <result>, … — <failing check, and whether the rerun passed>
```
