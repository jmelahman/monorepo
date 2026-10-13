# monorepo-template

A template for a personal monorepo that publishes each of its projects as a
standalone repository. You develop in one repository; every project still has
its own GitHub repository, CI, and releases, and works on its own when cloned.

It is the setup behind [jmelahman/monorepo](https://github.com/jmelahman/monorepo),
minus the projects, and it is kept in step with that monorepo (see
[Staying current](#staying-current)).

## What's included

| Piece                                      | What it does                                                                                                             |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------ |
| `.pre-commit-config.yaml`                  | The root [prek](https://prek.j178.dev) config. Workspace mode also runs each subtree's own config, from its directory.   |
| `.github/workflows/pre-commit.yml`         | CI: `prek` on the pull request's diff, or `--all-files` on `master`. Only projects with changes run their hooks.        |
| `.github/workflows/mirror.yml`             | On push, splits each changed subtree and pushes it to its own repository with [git-orchard](https://github.com/jmelahman/git-orchard). Fast-forward only. |
| `.config/git-orchard/subtrees`             | The manifest: each subtree, its upstream, and the shared profiles it uses.                                             |
| `.config/git-orchard/shared/base/`         | A starter profile: builtin checks, secret scanning, actionlint, zizmor, and the CI workflow every mirror runs.           |
| `.vscode/tasks.json`, `.kanban.toml`       | Editor tasks (prek, sync, status) and [agentic-kanban](https://github.com/jmelahman/agentic-kanban) settings.            |

## Getting started

1. Create your repository from this template, then install the tools:

   ```shell
   uv tool install prek
   go install github.com/jmelahman/git-orchard@latest
   prek install
   ```

2. Create a GitHub App for the mirror action. It stores the App's client ID
   and key on this repository; install the App on each repository you mirror to.

   ```shell
   git orchard github-app
   ```

   Until this is done, `mirror.yml` is skipped.

3. Add a project. `git orchard add` brings an existing repository in with its
   history; a new project can be an ordinary directory plus a manifest entry.

   ```shell
   git orchard add hello git@github.com:you/hello.git
   ```

   Then list its profiles in `.config/git-orchard/subtrees`:

   ```gitconfig
   [subtree "hello"]
   	remote = git@github.com:you/hello.git
   	branch = master
   	shared = base
   ```

4. Give the project a `.pre-commit-config.yaml` with a block for each
   profile, and run `git orchard sync` to fill it in:

   ```yaml
   repos:
     # BEGIN orchard:base
     # END orchard:base
     # The project's own hooks, e.g. its tests.
   ```

5. Commit and push. CI checks the change, and `mirror.yml` publishes `hello/`
   to `you/hello`, with its own copy of the config and CI workflow.

## Day to day

- **Shared tooling.** Edit a profile under `.config/git-orchard/shared/` and
  run `git orchard sync`; every subtree that lists it gets the change in the
  same commit. The `git-orchard-sync` hook fails CI when a subtree's copy
  differs, so none of them can drift.
- **Profiles.** Add one per concern, e.g. `go/` with Go hooks and a
  `.golangci.yml`, or `goreleaser/` with a release workflow. A file in a
  profile lands at the same path in the subtree, replacing it whole, unless
  the subtree's file marks a `BEGIN orchard:<profile>` block.
- **Releases.** `git orchard release hello` tags the next version as
  `hello/v1.2.4` and pushes it; `mirror.yml` publishes it upstream as
  `v1.2.4`, where the project's own release workflow takes over.
- **Dependabot.** Mirrors should not have their own `dependabot.yml`: an
  update opened upstream diverges from the monorepo and blocks the next push.
  Keep one at the root that covers every subtree's directory:

  ```yaml
  version: 2
  updates:
    - package-ecosystem: "github-actions"
      directories: ["/", "/*"]
      schedule:
        interval: "weekly"
      groups:
        actions:
          patterns: ["*"]
  ```

## Staying current

This template is a subtree of
[jmelahman/monorepo](https://github.com/jmelahman/monorepo), where it uses the
same `base` profile as every other project there. `.pre-commit-config.yaml`,
`.github/workflows/pre-commit.yml`, and the starter profile in
`.config/git-orchard/shared/base/` are synced from that monorepo, so each
update to its tooling reaches the template in the same commit.
