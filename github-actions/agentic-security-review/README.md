# agentic-security-review

A GitHub Action that reviews each push for exploitable vulnerabilities and uploads the findings to [code scanning](https://docs.github.com/en/code-security/code-scanning).

It supports:

- **Claude Code** with a Claude subscription.
- **OpenCode** with [OpenCode Console](https://opencode.ai/docs/console/models) pay-as-you-go or a personal [OpenCode Go](https://opencode.ai/docs/console/go) subscription.

Each run reviews the changes since the last analysis.
New findings open alerts, fixed findings close alerts, and the remaining alerts stay open.
Each agent has its own set of alerts, so switching agents starts fresh.

## Usage

### Claude Code

```yaml
on:
  push:
    branches: [main]

permissions: {}

# Prevent concurrent runs from closing each other's alerts.
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
      - uses: jmelahman/agentic-security-review@06987bc39f6dbd241ac83bf59fb5e871b3a7bb50
        with:
          claude-code-oauth-token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
```

Run `claude setup-token` and store the token as the `CLAUDE_CODE_OAUTH_TOKEN` secret.

### OpenCode Console

Use the workflow above, but replace the review step with:

```yaml
- uses: jmelahman/agentic-security-review@06987bc39f6dbd241ac83bf59fb5e871b3a7bb50
  with:
    provider: opencode
    opencode-api-key: ${{ secrets.OPENCODE_API_KEY }}
    model: opencode/deepseek-v4.1-flash
```

For CI, create an [OpenCode service account](https://opencode.ai/docs/console/api#service-accounts) so usage and spending are not tied to a person:

1. In the [Console](https://opencode.ai/console), open **Settings → Service accounts** and select **Add Service Account**. You must be an owner or admin.
2. Open the account and select **Add API Key**. Choose an optional expiration.
3. Set **Permissions** to **Restricted** and **Models** to **Request**, then create and copy the key. It is shown only once.

Store the key as a GitHub secret:

```sh
gh secret set OPENCODE_API_KEY
```

You can set a separate [budget](https://opencode.ai/docs/console/budgets#service-account-budgets) for the service account. Service accounts use Console pay-as-you-go models; they are not covered by Go Teams.

### Personal OpenCode Go

Use the same OpenCode setup and choose an `opencode-go` model:

```yaml
- uses: jmelahman/agentic-security-review@06987bc39f6dbd241ac83bf59fb5e871b3a7bb50
  with:
    provider: opencode
    opencode-api-key: ${{ secrets.OPENCODE_API_KEY }}
    model: opencode-go/kimi-k3
```

## Inputs

| Input                     | Default                                                        | Description                                   |
| ------------------------- | -------------------------------------------------------------- | --------------------------------------------- |
| `provider`                | `claude`                                                       | `claude` or `opencode`.                       |
| `claude-code-oauth-token` | required for Claude                                            | Token from `claude setup-token`.              |
| `opencode-api-key`        | required for OpenCode                                          | OpenCode Console or Go API key.               |
| `github-token`            | `github.token`                                                 | Needs `security-events: write`.               |
| `model`                   | `opus` for Claude; `opencode/deepseek-v4.1-flash` for OpenCode | Model to review with.                         |
| `base`                    | last analyzed commit                                           | Commit or ref to review from.                 |
| `max-diff-bytes`          | `300000`                                                       | Skip larger diffs with a warning.             |
| `exclude`                 | lockfiles, `vendor/`, Markdown                                 | Globs to leave out of the diff, one per line. |

## Security and requirements

The agent can read the checkout but cannot change files or run code.
Findings are as visible as your code scanning alerts.
The action refuses to upload findings that contain a Claude or OpenCode credential.

Do not run this action on `pull_request`: it would give your credential to an agent reviewing code that anyone can submit.

Code scanning is free on public repositories.
Private repositories need GitHub Advanced Security, and uploading SARIF also needs `actions: read`.

## Token usage

Each run adds token counts, duration, and API-equivalent cost to the job summary.
It also uploads them as the `agentic-security-review-usage` artifact, retained for 90 days.
Claude subscription costs are estimates, not additional charges.
