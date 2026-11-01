# Releases

The **Releases** tab is for monorepos that publish subtrees with [git-orchard](https://github.com/jmelahman/git-orchard). It keeps notes on what you meant to release, shows each subtree's state, and releases through the same code as `git orchard release`.

A repo appears here when a board uses it and it has a git-orchard manifest.

## Reminders

A reminder is a note against a subtree, with the bump you intend: "release a minor of git-orchard, it has the new X". It doesn't watch tickets, PRs or commits, and it never fires by itself. You decide when the change is in.

Add one with **remind me later** in a subtree's release dialog. Each subtree has at most one open reminder, so a second thought goes in the same note, with **edit**. A successful release closes it.

## Subtree state

Each subtree shows how far it is ahead of and behind its upstream, its latest release, and the commits since. This is recomputed at startup, every 30 minutes and when you refresh, because checking every subtree can take a while. The last result is saved, so after a restart the tab shows it right away while the refresh runs.

Subtrees that have never been released are hidden until you tick **show unreleased**. Your browser remembers the choice.

It also shows each subtree's latest GitHub Actions runs on its upstream, checked at most every 5 minutes while something is reading the page or `kanban release list`, so an idle server spends none of your API quota. This is for information only: a release doesn't check CI. It needs `gh` on the server, signed in.

## Assessing unreleased commits

The tab can also estimate how likely users are to notice each subtree's unreleased commits, using a [Kev](https://github.com/jaredpalmer/kev) decision model. This is off by default, and nothing is loaded unless you turn it on.

To turn it on, point the server at a Kev checkpoint:

```sh
kanban serve --kev-checkpoint /path/to/kev-0.8b --kev-base /path/to/qwen3.5-0.8b-base
```

The checkpoint needs a `head.safetensors`. Create one with [`tools/convert_head.py`](https://github.com/jmelahman/kev-go/blob/master/tools/convert_head.py). `--kev-base` is the backbone an adapter was trained on, and a full-weight checkpoint doesn't need it.

A **user-visible** column then shows the estimate as a percentage. It's for information only: a release doesn't check it, and it doesn't tell you whether to release. Against one monorepo's release history, Kev-0.8B couldn't predict when a release happened, and its suggested bumps were worse than always choosing patch. It did separate clearly internal commits from user-facing ones, but read the percentage as a hint.

The model reads only the subjects of the newest 60 commits since the latest release, and runs on the server, so nothing is sent elsewhere. Subtrees with no commits since a release aren't assessed, and neither are those never released.

The model loads only when the page or `kanban release list` reads the state and there are commits it hasn't read, and it's closed as soon as it has read them. Kev-0.8B takes about 4 GB of memory and a few seconds per subtree on a 12-core CPU while it runs. Answers are saved, so refreshes and restarts don't load it again for the same commits. Pointing at a different checkpoint path assesses everything again; replacing the files at the same path doesn't.

## Releasing

**release** fetches, then shows the version and commit that would be released: the tip of the board's base branch on `origin`. Your checkout isn't touched.

If the upstream lacks commits from that branch, the mirror hasn't published them yet, and the release is blocked until it has. Confirming sends back what you saw, so if the branch has moved on, the release is refused rather than tagging something you didn't review.

The release pushes the tag `<prefix>/<version>` to `origin`, then follows the GitHub Actions runs that publish it. Pre-push hooks don't run, since only a tag is pushed, for commits already on the base branch.

A failed or interrupted release can be run again. A tag it left behind is kept and published again.

## GitHub access

Where the server has a token (`GH_TOKEN`, `GITHUB_TOKEN`, or `gh auth token`), git uses it for GitHub remotes through its environment, so it isn't written to disk. Without one, git uses whatever ssh access the server has.

The tagger is the board's git author, falling back to the git identity of the repo.

## From the terminal

[`kanban release`](/reference/cli#release) does the same from a terminal: `kanban release remind tools/foo --minor "has the new flag"` right after a change, and `kanban release` for a view of your reminders.
