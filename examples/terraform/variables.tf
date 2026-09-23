variable "name" {
  description = "Name prefix for every resource this module creates."
  type        = string
  default     = "local-preview"
}

variable "allowed_ingress_cidrs" {
  description = <<-EOT
    CIDRs allowed to reach the dashboard and previews. Required, with no
    default, on purpose: registering a repo makes the host run that repo's
    build commands, so reaching an unauthenticated dashboard is equivalent
    to shell access here. Keep this to a VPN/office range unless `sso` or
    `oidc` is configured.
  EOT
  type        = list(string)

  validation {
    condition     = length(var.allowed_ingress_cidrs) > 0
    error_message = "allowed_ingress_cidrs must not be empty — a server nothing can reach is not a security posture."
  }

  # 0.0.0.0/0 is a decision, not a typo, so it is allowed — but only once
  # something else is checking who is calling. Without sso or oidc the
  # security group is the whole of the authentication, and opening it turns
  # the dashboard's build-running into remote code execution for anyone.
  validation {
    condition     = !contains(var.allowed_ingress_cidrs, "0.0.0.0/0") || var.sso != null || var.oidc != null
    error_message = "Refusing 0.0.0.0/0 with no authentication: an open dashboard runs arbitrary build commands on this host. Set sso (or oidc) first."
  }
}

variable "preview_domain" {
  description = <<-EOT
    Base domain previews are served under, e.g. "preview.example.com".
    Previews resolve as <sha>-<repo>.<preview_domain> — one label, so a
    single wildcard record covers every repo.
  EOT
  type        = string
}

variable "route53_zone_id" {
  description = "Hosted zone for the preview records. Leave null to manage DNS elsewhere and point it at the public_ip output."
  type        = string
  default     = null
}

variable "subnet_id" {
  description = "Public subnet for the instance. Defaults to a subnet of the region's default VPC."
  type        = string
  default     = null
}

variable "access_logs_bucket" {
  description = <<-EOT
    S3 bucket the ALB writes access logs to. Unset disables logging.

    Worth setting whenever allowed_ingress_cidrs is open: the server's own
    logs record what it served, but only the load balancer sees the requests
    it rejected, and a rejected request is the interesting one. The bucket
    must already carry a policy admitting the regional ELB log-delivery
    principal — the module does not create it, since a log bucket usually
    outlives and outscopes one server.

    Ignored when enable_tls is false; without the ALB there is nothing to log.
  EOT
  type        = string
  default     = null
}

variable "access_logs_prefix" {
  description = "Key prefix within access_logs_bucket. Lets one bucket hold logs from several load balancers."
  type        = string
  default     = null
}

variable "enable_tls" {
  description = <<-EOT
    Front the server with an ALB holding an ACM certificate for
    <preview_domain> and *.<preview_domain>, and serve HTTPS. Requires
    route53_zone_id, since the certificate is DNS-validated.

    The server itself only speaks HTTP, so this is the only way to get TLS.
    With it on, the instance stops accepting traffic from anywhere but the
    load balancer.
  EOT
  type        = bool
  default     = true
}

variable "alb_subnet_ids" {
  description = "Subnets for the load balancer; needs at least two in different AZs. Defaults to the default VPC's."
  type        = list(string)
  default     = null
}

variable "oidc" {
  description = <<-EOT
    Authenticate browser sessions at the load balancer. This is what lets you
    widen allowed_ingress_cidrs beyond a trusted range, because the server
    itself has no authentication.

    Non-browser clients cannot complete an OIDC redirect, so the `preview`
    CLI and any webhook sender must come from oidc_bypass_cidrs.
  EOT
  type = object({
    issuer                 = string
    authorization_endpoint = string
    token_endpoint         = string
    user_info_endpoint     = string
    client_id              = string
    client_secret          = string
    scope                  = optional(string, "openid email")
    session_timeout        = optional(number, 43200)
  })
  default   = null
  sensitive = true
}

variable "oidc_bypass_cidrs" {
  description = "Source ranges that skip OIDC — for the CLI and webhook senders, which can't follow a login redirect. Still bounded by allowed_ingress_cidrs."
  type        = list(string)
  default     = []
}

variable "sso" {
  description = <<-EOT
    Sign in with GitHub, enforced by the server itself: the dashboard, the
    `/api/` control plane, and the previews all require a signed-in user on
    the allowlist. Unlike `oidc` above — which authenticates at the load
    balancer and so cannot gate anything reaching the instance directly —
    this covers every path into the server, and the CLI authenticates against
    the same allowlist with a GitHub personal-access token.

    The OAuth App's callback URL must equal `callback_url` exactly; leave it
    null to use https://<preview_domain>/api/auth/callback, which is what the
    module's own DNS and certificate cover.

    The client secret is never a Terraform value — it would land in state and
    in the process table. Put it in a SecureString SSM parameter and name it
    here; the instance reads it at boot into an env file.

    At least one allowlist rule is required: an empty allowlist would admit
    every GitHub user, so the server refuses to start with one.
  EOT
  type = object({
    github_client_id            = string
    client_secret_ssm_parameter = string
    callback_url                = optional(string)
    allowed_org                 = optional(string)
    allowed_team                = optional(string)
    allowed_logins              = optional(list(string), [])
    allowed_emails              = optional(list(string), [])
  })
  default = null

  validation {
    condition = var.sso == null || try(
      var.sso.allowed_org != null ||
      length(var.sso.allowed_logins) > 0 ||
      length(var.sso.allowed_emails) > 0,
    false)
    error_message = "sso needs at least one allowlist rule (allowed_org, allowed_logins, or allowed_emails) — an empty allowlist admits every GitHub user."
  }

  validation {
    condition     = var.sso == null || try(var.sso.allowed_team == null || var.sso.allowed_org != null, false)
    error_message = "sso.allowed_team narrows an org, so it requires sso.allowed_org."
  }
}

variable "github_oidc_audience" {
  description = <<-EOT
    Expected `aud` of the GitHub Actions OIDC tokens that authenticate
    uploads. Setting it makes the upload endpoints require a token minted by
    a workflow in the same GitHub repository the target repo was registered
    from; leaving it null keeps uploads unauthenticated, which is only safe
    while the security group is the whole gate.

    Use a value unique to this server — its own URL,
    https://<preview_domain>, is a good choice. GitHub's *default* audience
    is the repository owner URL, which every workflow in the org can mint, so
    a token minted for it would be replayable here from any repo in the org.

    Uploads stay exempt from `sso`, so CI needs no session and no PAT. The
    workflow passes the same value to `preview upload --oidc-audience`.
  EOT
  type        = string
  default     = null
}

variable "instance_type" {
  description = "Instance type. Previews build and run target repos, so size for the heaviest target build."
  type        = string
  default     = "t3.large"
}

variable "ami_id" {
  description = "AMI to boot. Defaults to the newest Amazon Linux 2023 for the instance's architecture."
  type        = string
  default     = null
}

variable "key_pair_name" {
  description = "EC2 key pair for SSH. Null relies on SSM Session Manager, which the instance profile already allows."
  type        = string
  default     = null
}

variable "allow_ssh" {
  description = "Open port 22 to allowed_ingress_cidrs. Off by default — the instance profile grants SSM Session Manager access."
  type        = bool
  default     = false
}

variable "http_port" {
  description = "Port the server listens on. Previews are addressed without an explicit port only when this is 80."
  type        = number
  default     = 80
}

variable "image" {
  description = "Container image to run. Pin a release tag rather than tracking latest."
  type        = string
  default     = "lahmanja/local-preview:latest"
}

variable "data_dir" {
  description = <<-EOT
    Absolute path of the data directory, mounted from a dedicated EBS volume
    and bind-mounted into the container at the SAME path — build containers
    are started through the host daemon, which resolves bind sources against
    its own filesystem.
  EOT
  type        = string
  default     = "/var/lib/local-preview"
}

variable "data_volume_size_gb" {
  description = "Size of the data volume. Artifacts, mirror clones, and per-artifact state accumulate here until retention evicts them."
  type        = number
  default     = 100
}

variable "root_volume_size_gb" {
  description = "Size of the root volume (OS + container images)."
  type        = number
  default     = 30
}

variable "github_webhook_secret_ssm_parameter" {
  description = <<-EOT
    Name of a SecureString SSM parameter holding the GitHub webhook secret.
    Read on the instance at boot, never passed as a Terraform variable —
    a plain variable would land in state and in the process table.
  EOT
  type        = string
  default     = null
}

variable "local_manifests" {
  description = <<-EOT
    Out-of-repo preview manifests, keyed by registered repo name: the server
    falls back to <name>.toml here when a deployed commit carries no
    preview.toml of its own. For onboarding a repo whose upstream can't take
    the manifest.

    Written to the instance by user_data, so changing one rebuilds the
    instance. They also land in the cloud-init log — don't put secrets here.
  EOT
  type        = map(string)
  default     = {}
}

variable "compose_stacks" {
  description = <<-EOT
    Auxiliary dependency stacks, keyed by compose project name: the contents
    of a compose file, brought up by systemd at boot and left running. This is
    where a target repo's shared Postgres/search/cache lives — previews build
    per commit, their dependencies do not.

    The project name is the key, so a stack named "onyx" gives the docker
    network "onyx_default" — the name a manifest's `networks` joins, and the
    namespace its compose service names resolve in.

    Each stack gets <data_dir>/stacks/<name> as its working directory, on the
    data volume and preserved across instance rebuilds. Bind-mount persistent
    data under it (./data/...) rather than using named volumes, which land on
    the root disk.

    Written by user_data, so changing a stack rebuilds the instance (the data
    volume survives). Stack contents land in the cloud-init log — put
    credentials in an SSM parameter and reference it from the compose file,
    not inline.
  EOT
  type        = map(string)
  default     = {}
}

variable "docker_compose_version" {
  description = "Compose plugin release to install, needed only when compose_stacks is set (Amazon Linux ships docker without it)."
  type        = string
  default     = "v2.39.1"
}

variable "extra_server_args" {
  description = "Additional `preview serve` flags, e.g. [\"--max-warm\", \"16\"]."
  type        = list(string)
  default     = []
}

variable "private_ip" {
  description = <<-EOT
    Static private IP for the server, inside subnet_id's CIDR. Pin it when
    anything else addresses this host by private IP — a worker tier reaching
    the shared dependency stack, most notably — so an instance rebuild does
    not re-roll the address every manifest and worker points at. Requires
    subnet_id to be pinned too: the address must belong to that subnet.
  EOT
  type        = string
  default     = null
}

variable "secret_env_ssm_parameters" {
  description = <<-EOT
    Environment variables read from SecureString SSM parameters at boot, keyed
    by variable name: { PREVIEW_SECRET_FOO = "/path/to/parameter" }. Written to
    the server's env file and to every compose stack's .env (for $${VAR}
    interpolation in stack definitions), and to each worker's env file — so a
    manifest `{secret:FOO}` placeholder resolves identically on every serving
    node. Values never pass through Terraform, so they stay out of state and
    of the cloud-init log.

    Only variables named PREVIEW_SECRET_* are reachable from manifest env; the
    prefix is the server's authorization boundary (manifests are
    repo-controlled). Other names are still written — useful for stack-only
    interpolation values.
  EOT
  type        = map(string)
  default     = {}
}

variable "workers" {
  description = <<-EOT
    Worker tier: instances running the same image with --role=worker, serving
    preview processes the control node routes to. Setting this flips the server
    to --role=control. Choose exactly one tier shape:

      - Static (private_ips): one instance per literal address, and the control
        node gets --worker-endpoints derived from them. Literals keep user_data
        known at plan time so instance replacement still triggers.

      - Elastic (autoscale): an EC2 Auto Scaling group behind a launch template.
        Workers have no fixed address — they read their own private IP from IMDS
        at boot and self-register with the control node's --control-listen API,
        so nothing about a worker reaches anyone's user_data. desired_capacity
        may be 0 (scale-from-zero); a scaling policy / scheduled actions drive it
        at runtime. Requires private_ip (workers register to that address).

    The worker API starts arbitrary preview processes — remote code execution
    by design. The module binds it to the worker's private IP and admits only
    the server's security group; never route it through a load balancer.

    stack_ingress_ports opens the listed server ports to the worker security
    group — how workers reach a shared dependency stack (compose_stacks) that
    publishes ports on the server's private IP.

    secret_ssm_parameter names a SecureString holding the shared secret that
    authenticates both directions of the worker API (and, for the elastic tier,
    worker self-registration).
  EOT
  type = object({
    secret_ssm_parameter = string
    # Static tier: one worker instance per literal private IP. Mutually
    # exclusive with autoscale.
    private_ips   = optional(list(string), [])
    instance_type = optional(string, "r7i.large")
    # Elastic tier: the candidate instance types the mixed-instances policy
    # draws spot capacity from. More types means more spot capacity pools, so
    # price-capacity-optimized allocation lands in a deep, cheap pool and stops
    # the churn a single-type spot request suffers when its one pool is thin
    # (constant rebalance recommendations). Leave empty to fall back to the
    # single instance_type above. Keep the list to comparable memory sizes
    # unless warm_per_gb is set — max_warm is a fixed per-worker cap otherwise,
    # so a smaller instance would over-commit warm slots. The static tier
    # ignores this and always uses instance_type.
    instance_types = optional(list(string), [])
    api_port       = optional(number, 9100)
    # Elastic tier only: the control node's worker-registration listener port.
    control_port = optional(number, 9101)
    max_warm     = optional(number, 12)
    # Per-instance warm calibration. When warm_per_gb > 0 the worker derives its
    # own warm cap from its RAM at boot — (MemTotal_GiB - warm_reserve_gb) *
    # warm_per_gb, floored at 1 — instead of the fixed max_warm above. This is
    # what lets one launch template serve a mixed-instances fleet where a larger
    # node hosts proportionally more warm processes; 0 keeps the fixed max_warm.
    warm_per_gb     = optional(number, 0)
    warm_reserve_gb = optional(number, 1)
    # The root disk holds the OS, the run images, and the hydrated-artifact
    # cache. The worker caps that cache at half the disk by default (see
    # --cache-max-artifact-bytes), so size for images + 2x the working set.
    root_volume_size_gb = optional(number, 60)
    extra_server_args   = optional(list(string), [])
    stack_ingress_ports = optional(list(number), [])
    # Static tier runs persistent spot instances (interruption behavior: stop),
    # so a user-initiated stop (the business-hours schedule) and the pinned
    # private IP both survive. The elastic tier runs ordinary spot instances
    # that terminate on interruption — the ASG relaunches them, and there is no
    # fixed address to preserve. Either way workers are stateless: run specs
    # arrive on the wire and artifact files hydrate from S3, so an interruption
    # only makes previews unavailable until capacity returns.
    spot = optional(bool, false)
    # Images pre-pulled (best-effort, in the background) at worker boot, so
    # the first preview on a fresh worker doesn't pay the multi-GB
    # devcontainer pull. Digest-pinned manifests still pull what they name;
    # a tag listed here warms the shared layers.
    warm_images = optional(list(string), [])
    # Elastic tier: the Auto Scaling group's bounds. desired_capacity is the
    # initial value only — Terraform stops managing it after create (a scaling
    # policy or scheduled actions own it at runtime), so leaving it at 0 boots
    # a truly on-demand-from-zero fleet. max_size is the hard node cap.
    autoscale = optional(object({
      max_size         = number
      min_size         = optional(number, 0)
      desired_capacity = optional(number, 0)
    }))
  })
  default = null

  validation {
    # Exactly one tier shape. Neither would give a control node with nothing to
    # route to; both would be ambiguous.
    condition     = var.workers == null || ((var.workers.autoscale != null) != (length(var.workers.private_ips) > 0))
    error_message = "workers needs exactly one of autoscale (elastic tier) or a non-empty private_ips (static tier) — not both, not neither."
  }
}

variable "tags" {
  description = "Tags merged into every resource."
  type        = map(string)
  default     = {}
}
