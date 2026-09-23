# skills

[Agent Skills](https://agentskills.io) I use with Claude Code, kept under version control and
updated via `git`.

## Skills

| Skill | Description |
| --- | --- |
| [testing](skills/testing/) | Rules for tests that fail only when real behavior breaks, distilled from 75 Google Testing on the Toilet posts: what to test, test doubles, structure, assertions, flakiness, and design for testability. |

## Install

### From a clone

```sh
git clone git@github.com:jmelahman/skills.git ~/.claude/skills/jmelahman
cd ~/.claude/skills
ln -s jmelahman/skills/testing testing
```

The clone has no top-level `SKILL.md`, so Claude Code ignores it and picks up only the links.

### Into a project with `git subtree`

To check skills into an existing `git` project so that everyone working on it gets them, vendor
this repository with `git subtree` and link as needed,

```sh
git remote add skills git@github.com:jmelahman/skills.git
git subtree add --prefix .claude/skills/jmelahman skills master --squash
ln -s jmelahman/skills/testing .claude/skills/testing
git add .claude/skills/testing && git commit -m "Add the testing skill"
```

Update with:

```sh
git subtree pull --prefix .claude/skills/jmelahman skills master --squash
```

### As a Claude Code plugin

The repository is its own [plugin marketplace](https://code.claude.com/docs/en/plugin-marketplaces).
Inside Claude Code:

```
/plugin marketplace add jmelahman/skills
/plugin install skills@jmelahman-skills
```

Skills arrive namespaced as `/skills:<name>`, and `/plugin update skills` picks up
new versions. This is the option to pick if you just want the skills.

### With the skills.sh CLI

For Claude Code, Cursor, Copilot, and the other agents [skills.sh](https://skills.sh)
supports:

```sh
npx skills add jmelahman/skills
```

## Checks

[prek](https://github.com/j178/prek) drives everything:

```sh
prek run --all-files
```

CI runs the same hooks — [`.github/workflows/validate.yml`](.github/workflows/validate.yml)

## License

[GPL-3.0-or-later](LICENSE).
