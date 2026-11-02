# Releases

The **Releases** tab is for monorepos that publish subtrees with [git-orchard](https://github.com/jmelahman/git-orchard). It keeps notes on what you meant to release, shows each subtree's state, and releases through the same code as `git orchard release`.

A repo appears here when a board uses it and it has a git-orchard manifest.

## Reminders

A reminder is a note against a subtree, with the bump you intend: "release a minor of git-orchard, it has the new X". It doesn't watch tickets, PRs or commits, and it never fires by itself. You decide when the change is in.

Add one with **remind me later** in a subtree's release dialog. Each subtree has at most one open reminder, so a second thought goes in the same note, with **edit**. A successful release closes it.

A release made elsewhere, such as with `git orchard release`, can't tell which reminder it covered. So once a subtree's state shows a release newer than a reminder's last edit, the reminder says so, since it may be done. Dismiss it, or edit it to keep it and clear the warning.

## Subtree state

Each subtree shows how far it is ahead of and behind its upstream, its latest release, and the commits since. This is recomputed at startup, every 30 minutes and when you refresh, because checking every subtree can take a while. The last result is saved, so after a restart the tab shows it right away while the refresh runs.

**filter** picks which subtrees to list, repo by repo. Subtrees you haven't toggled are listed once they've been released, so a new one turns up by itself after its first release. **view all** lists every subtree whatever the filter says. Your browser remembers both, and neither hides reminders.

It also shows each subtree's latest GitHub Actions runs on its upstream, checked at most every 5 minutes while something is reading the page or `kanban release list`, so an idle server spends none of your API quota. This is for information only: a release doesn't check CI. It needs a GitHub token on the server (`GH_TOKEN`, `GITHUB_TOKEN`, or `gh auth token`).

## Suggested bumps

The **suggested** column reads the titles of each subtree's commits since its latest release as [Conventional Commits](https://www.conventionalcommits.org), and shows the largest bump any of them calls for:

| Title | Suggests |
| --- | --- |
| `!` before the colon, on any type: `feat!: ...` | major |
| `feat: ...` | minor |
| `fix: ...`, `perf: ...`, `revert: ...` | patch |
| `docs`, `style`, `refactor`, `test`, `build`, `ci`, `chore` | none |

Any other title is skipped, so commits that don't follow the convention can leave the suggestion too low. Merge commits are ignored, and subtrees never released have no suggestion.

A subtree's release dialog starts at the suggestion, or at patch when it's none or there isn't one, and you can change it there. A release from a reminder or a retry starts at the bump it was given instead, and a release doesn't check the suggestion.

## Releasing

**release** fetches, then shows the version and commit that would be released: the tip of the board's base branch on `origin`. Your checkout isn't touched.

If the upstream lacks commits from that branch, the mirror hasn't published them yet, and the release waits until it has, unless you tick the option to push them to the upstream now (`--upstream` on `kanban release run`). That pushes the subtree's branch and tag there directly, so the server's credentials need write access to the upstream. Confirming sends back what you saw, so if the branch has moved on, the release is refused rather than tagging something you didn't review.

The release pushes the tag `<prefix>/<version>` to `origin`, then follows the GitHub Actions runs that publish it. Pre-push hooks don't run, since only a tag is pushed, for commits already on the base branch.

Releases queue. Up to 4 per repo tag at once, the rest wait in order, and each follows what it publishes without holding up the next. A queued release keeps the version and commit you confirmed, even if the branch moves on while it waits. A subtree can't have a second release until its first finishes, because the first would make the version you confirmed out of date. Stopping the server interrupts whatever is queued.

A subtree's `pre-release` hook runs in a temporary worktree at the tip of the base branch, not in your checkout, so files ignored by git aren't there for it. Hooks go one at a time. If a hook changes files, the commit is pushed to the base branch before anything is published, then the release waits for the mirror to publish it, or pushes it upstream itself if you chose to. The hook's release fails if the subtree changed on the base branch after you confirmed, or if the branch rejects the push; either way nothing has been published, so release it again.

The **deployment** column shows the state of each subtree's latest release from here: queued, running, released or failed, or empty if it hasn't had one. Select it to read that release's log below the table, and to retry it if it failed.

A failed or interrupted release can be run again. A tag it left behind is kept and published again.

## GitHub access

Where the server has a token (`GH_TOKEN`, `GITHUB_TOKEN`, or `gh auth token`), git uses it for GitHub remotes through its environment, so it isn't written to disk. Without one, git uses whatever ssh access the server has.

The tagger is the board's git author, falling back to the git identity of the repo.

## From the terminal

[`kanban release`](/reference/cli#release) does the same from a terminal: `kanban release remind tools/foo --minor "has the new flag"` right after a change, and `kanban release` for a view of your reminders.
