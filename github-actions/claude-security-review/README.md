# claude-security-review

A GitHub Action in which Claude Code reviews what each push changed for exploitable vulnerabilities and uploads its findings to [code scanning](https://docs.github.com/en/code-security/code-scanning) as SARIF. It runs on a Claude subscription rather than an API key.

Unlike [anthropics/claude-code-security-review](https://github.com/anthropics/claude-code-security-review), which comments on pull requests, it reviews the default branch and keeps the findings as code scanning alerts: each run adds what's new, closes what a change fixed, and carries the rest forward.

## Usage

```yaml
on:
  push:
    branches: [main]

permissions: {}

# Two runs uploading at once could close each other's alerts.
concurrency:
  group: security-review
  cancel-in-progress: false

jobs:
  review:
    runs-on: ubuntu-latest
    timeout-minutes: 30
    permissions:
      contents: read
      security-events: write
    steps:
      - uses: actions/checkout@v6
        with:
          persist-credentials: false
          fetch-depth: 2
      - uses: jmelahman/claude-security-review@<sha>
        with:
          claude-code-oauth-token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
```

Create the token with `claude setup-token` and store it as the `CLAUDE_CODE_OAUTH_TOKEN` secret. Code scanning is free on public repositories; a private one needs GitHub Advanced Security, and `upload-sarif` also needs `actions: read`.

Don't run it on `pull_request`: it would run Claude with your token on code anyone can open a pull request with.

## Inputs

| Input | Default | |
|---|---|---|
| `claude-code-oauth-token` | required | From `claude setup-token`. |
| `github-token` | `github.token` | Needs `security-events: write`. |
| `model` | `opus` | |
| `base` | last analyzed commit | Commit or ref to review from. |
| `max-diff-bytes` | `300000` | Bigger diffs are skipped with a warning. |
| `exclude` | lockfiles, `vendor/`, Markdown | Globs left out of the diff, one per line. |

## How it works

1. The diff runs from the commit of the last analysis uploaded to `HEAD`, so a skipped or failed run is covered by the next one.
2. Claude Code reviews it with only `Read`, `Grep` and `Glob`, in `--restricted` mode: nothing that runs code, and file access confined to the checkout. It is shown the open alerts in the files the diff touches and says which the diff fixed. The prompt's exclusions follow anthropics/claude-code-security-review's; it reports only findings it is at least 80% confident are exploitable.
3. The findings, plus every open alert it didn't resolve, are uploaded as SARIF. Code scanning closes any alert an upload leaves out, so the carried-forward alerts stay open until a change fixes them or deletes their file.

The findings are as public as your code scanning alerts. The upload refuses anything shaped like a Claude credential.

## Token usage

Each run writes its token counts per model, turns, duration and API-equivalent cost to the job summary, and uploads them as the `claude-security-review-usage` artifact (90 days). On a subscription the cost is what the tokens would cost on the API, not a charge. To total a workflow's runs:

```sh
gh run list -w security-review.yml --json databaseId -q '.[].databaseId' \
  | xargs -I{} gh run download {} -n claude-security-review-usage -D usage/{}
jq -s 'map(.usage.output_tokens) | add' usage/*/usage.json
```

## Development

Developed in [jmelahman/monorepo](https://github.com/jmelahman/monorepo) at `github-actions/claude-security-review`, and mirrored here.
