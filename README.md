# PKGBUILDs-template

[![Test status](https://github.com/OWNER/REPO/actions/workflows/test.yml/badge.svg)](https://github.com/OWNER/REPO/actions)

A template repository for maintaining a collection of Arch Linux
[PKGBUILDs](https://wiki.archlinux.org/title/PKGBUILD) and publishing them to
the [AUR](https://aur.archlinux.org/). Each package lives in its own directory
as a git [subtree](https://git-scm.com/book/en/v2/Git-Tools-Advanced-Merging#_subtree_merge)
of its AUR repository, so this repository is the single source of truth and the
AUR mirrors it.

Out of the box it gives you:

- **Linting** with `shellcheck` and [pkglint](https://github.com/jmelahman/pkglint)
  on every pull request, plus a test build of every changed package inside an
  `archlinux:base-devel` container.
- **Version bumps** via [nvchecker](https://github.com/lilydjwg/nvchecker):
  an hourly workflow checks every upstream, bumps `pkgver`, refreshes checksums
  and `.SRCINFO`, and opens an auto-merging pull request per package.
- **Deploys** to the AUR on every push to `master`, with a nightly fallback.
- Workflow hardening checks with [zizmor](https://github.com/zizmorcore/zizmor)
  and [actionlint](https://github.com/rhysd/actionlint).

## Setup

1. Click **Use this template** on GitHub and clone your new repository.
2. Replace `OWNER/REPO` in the badge above and update `LICENSE` with your name.
3. Add the repository secrets under _Settings → Secrets and variables → Actions_:

   | Secret                | Used by      | Purpose                                                                                                                                                                                                                    |
   | --------------------- | ------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
   | `AUR_SSH_PRIVATE_KEY` | `deploy.yml` | An SSH key whose public half is registered on your [AUR account](https://aur.archlinux.org/account/). Pushes each package subtree to the AUR.                                                                              |
   | `PAT`                 | `hourly.yml` | A fine-grained personal access token with **Contents: read/write** and **Pull requests: read/write** on this repository. Pushes bump branches and opens pull requests that trigger CI (the default `GITHUB_TOKEN` cannot). |

4. Create an environment named `pkgreleaser` (_Settings → Environments_). The
   hourly workflow runs in it so the token is scoped to that job.
5. Enable **Allow auto-merge** under _Settings → General → Pull Requests_ so
   version-bump PRs merge themselves once CI passes, and add a branch
   protection rule on `master` requiring the `Lint and build changed packages`
   check.

## Managing packages

### Adding a new package

For a package that already exists on the AUR:

```shell
git subtree add --prefix=$PACKAGE ssh://aur@aur.archlinux.org/$PACKAGE.git master
```

For a brand-new package, create `$PACKAGE/PKGBUILD` and generate its
`.SRCINFO`:

```shell
(cd $PACKAGE && makepkg --printsrcinfo > .SRCINFO)
```

The first deploy creates the AUR repository automatically.

Then add a section for the package to `nvchecker.toml` so it gets version
checks. `validate_configs.py` fails the pre-commit run if a package is missing
from it. If the upstream name differs from the AUR package name (e.g.
`python-foo` for the PyPI project `foo`), add the mapping to
`NVCHECKER_NAME_MAP` in `validate_configs.py` and `ENTRY_TO_UPSTREAM` in
`pkgreleaser.py`.

Verify the package builds:

```shell
prek run --stage manual --files $PACKAGE/PKGBUILD
```

The build runs `makepkg -s` on the host, so it asks for a sudo password only
when a dependency is missing. CI runs the same hook inside an
`archlinux:base-devel` container, as a build user with passwordless
`sudo pacman`.

### Bumping a version by hand

```shell
GITHUB_TOKEN=$(gh auth token) uv run ./pkgreleaser.py $PACKAGE
```

This rewrites `pkgver`, resets `pkgrel`, resolves `_commit` for git sources,
runs `updpkgsums` and regenerates `.SRCINFO`. Requires `pacman-contrib` and
`uv`.

### Removing a package

Delete the directory and its `nvchecker.toml` section. The AUR repository is
not deleted; file an
[orphan or deletion request](https://wiki.archlinux.org/title/AUR_submission_guidelines#Requests)
there.

## Running tests

```shell
prek run --all-files
```

Runs `shellcheck`, `pkglint`, `zizmor`, `actionlint` and `validate_configs.py`
over the whole tree. Builds are opt-in (`stages: [manual]`) and are not part
of that run. [prek](https://github.com/j178/prek) is a drop-in replacement for
pre-commit; `pre-commit` works too.

## How it works

| File                           | Role                                                                                                             |
| ------------------------------ | ---------------------------------------------------------------------------------------------------------------- |
| `.github/workflows/test.yml`   | Lints and test-builds changed packages on PRs (`presubmit`) and merged packages on `master` (`postsubmit`).      |
| `.github/workflows/hourly.yml` | Runs `pkgreleaser.py`, commits each bump to a `pkgreleaser/<pkg>/<version>` branch and opens an auto-merging PR. |
| `.github/workflows/deploy.yml` | `git subtree push`es every package directory to its AUR repository, skipping ones whose tree already matches.    |
| `.github/actions/build-user`   | Creates an unprivileged `builder` user, since `makepkg` refuses to run as root.                                  |
| `.github/actions/prek`         | Installs prek and runs every hook, including the manual build stage, over the files a change touches.            |
| `.pre-commit-config.yaml`      | Hook definitions. Packages that cannot build in CI go in the `pkglint-build` `exclude:` list.                    |
| `nvchecker.toml`               | One section per package describing where to look for new upstream versions.                                      |
| `pkgreleaser.py`               | Runs nvchecker and rewrites the PKGBUILD/.SRCINFO of any package with a newer upstream version.                  |
| `validate_configs.py`          | Checks that every package directory has an nvchecker entry and vice versa.                                       |
| `.gitignore`                   | Keeps package directories to only what the AUR needs; makepkg's sources and build products are ignored.          |
