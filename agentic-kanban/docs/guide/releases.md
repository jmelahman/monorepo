# Releases

The **Releases** tab is for monorepos that publish subtrees with [git-orchard](https://github.com/jmelahman/git-orchard). It keeps notes on what you meant to release, shows each subtree's state, and releases through the same code as `git orchard release`.

A repo appears here when a board uses it and it has a git-orchard manifest.

## Reminders

A reminder is a note against a subtree, with the bump you intend: "release a minor of git-orchard, it has the new X". It doesn't watch tickets, PRs or commits, and it never fires by itself. You decide when the change is in.

Each subtree has at most one open reminder, so a second thought goes in the same note. A successful release closes it.

## Subtree state

Each subtree shows how far it is ahead of and behind its upstream, its latest release, and the commits since. This is recomputed every 30 minutes and when you refresh, because checking every subtree can take a while.

It also shows each subtree's latest GitHub Actions runs on its upstream, checked at most every 5 minutes while something is reading the page or `kanban release list`, so an idle server spends none of your API quota. This is for information only: a release doesn't check CI. It needs `gh` on the server, signed in.

## Releasing

**Release...** fetches, then shows the version and commit that would be released: the tip of the board's base branch on `origin`. Your checkout isn't touched.

If the upstream lacks commits from that branch, the mirror hasn't published them yet, and the release is blocked until it has. Confirming sends back what you saw, so if the branch has moved on, the release is refused rather than tagging something you didn't review.

The release pushes the tag `<prefix>/<version>` to `origin`, then follows the GitHub Actions runs that publish it. Pre-push hooks don't run, since only a tag is pushed, for commits already on the base branch.

A failed or interrupted release can be run again. A tag it left behind is kept and published again.

## GitHub access

Where the server has a token (`GH_TOKEN`, `GITHUB_TOKEN`, or `gh auth token`), git uses it for GitHub remotes through its environment, so it isn't written to disk. Without one, git uses whatever ssh access the server has.

The tagger is the board's git author, falling back to the git identity of the repo.

## From the terminal

[`kanban release`](/reference/cli#release) does the same from a terminal: `kanban release remind tools/foo --minor "has the new flag"` right after a change, and `kanban release` for a view of your reminders.
