---
name: dependabot
description: >
  Triages and lands open Dependabot PRs in jmelahman/monorepo: merges green
  ones, fixes known mechanical CI failures, separates real regressions from
  breakage already on master, and checks the subtree mirrors afterwards. Use
  when asked to merge, clean up, clear out, or land Dependabot PRs or
  dependency bumps in the monorepo.
model: sonnet
metadata:
  author: jmelahman
  version: "2.4"
---

# Dependabot PRs

GitHub does not gate `master`: no required checks, no required reviews. A red
PR merges as readily as a green one, so **this skill is the gate**. Merge
only what the script puts under `MERGE NOW`.

## Rules

- Merge with `gh pr merge <pr> --squash`. No other method is enabled. Don't
  approve and don't pass `--auto`.
- Do without asking: merge `MERGE NOW` PRs, comment `@dependabot rebase`.
- Ask first, once, with `AskUserQuestion`, everything in one question set:
  merging an `ASK FIRST` PR, pushing any commit (to a PR branch or to
  `master`), closing a PR.
- Never merge a PR that is red, pending or conflicting.

## 1. Triage

```bash
${CLAUDE_SKILL_DIR}/scripts/triage.py
```

Read-only. It prints one row per PR, then the plan. Follow the plan as
printed; don't re-derive it.

| Bucket | Meaning | Do |
|---|---|---|
| `MERGE NOW` | green patch or minor bump | step 2 |
| `NEXT WAVE` | green, but shares a file with a `MERGE NOW` PR | step 2 |
| `ASK FIRST` | green, but major, 0.x minor, or no version found | ask, then step 2 |
| `RED` | check failed; the row names the hook | step 3 |
| `CONFLICT` | does not merge cleanly | step 4 |
| `PENDING` | check still running | `gh pr checks <pr> --watch`, then rerun the script |

For an `ASK FIRST` PR, give the user one line per bump: the version change
and any breaking change from the release notes in the PR body. If the jump is
`unknown`, get the change from `gh pr diff <pr>`.

## 2. Merge

1. `gh pr merge <pr> --squash` each `MERGE NOW` PR.
2. Comment `@dependabot rebase` on each `NEXT WAVE` PR.
3. `gh pr checks <pr> --watch` on those, then rerun the script.
4. Repeat until `MERGE NOW` and `NEXT WAVE` are both `-`.

## 3. Red

Get the failing hook's output. Don't read the raw run log.

```bash
${CLAUDE_SKILL_DIR}/scripts/triage.py --log <pr>
```

Then pick the first that applies:

1. The log shows a network, registry or rate-limit error: rerun once with
   `gh run rerun <runId> --failed` (`<runId>` is in the URL printed by
   `gh pr checks <pr>`). If it fails the same way again, treat it as real.
2. Otherwise, run the same hook on a checkout of `origin/master`:

   ```bash
   prek run <hook-id> --all-files
   ```

   - It passes: the bump broke it. Leave the PR open and report the hook and
     the error. Don't edit source code to make it pass.
   - It fails the same way: the PR didn't cause it. If the hook is
     `bun-audit`, run `bun audit fix` in the `web/` directory the output
     names, check the diff is `bun.lock` only, commit as
     `` `bun audit fix` ``, push to `master`, then comment
     `@dependabot rebase` on the red PRs. For any other hook, leave the PR
     open and report it.

## 4. Conflict

- Branch has only Dependabot's commits: comment `@dependabot rebase`.
- Branch has a fix we pushed: rebase it onto `origin/master` yourself,
  regenerate the lockfile with `bun install` in the bumped directory instead
  of hand-merging it, and push with `--force-with-lease`.

## Pushing a fix

The main checkout usually has work in progress, so don't use it. Create a
worktree with `EnterWorktree` (not under `/tmp`), then:

```bash
gh pr checkout <pr>
# make the change
prek run --from-ref origin/master --to-ref HEAD
git commit -am "<what changed>" && git push
```

The `prek` line is the PR check. It must pass before pushing.

## 5. Finish

```bash
${CLAUDE_SKILL_DIR}/scripts/triage.py --master
```

- `Mirror` must be `success` on the last merged commit. If it failed, report
  it; don't fix it.
- `Tests` should be `success`. If not, report the failing hook.
- `Devcontainer` appears only if a Dockerfile it builds changed; it must be
  `success` too.

Then report:

```
Merged (N): #A, #B
Fixed: <commit or PR> — <what>
Left for you (N): #C — <specific reason>
master: Mirror <result>, Tests <result>
```
