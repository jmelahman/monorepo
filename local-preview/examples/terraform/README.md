# Terraform deployment example (AWS)

Provisions a single EC2 host that runs the orchestrator for a team, so
previews live at `<sha>-<repo>.preview.example.com` instead of
`preview.localhost`.

Everything the server owns is on one encrypted EBS volume; the instance is
otherwise disposable. Replace it and the data volume reattaches with every
registered repo, artifact, and state dir intact.

## What it provisions

- EC2 instance (Amazon Linux 2023) running `lahmanja/local-preview` under
  systemd, restarted on failure and on reboot
- A dedicated gp3 data volume mounted at `/var/lib/local-preview`
  (`prevent_destroy` — it holds the mirror clones, artifacts, and state)
- A security group restricted to `allowed_ingress_cidrs`
- An instance profile with SSM Session Manager (so port 22 stays closed)
- An Elastic IP, plus optional Route 53 records
- An ALB with an ACM wildcard certificate terminating HTTPS, and optional
  OIDC login in front of it (`enable_tls`, on by default)
- Optional GitHub SSO in the server itself (`sso`) and GitHub Actions OIDC on
  the upload endpoints (`github_oidc_audience`)

## Usage

```hcl
module "local_preview" {
  source = "github.com/jmelahman/local-preview//examples/terraform?ref=v0.4.0"

  preview_domain        = "preview.example.com"
  allowed_ingress_cidrs = ["10.0.0.0/8"] # VPN range — see below
  route53_zone_id       = "Z123456789ABCDEFGHIJK"
  instance_type         = "t3.large"
}
```

```sh
terraform init
terraform apply
```

Then register a repo against the dashboard the module printed:

```sh
preview --server https://preview.example.com repo add my-app https://github.com/me/my-app
```

## Authentication

An unconfigured server has none, and reaching it is equivalent to shell
access on this host: anyone who can load the dashboard can register a repo
and make the instance run that repo's build commands. The module therefore
requires `allowed_ingress_cidrs`, and rejects `0.0.0.0/0` unless `sso` or
`oidc` is configured — open ingress is a supported choice, but only once
something is checking who is calling.

There are three ways to add authentication, and they gate different things:

| | Gates | Covers the CLI? |
| --- | --- | --- |
| `sso` | dashboard, `/api/`, and previews, in the server | yes, with a GitHub PAT |
| `oidc` | everything, at the load balancer | no — needs `oidc_bypass_cidrs` |
| `allowed_ingress_cidrs` | everything, at the security group | yes |

`sso` is the one to reach for: it is enforced by the server, so it holds for
any path to the instance, and one allowlist covers browsers and the CLI
alike. `oidc` stops requests earlier — before they reach the instance at all
— but only browsers can complete its redirect. They compose; the security
group stays closed either way unless you decide otherwise.

Opening it is a real option once `sso` is on — it is what lets uploads run
from hosted CI runners, whose addresses you do not control. What you give up
is containment: the security group is what stops a bug in the session check
from being reachable by everyone, so weigh how long the auth path has been
in service. If you do open it, set `access_logs_bucket` — the load balancer
is the only thing that records requests it turned away.

### Sign in with GitHub (`sso`)

Register an **OAuth App** whose Authorization callback URL is exactly the
module's `sso_callback_url` output (by default
`https://<preview_domain>/api/auth/callback`), then:

```sh
aws ssm put-parameter --name /local-preview/sso-client-secret \
  --type SecureString --value "$GITHUB_OAUTH_CLIENT_SECRET"
```

```hcl
sso = {
  github_client_id            = "Iv1.0123456789abcdef"
  client_secret_ssm_parameter = "/local-preview/sso-client-secret"
  allowed_org                 = "my-org"
  # allowed_team   = "platform"        # narrows the org
  # allowed_logins = ["octocat"]       # or an explicit list
}
```

The client secret is read from SSM at boot into a root-only env file, like
the webhook secret — passing it as a Terraform value would persist it in
state and expose it in `ps`. Everything else is a flag.

At least one allowlist rule is required: an empty allowlist would admit every
GitHub user, and both this module and the server refuse to start with one.

Previews are gated too, so a browser bounces once through the dashboard to
pick up a preview-scoped session the first time it opens one. That handshake
needs the dashboard and the previews on the same site, which the module's own
`<preview_domain>` / `*.<preview_domain>` layout already gives you.

### Authenticating at the load balancer (`oidc`)

## DNS is two records, whatever the repo count

Previews resolve as `<sha>-<repo>.<preview_domain>`, which is a single label
below the base domain, so one wildcard covers every repo:

| Record | Serves |
| --- | --- |
| `*.<preview_domain>` | every preview |
| `<preview_domain>` | the dashboard (every Host that isn't `*.<preview_domain>`) |

Registering a repo needs no DNS change. Set `route53_zone_id` and the module
creates both — aliases to the load balancer with `enable_tls`, A records to
the Elastic IP without. Leave it unset and create what `dns_records` lists.

## TLS

On by default. The server itself only speaks HTTP, so `enable_tls` puts an
ALB in front holding an ACM certificate for `<preview_domain>` and
`*.<preview_domain>` — one certificate for every preview, because the host is
a single label. Port 80 redirects to 443, and the instance's security group
stops accepting anything but the load balancer.

The certificate is DNS-validated, so this needs `route53_zone_id`. To manage
DNS elsewhere, set `enable_tls = false` and terminate TLS yourself; the module
then points the records at the instance's Elastic IP and serves plain HTTP.

Set `oidc` and the ALB requires a login before any request reaches the
server:

```hcl
oidc = {
  issuer                 = "https://accounts.google.com"
  authorization_endpoint = "https://accounts.google.com/o/oauth2/v2/auth"
  token_endpoint         = "https://oauth2.googleapis.com/token"
  user_info_endpoint     = "https://openidconnect.googleapis.com/v1/userinfo"
  client_id              = "..."
  client_secret          = var.oidc_client_secret
}

# The CLI can't follow a login redirect.
oidc_bypass_cidrs = ["10.0.0.0/8"]
```

Anything that isn't a browser — the `preview` CLI, webhook senders — cannot
complete the redirect, so it has to come from `oidc_bypass_cidrs`. Those
ranges reach the server unauthenticated, so keep them as tight as the
allowlist you'd have used anyway.

The client secret lands in Terraform state; keep the state encrypted, or pass
it from a secrets manager rather than a `.tfvars` file.

## Build toolchains

The published image bundles no toolchains, so on its own it can only build
targets whose build commands need nothing beyond a shell. The service mounts
the host docker socket, so targets that declare an `image` in their manifest
build in that image on the host's daemon — that, rather than baking toolchains
into the host, is the way to build real projects here. Backend `run` commands
still execute inside the server's container unless the manifest sets
`run_image`.

## Manifests for repos that can't carry one

A commit is normally built from its own `preview.toml`. When the upstream repo
can't take one, put the manifest in `local_manifests` keyed by the registered
repo name and the server falls back to it:

```hcl
local_manifests = {
  my-app = file("${path.module}/manifests/my-app.toml")
}
```

The file is the ordinary `preview.toml` schema. It is written to
`/etc/local-preview/manifests/<name>.toml` by user_data, so editing one
rebuilds the instance, and it appears in the cloud-init log — keep secrets out
of it.

An in-repo `preview.toml` always wins, so adding a manifest here is safe to
leave in place after the repo grows its own.

## Dependency stacks

Previews are per-commit; the databases they talk to are not. `compose_stacks`
runs those shared services on the instance, keyed by compose project name:

```hcl
compose_stacks = {
  onyx = file("${path.module}/stacks/onyx.yaml")
}
```

The key is the project name, which fixes the network name: project `onyx`
owns `onyx_default`, so a manifest with `networks = ["onyx_default"]` joins
it and reaches the services by their compose names.

Each stack runs from `/var/lib/local-preview/stacks/<name>` on the data
volume, started by a `local-preview-stack@<name>.service` unit at boot. Bind
persistent data under that directory (`./data/postgres:/var/lib/...`) so it
survives an instance rebuild; a named volume would sit on the root disk and go
with it. Setting a stack installs the docker compose plugin, which Amazon
Linux doesn't ship.

Like manifests, stack files are written by user_data — editing one rebuilds
the instance, and the contents reach the cloud-init log, so reference an SSM
parameter rather than inlining a credential.

Manifests and stacks share EC2's 16 KiB user-data budget. It is sent gzipped,
which buys roughly a 4× margin, but a deployment with many large files will
eventually hit the cap; fetch those from S3 at boot instead.

## Webhook secret

Set `github_webhook_secret_ssm_parameter` to the name of a SecureString
parameter and the instance reads it at boot into a root-only env file. Passing
the secret as a Terraform variable instead would persist it in state.

```sh
aws ssm put-parameter --name /local-preview/webhook-secret \
  --type SecureString --value "$(openssl rand -hex 32)"
```

Note that GitHub's webhook senders must reach the server — with a closed
security group they can't, so webhook triggers suit a self-hosted forge or an
allowlisted proxy. Watched repos (polling, `--poll-interval`) need no inbound
access at all.

## Uploads from GitHub Actions

CI can publish what it already built — a frontend bundle, a backend tree, a
downloadable artifact — into the server's content-addressed store, and a
deploy of that commit then skips the build. Set an audience to require those
uploads to authenticate:

```hcl
github_oidc_audience = "https://preview.example.com"
```

Every upload then needs a GitHub Actions OIDC token, and the server accepts
one only for the registered repo whose `source` is the same GitHub repository
the token was minted in. Pick a value unique to this server — its URL is the
obvious one. GitHub's *default* audience is the repository owner URL, which
any workflow in the org can mint, so leaving it at the default would let any
org repo upload here.

In the workflow, grant `id-token: write` and pass `--oidc`:

```yaml
- run: |
    preview upload frontend web/dist "$GITHUB_SHA" \
      --repo my-app --server "$PREVIEW_URL" --oidc --deploy
```

Uploads are exempt from `sso` — CI needs no session and no PAT — but they are
*not* exempt from the security group. Hosted runners come from GitHub's
ranges, so with `allowed_ingress_cidrs` closed to an office or VPN range they
cannot reach the server at all; use a self-hosted runner inside an allowed
range, or widen the ingress deliberately.

## Upgrades

Pin `image` to a release tag and bump it to upgrade. The tag is baked into
the systemd unit that user_data writes, so applying a new one rebuilds the
instance; the data volume is a separate resource and reattaches with every
repo, artifact, and state dir intact. Expect a few minutes of downtime while
the replacement boots — the Elastic IP means DNS doesn't change.

```sh
terraform apply -var 'image=lahmanja/local-preview:0.4.0'
```

Note that Docker Hub tags carry no `v` prefix, unlike the git tags.
