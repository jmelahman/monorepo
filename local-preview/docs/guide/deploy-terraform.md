# Deploying to a server

Nothing about the orchestrator requires it to be local — point a wildcard
domain at a host running `preview serve` and a team shares one set of
previews. [`examples/terraform/`](https://github.com/jmelahman/local-preview/tree/master/examples/terraform)
is a worked AWS deployment: one EC2 instance, one data volume, one address.

```hcl
module "local_preview" {
  source = "github.com/jmelahman/local-preview//examples/terraform?ref=v0.10.0"

  preview_domain        = "preview.example.com"
  allowed_ingress_cidrs = ["10.0.0.0/8"]
  route53_zone_id       = "Z123456789ABCDEFGHIJK"
}
```

The instance runs the published image under systemd with the
[data directory](/guide/configuration#data-directory) on its own EBS volume,
so the host stays disposable: replace it and the volume reattaches with every
registered repo, artifact, and state dir intact.

The rest of this page is what changes once the server isn't local. The
example's README covers the module's own knobs.

## Expose it to a trusted network only

The server has no authentication. Anyone who can reach the dashboard can
register a repo and make the host run that repo's build commands, which is
shell access by another name. The example requires an ingress allowlist and
refuses `0.0.0.0/0`; widen the audience by authenticating in front of the
server — its `oidc` block, Tailscale, Cloudflare Access — never with an open
security group.

## Two DNS records

Preview hosts are `<sha>-<repo>.<domain>` — one label below the base domain,
so `*.<domain>` answers for every repo and registering a new one needs no DNS
change. A second record for the base domain itself serves the dashboard,
since every Host that isn't `*.<domain>` does.

The hyphen is what makes that possible: a DNS wildcard (and a wildcard
certificate) matches exactly one label, so a dotted `<sha>.<repo>.<domain>`
would need a record and a cert per repo.

Then set the base domain, whose default is `preview.localhost`:

```sh
preview serve --preview-domain preview.example.com
```

## TLS

The server speaks HTTP and has no certificate configuration, so TLS has to be
terminated in front of it. One `*.<domain>` certificate covers every preview,
which makes that cheap: the example puts an ALB with an ACM wildcard
certificate ahead of the instance, redirects port 80 to 443, and closes the
instance to everything but the load balancer. It needs a Route 53 zone,
because the certificate is DNS-validated.

That load balancer is also where authentication belongs. The example takes an
`oidc` block that makes the ALB require a login before any request reaches the
server — the only way to widen access beyond a trusted range, given the server
authenticates nothing itself. Non-browser clients (the CLI, webhook senders)
can't complete a login redirect, so they need an exempted source range.

## Manifests live on the server

A [local manifest](/reference/preview-toml#local-manifests-repos-you-can-t-change)
is read from the server's config dir, which on a shared host isn't a directory
anyone edits by hand. The example takes them as a `local_manifests` map keyed
by repo name and writes them out with the rest of the instance config, so
onboarding a repo that can't carry a `preview.toml` is a Terraform change like
any other.

## Dependencies live beside the server

[External dependencies](/guide/external-dependencies) — the Postgres, search,
and cache a target expects — are shared across previews, so on a shared host
nobody should be starting them by hand: an instance rebuild would take them
with it. The example takes them as `compose_stacks`, a map of compose project
name to compose file, and runs each as a systemd unit with its working
directory on the data volume.

The project name is the contract between the two halves: stack `onyx` owns the
network `onyx_default`, which is what a manifest's `networks` joins to resolve
the compose service names its `env` points at.

## Toolchains on the server

Build commands run on the server, not on the machine that triggered the
deploy, so whatever your targets build with has to be reachable there. The
published image bundles no toolchains — the practical answer for a container
deployment is to give the server the host's docker socket and have targets
declare an [`image`](/reference/preview-toml#build-images) in their manifest,
which the example does. Installing toolchains on the host instead only helps
when the server runs directly on it.

## Retention matters more here

A shared server accumulates artifacts from everyone's commits. The default
[retention policy](/guide/configuration#retention-garbage-collection) keeps
everything; set a per-repo deploy count or a max age from the dashboard's
**Storage & retention** dialog once real traffic starts, and size the data
volume for what's left.
