# Authentication (GitHub SSO)

By default a `preview serve` instance is **open**: anything that can reach it
can register repositories and run their build and run commands on the host.
Before exposing an instance to an untrusted network, turn on **Sign in with
GitHub** — it gates the dashboard, the `/api/` control plane, and the deployed
previews behind a GitHub login restricted to an allowlist.

One identity source covers everything: browsers sign in interactively, the CLI
presents a GitHub personal-access token, and both are checked against the same
allowlist.

## Create a GitHub OAuth App

Register an **OAuth App** (not a GitHub App) at
**Settings → Developer settings → OAuth Apps → New OAuth App**:

- **Homepage URL** — your dashboard URL, e.g. `https://preview.example.com`.
- **Authorization callback URL** — `https://preview.example.com/api/auth/callback`.
  GitHub matches this **exactly**; it must equal `--sso-callback-url`.

Copy the **Client ID** and generate a **Client secret**.

## Turn it on

Pass the client credentials, the callback URL, and an allowlist:

```bash
preview serve \
  --sso-github-client-id     "$GH_CLIENT_ID" \
  --sso-github-client-secret "$GH_CLIENT_SECRET" \
  --sso-callback-url         https://preview.example.com/api/auth/callback \
  --sso-allowed-org          my-org
```

All of these have `$PREVIEW_SSO_*` [environment variables](/guide/configuration#environment-variables);
prefer them for the secret, since flags are visible in `ps`.

Setting `--sso-github-client-id` **fails closed**: the server refuses to start
without a secret, a callback URL, and a non-empty allowlist — an empty allowlist
would otherwise admit every GitHub user.

## The allowlist

Authenticating with GitHub proves *who* someone is; the allowlist decides *who
may in*. Configure any combination:

| Flag | Allows |
| --- | --- |
| `--sso-allowed-org` | anyone in the GitHub org (the usual choice for a team) |
| `--sso-allowed-team` | narrows the org to one team slug (requires `--sso-allowed-org`) |
| `--sso-allowed-logins` | a comma-separated list of GitHub usernames |
| `--sso-allowed-emails` | a comma-separated list of verified emails |

Org- or team-based rules make the login request the `read:org` scope and read
the signer's org membership; a login/email-only allowlist needs only
`read:user` and `user:email`. An org's third-party-application policy may
require an owner to approve the OAuth App before members can sign in.

## The CLI (GitHub token)

The interactive flow needs a browser, so scripts and CLI commands present a
**GitHub token** instead. The server verifies it against the GitHub API and
the *same* allowlist, so a token authorizes exactly the accounts a browser
login would.

**If you use the [GitHub CLI](https://cli.github.com), you're already done**:
with no token configured, `preview` automatically presents `gh auth token`.
gh's OAuth token carries `read:org`, which covers org/team allowlists (an
email-only allowlist still needs a PAT with `user:email` — gh's token can't
read verified emails).

To pin an explicit token instead, store a personal-access token once (or set
`$PREVIEW_TOKEN`):

```bash
preview configure --token ghp_yourPersonalAccessToken
```

A PAT needs `read:user` (and `read:org` when the allowlist gates by
org/team). Precedence: `$PREVIEW_TOKEN`, then the stored token, then
`gh auth token`.

CI **uploads** are unaffected: the upload endpoints keep their own
[GitHub Actions OIDC](/guide/uploads) authentication and never require a session
or a personal-access token.

## Viewing previews

Preview subdomains are gated too. The first time you open a preview after
signing in, the browser bounces once through the dashboard to establish a
preview-scoped session, then returns to the preview — after that, every preview
opens directly. Access is all-or-nothing: any allowlisted, signed-in user can
view any preview.

For this handshake and the login cookies to work, reach the dashboard at the
same origin as `--sso-callback-url`, and serve previews under a subdomain of
that site (the default `*.preview.<domain>` layout already does).

## What stays exempt

Two server-to-server paths keep their existing authentication and never require
a browser session:

- **`POST /api/webhooks/github`** — validated by its HMAC signature
  (`--github-webhook-secret`).
- **`POST /api/repos/{repo}/uploads/*`** — gated by GitHub Actions OIDC when
  `--github-oidc-audience` is set.

`GET /api/health` is also always reachable, for liveness probes.
