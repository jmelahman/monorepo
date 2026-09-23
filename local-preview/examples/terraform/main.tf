data "aws_region" "current" {}

# Default VPC/subnet only when the caller doesn't pin one, so the example
# stands alone in a fresh account without pulling in a VPC module.
data "aws_vpc" "default" {
  count   = local.need_default_subnets ? 1 : 0
  default = true
}

data "aws_subnets" "default" {
  count = local.need_default_subnets ? 1 : 0

  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default[0].id]
  }
}

data "aws_ami" "al2023" {
  count       = var.ami_id == null ? 1 : 0
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-x86_64"]
  }
}

locals {
  # The load balancer needs subnets in two AZs, so TLS pulls in the default
  # VPC even when the caller pinned the instance's own subnet.
  need_default_subnets = var.subnet_id == null || (var.enable_tls && var.alb_subnet_ids == null)

  subnet_id      = var.subnet_id != null ? var.subnet_id : sort(data.aws_subnets.default[0].ids)[0]
  alb_subnet_ids = var.alb_subnet_ids != null ? var.alb_subnet_ids : data.aws_subnets.default[0].ids
  ami_id         = var.ami_id != null ? var.ami_id : data.aws_ami.al2023[0].id

  scheme = var.enable_tls ? "https" : "http"

  # $PREVIEW_CONFIG_DIR, bind-mounted read-only. Holds manifests/<repo>.toml.
  config_dir = "/etc/local-preview"

  tags = merge({
    Name      = var.name
    ManagedBy = "terraform"
  }, var.tags)

  # GitHub matches the OAuth App's callback URL exactly, so it has to be the
  # public origin — the load balancer's scheme and the preview domain, which
  # is what the module's certificate and records cover.
  sso_callback_url = var.sso == null ? null : coalesce(
    var.sso.callback_url,
    "${local.scheme}://${var.preview_domain}/api/auth/callback",
  )

  # Non-secret flags only. The client secret arrives through the env file
  # (PREVIEW_SSO_GITHUB_CLIENT_SECRET) so it stays out of `ps` and of state.
  sso_args = var.sso == null ? [] : concat(
    [
      "--sso-github-client-id", var.sso.github_client_id,
      "--sso-callback-url", local.sso_callback_url,
    ],
    var.sso.allowed_org != null ? ["--sso-allowed-org", var.sso.allowed_org] : [],
    var.sso.allowed_team != null ? ["--sso-allowed-team", var.sso.allowed_team] : [],
    length(var.sso.allowed_logins) > 0 ? ["--sso-allowed-logins", join(",", var.sso.allowed_logins)] : [],
    length(var.sso.allowed_emails) > 0 ? ["--sso-allowed-emails", join(",", var.sso.allowed_emails)] : [],
  )

  oidc_upload_args = var.github_oidc_audience == null ? [] : [
    "--github-oidc-audience", var.github_oidc_audience,
  ]

  # The server only ever sees plain HTTP, so left alone it hands out http://
  # preview URLs even when the load balancer serves them over TLS. That is
  # cosmetic until previews are gated — then the redirect handshake carries a
  # Secure cookie, which an http:// URL doesn't. A caller who sets the flag
  # in extra_server_args keeps their own value.
  base_url = var.enable_tls || var.http_port == 80 ? (
    "${local.scheme}://${var.preview_domain}"
  ) : "http://${var.preview_domain}:${var.http_port}"

  base_url_args = contains(var.extra_server_args, "--preview-base-url") ? [] : [
    "--preview-base-url", local.base_url,
  ]

  # With a worker tier, the server becomes the control node: builds stay here,
  # serving is routed to the workers.
  worker_autoscale = var.workers != null && var.workers.autoscale != null
  worker_static    = var.workers != null && var.workers.autoscale == null

  # CloudWatch namespace for the fleet autoscaling metrics. One source of truth
  # for the control node's --metrics-namespace flag, the IAM PutMetricData
  # condition, and the scaling alarms' namespace.
  metrics_namespace = "LocalPreview"

  # The instance types the elastic tier's mixed-instances policy draws from:
  # the explicit list when given, else the single instance_type. The launch
  # template still names instance_type as its own default, but the ASG's
  # overrides below are what actually launch, so this drives the spot pool set.
  worker_instance_types = var.workers == null ? [] : (
    length(var.workers.instance_types) > 0 ? var.workers.instance_types : [var.workers.instance_type]
  )

  # Static tier: endpoints rendered from the literal private_ips, so user_data
  # stays known at plan time. Elastic workers aren't listed anywhere — they
  # self-register at runtime, which is the whole point (nothing dynamic reaches
  # user_data to defeat user_data_replace_on_change).
  worker_endpoints = local.worker_static ? [
    for ip in var.workers.private_ips : "http://${ip}:${var.workers.api_port}"
  ] : []
  worker_args = var.workers == null ? [] : concat(
    ["--role", "control"],
    local.worker_autoscale ? [
      # Accept worker self-registrations on the control node's pinned private
      # address; workers dial it to join and leave the fleet.
      "--control-listen", "${var.private_ip}:${var.workers.control_port}",
      # Publish fleet autoscaling metrics (UnservedDemand, FleetLoad) to
      # CloudWatch under this ASG's name and reconcile scale-in protection on
      # busy workers. The control node reaches the AWS APIs via its instance
      # role over host-network IMDS (hop limit 1). The ASG name is the literal
      # "${var.name}-worker" — not aws_autoscaling_group.worker[0].name — so it
      # stays known at plan time; a resource reference would render user_data
      # "(known after apply)" and silently defeat user_data_replace_on_change.
      "--worker-asg-name", "${var.name}-worker",
      "--aws-region", data.aws_region.current.name,
      "--metrics-namespace", local.metrics_namespace,
      ] : [
      "--worker-endpoints", join(",", local.worker_endpoints),
    ],
  )

  server_args = concat(local.base_url_args, local.sso_args, local.oidc_upload_args, local.worker_args, var.extra_server_args)

  # All SecureString parameters the instances read at boot; none is a
  # Terraform value, so none reaches state.
  secret_ssm_parameters = distinct(concat(compact([
    var.github_webhook_secret_ssm_parameter,
    var.sso == null ? null : var.sso.client_secret_ssm_parameter,
    var.workers == null ? null : var.workers.secret_ssm_parameter,
  ]), values(var.secret_env_ssm_parameters)))

  # What the *worker* boot script reads: the shared secret plus the secret env
  # (a worker resolves manifest {secret:...} placeholders itself).
  worker_secret_ssm_parameters = var.workers == null ? [] : distinct(concat(
    [var.workers.secret_ssm_parameter],
    values(var.secret_env_ssm_parameters),
  ))
}

# The data volume is AZ-bound, so it has to land in the instance's AZ.
data "aws_subnet" "instance" {
  id = local.subnet_id
}

resource "aws_security_group" "server" {
  name        = var.name
  description = "Dashboard and preview ingress for ${var.name}"
  vpc_id      = data.aws_subnet.instance.vpc_id

  # Behind a load balancer the instance takes traffic from it and nothing
  # else; without one it is the front door itself.
  dynamic "ingress" {
    for_each = var.enable_tls ? [] : [1]

    content {
      description = "Dashboard and previews"
      from_port   = var.http_port
      to_port     = var.http_port
      protocol    = "tcp"
      cidr_blocks = var.allowed_ingress_cidrs
    }
  }

  dynamic "ingress" {
    for_each = var.enable_tls ? [1] : []

    content {
      description     = "Dashboard and previews, from the load balancer"
      from_port       = var.http_port
      to_port         = var.http_port
      protocol        = "tcp"
      security_groups = [aws_security_group.alb[0].id]
    }
  }

  dynamic "ingress" {
    for_each = var.allow_ssh ? [1] : []

    content {
      description = "SSH"
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      cidr_blocks = var.allowed_ingress_cidrs
    }
  }

  egress {
    description = "All egress: target builds fetch their own dependencies"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = local.tags
}

data "aws_iam_policy_document" "ec2_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "instance" {
  name               = var.name
  assume_role_policy = data.aws_iam_policy_document.ec2_assume.json
  tags               = local.tags
}

# Session Manager, so the module needn't open port 22 to get a shell.
resource "aws_iam_role_policy_attachment" "ssm_core" {
  role       = aws_iam_role.instance.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

# One read per configured secret parameter: the webhook secret, the SSO
# client secret, or both.
data "aws_iam_policy_document" "secrets" {
  count = length(local.secret_ssm_parameters) > 0 ? 1 : 0

  statement {
    actions = ["ssm:GetParameter"]
    resources = [
      for name in local.secret_ssm_parameters :
      "arn:aws:ssm:${data.aws_region.current.name}:*:parameter/${trimprefix(name, "/")}"
    ]
  }
}

resource "aws_iam_role_policy" "secrets" {
  count = length(local.secret_ssm_parameters) > 0 ? 1 : 0

  name   = "${var.name}-secrets"
  role   = aws_iam_role.instance.id
  policy = data.aws_iam_policy_document.secrets[0].json
}

resource "aws_iam_instance_profile" "instance" {
  name = var.name
  role = aws_iam_role.instance.name
  tags = local.tags
}

# Control-node autoscaling permissions: publish the fleet metrics and
# reconcile scale-in protection on busy workers. Only the elastic tier's
# control node needs these — the cloudscale publisher (cmd/server) calls exactly
# PutMetricData and SetInstanceProtection, nothing else.
data "aws_iam_policy_document" "autoscale" {
  count = local.worker_autoscale ? 1 : 0

  # PutMetricData has no resource-level ARN; scope it to our namespace with a
  # condition so this role can't write metrics into any other namespace.
  statement {
    actions   = ["cloudwatch:PutMetricData"]
    resources = ["*"]

    condition {
      test     = "StringEquals"
      variable = "cloudwatch:namespace"
      values   = [local.metrics_namespace]
    }
  }

  # Drain-aware scale-in: protect a worker that is serving a preview so the ASG
  # reclaims an idle node instead. Scoped to this deployment's worker ASG.
  statement {
    actions   = ["autoscaling:SetInstanceProtection"]
    resources = [aws_autoscaling_group.worker[0].arn]
  }
}

resource "aws_iam_role_policy" "autoscale" {
  count = local.worker_autoscale ? 1 : 0

  name   = "${var.name}-autoscale"
  role   = aws_iam_role.instance.id
  policy = data.aws_iam_policy_document.autoscale[0].json
}

resource "aws_ebs_volume" "data" {
  availability_zone = data.aws_subnet.instance.availability_zone
  size              = var.data_volume_size_gb
  type              = "gp3"
  encrypted         = true

  tags = merge(local.tags, { Name = "${var.name}-data" })

  # Artifacts, mirror clones, and per-artifact state live here; losing it
  # means every registered repo has to be re-registered and rebuilt.
  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_instance" "server" {
  ami                         = local.ami_id
  instance_type               = var.instance_type
  subnet_id                   = local.subnet_id
  private_ip                  = var.private_ip
  key_name                    = var.key_pair_name
  iam_instance_profile        = aws_iam_instance_profile.instance.name
  vpc_security_group_ids      = concat([aws_security_group.server.id], aws_security_group.stack_ingress[*].id, aws_security_group.worker_registration[*].id)
  associate_public_ip_address = true

  # Gzipped, which cloud-init unpacks itself: EC2 caps user data at 16 KiB and
  # local_manifests/compose_stacks are embedded in it, so plain text runs out
  # of room after a couple of repos.
  user_data_base64 = base64gzip(templatefile("${path.module}/user-data.sh.tftpl", {
    aws_region      = data.aws_region.current.name
    compose_stacks  = var.compose_stacks
    compose_version = var.docker_compose_version
    config_dir      = local.config_dir
    data_dir        = var.data_dir
    data_volume_id  = aws_ebs_volume.data.id
    http_port       = var.http_port
    image           = var.image
    local_manifests = var.local_manifests
    preview_domain  = var.preview_domain
    # Each arg is double-quoted for systemd's ExecStart parser: unquoted, a
    # value with spaces ("--min-warm-window Mon-Fri 08:00-18:00 ...")
    # word-splits into stray args and crash-loops the service. Args must not
    # themselves contain double quotes or dollar signs (the unit is written
    # through an unquoted heredoc).
    server_args    = join(" ", [for a in local.server_args : "\"${a}\""])
    webhook_ssm    = var.github_webhook_secret_ssm_parameter == null ? "" : var.github_webhook_secret_ssm_parameter
    sso_secret_ssm = var.sso == null ? "" : var.sso.client_secret_ssm_parameter
    worker_ssm     = var.workers == null ? "" : var.workers.secret_ssm_parameter
    secret_env     = var.secret_env_ssm_parameters
  }))

  # The image is baked into the unit file that user_data writes, so an image
  # bump only takes effect if the instance is rebuilt. The data volume is a
  # separate resource and survives.
  user_data_replace_on_change = true

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
    # Hop limit 1 keeps IMDS reachable from the host and from host-network
    # containers (the orchestrator runs `--network host`) but NOT from preview
    # containers on the docker bridge — those are one hop further out. Without
    # this, arbitrary branch code in a preview could reach IMDSv2 and assume
    # this instance's role (S3 artifacts + the SSM PREVIEW_SECRET_* deps
    # credentials). Previews never need IMDS: they reach the deps by private IP.
    http_put_response_hop_limit = 1
  }

  root_block_device {
    volume_size           = var.root_volume_size_gb
    volume_type           = "gp3"
    encrypted             = true
    delete_on_termination = true
  }

  tags = local.tags
}

resource "aws_volume_attachment" "data" {
  device_name = "/dev/sdf"
  volume_id   = aws_ebs_volume.data.id
  instance_id = aws_instance.server.id
}

# ---------------------------------------------------------------------------
# TLS. The server speaks only HTTP, so the certificate lives on a load
# balancer in front of it. Preview hosts are one label deep, so a single
# wildcard certificate covers every repo.
# ---------------------------------------------------------------------------

resource "aws_security_group" "alb" {
  count = var.enable_tls ? 1 : 0

  name        = "${var.name}-alb"
  description = "Public ingress for ${var.name}"
  vpc_id      = data.aws_subnet.instance.vpc_id

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = var.allowed_ingress_cidrs
  }

  ingress {
    description = "HTTP, redirected to HTTPS"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = var.allowed_ingress_cidrs
  }

  egress {
    description = "To the server, and to the IdP when OIDC is on"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = local.tags
}

resource "aws_acm_certificate" "previews" {
  count = var.enable_tls ? 1 : 0

  domain_name               = var.preview_domain
  subject_alternative_names = ["*.${var.preview_domain}"]
  validation_method         = "DNS"
  tags                      = local.tags

  lifecycle {
    create_before_destroy = true

    precondition {
      condition     = var.route53_zone_id != null
      error_message = "enable_tls needs route53_zone_id: the certificate is DNS-validated. Set enable_tls = false to serve plain HTTP instead."
    }
  }
}

# Keyed by domain name, the one part of domain_validation_options known at
# plan time. The apex and the wildcard validate to the same CNAME, so the two
# instances write the same record — hence allow_overwrite.
resource "aws_route53_record" "cert_validation" {
  for_each = var.enable_tls ? {
    for dvo in aws_acm_certificate.previews[0].domain_validation_options :
    dvo.domain_name => dvo
  } : {}

  zone_id         = var.route53_zone_id
  name            = each.value.resource_record_name
  type            = each.value.resource_record_type
  records         = [each.value.resource_record_value]
  ttl             = 60
  allow_overwrite = true
}

resource "aws_acm_certificate_validation" "previews" {
  count = var.enable_tls ? 1 : 0

  certificate_arn         = aws_acm_certificate.previews[0].arn
  validation_record_fqdns = [for r in aws_route53_record.cert_validation : r.fqdn]
}

resource "aws_lb" "server" {
  count = var.enable_tls ? 1 : 0

  name               = var.name
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb[0].id]
  subnets            = local.alb_subnet_ids

  # Build logs stream, so don't cut idle connections at the 60s default.
  idle_timeout = 300

  drop_invalid_header_fields = true

  # Only the ALB sees requests it turned away, so this is the sole record of
  # traffic that never reached the server.
  dynamic "access_logs" {
    for_each = var.access_logs_bucket == null ? [] : [var.access_logs_bucket]
    content {
      bucket  = access_logs.value
      prefix  = var.access_logs_prefix
      enabled = true
    }
  }

  tags = local.tags
}

resource "aws_lb_target_group" "server" {
  count = var.enable_tls ? 1 : 0

  name        = var.name
  port        = var.http_port
  protocol    = "HTTP"
  target_type = "instance"
  vpc_id      = data.aws_subnet.instance.vpc_id

  health_check {
    path    = "/api/health"
    matcher = "200"
  }

  tags = local.tags
}

resource "aws_lb_target_group_attachment" "server" {
  count = var.enable_tls ? 1 : 0

  target_group_arn = aws_lb_target_group.server[0].arn
  target_id        = aws_instance.server.id
  port             = var.http_port
}

resource "aws_lb_listener" "https" {
  count = var.enable_tls ? 1 : 0

  load_balancer_arn = aws_lb.server[0].arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = aws_acm_certificate_validation.previews[0].certificate_arn

  # OIDC first when configured: without it the server is unauthenticated to
  # anyone the security group lets through.
  dynamic "default_action" {
    for_each = var.oidc != null ? [var.oidc] : []

    content {
      type  = "authenticate-oidc"
      order = 1

      authenticate_oidc {
        issuer                 = default_action.value.issuer
        authorization_endpoint = default_action.value.authorization_endpoint
        token_endpoint         = default_action.value.token_endpoint
        user_info_endpoint     = default_action.value.user_info_endpoint
        client_id              = default_action.value.client_id
        client_secret          = default_action.value.client_secret
        scope                  = default_action.value.scope
        session_timeout        = default_action.value.session_timeout
      }
    }
  }

  default_action {
    type             = "forward"
    order            = var.oidc != null ? 2 : null
    target_group_arn = aws_lb_target_group.server[0].arn
  }
}

# The CLI and webhook senders can't follow a login redirect, so they reach
# the server directly from known ranges instead.
resource "aws_lb_listener_rule" "oidc_bypass" {
  count = var.enable_tls && var.oidc != null && length(var.oidc_bypass_cidrs) > 0 ? 1 : 0

  listener_arn = aws_lb_listener.https[0].arn
  priority     = 1

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.server[0].arn
  }

  condition {
    source_ip {
      values = var.oidc_bypass_cidrs
    }
  }
}

resource "aws_lb_listener" "http" {
  count = var.enable_tls ? 1 : 0

  load_balancer_arn = aws_lb.server[0].arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

# A static address, so the DNS records survive an instance replacement.
resource "aws_eip" "server" {
  instance = aws_instance.server.id
  domain   = "vpc"
  tags     = local.tags
}

# Two records regardless of how many repos are registered: the dashboard
# answers anything whose Host isn't a preview, and preview hosts are
# <sha>-<repo>.<preview_domain> — a single label, so one wildcard covers
# every repo and registering one needs no DNS change.
#
# They point at the load balancer when it terminates TLS, and straight at the
# instance otherwise.
locals {
  dns_names = var.route53_zone_id == null ? [] : [var.preview_domain, "*.${var.preview_domain}"]
}

resource "aws_route53_record" "server" {
  for_each = toset(local.dns_names)

  zone_id = var.route53_zone_id
  name    = each.key
  type    = "A"

  ttl     = var.enable_tls ? null : 300
  records = var.enable_tls ? null : [aws_eip.server.public_ip]

  dynamic "alias" {
    for_each = var.enable_tls ? [1] : []

    content {
      name                   = aws_lb.server[0].dns_name
      zone_id                = aws_lb.server[0].zone_id
      evaluate_target_health = false
    }
  }
}

# ---------------------------------------------------------------------------
# Worker tier. Same image, --role=worker: the control node above routes
# preview serving here over the internal worker API. Workers are disposable —
# no EBS data volume, no builds, no manifests; artifact files hydrate from
# the S3 tier and run specs arrive on the wire. The worker API is remote code
# execution by design: it listens on the worker's private IP and admits only
# the server's security group. Never attach it to a load balancer.
# ---------------------------------------------------------------------------

resource "aws_security_group" "worker" {
  count = var.workers == null ? 0 : 1

  name        = "${var.name}-worker"
  description = "Worker tier for ${var.name}: control-node-only ingress"
  vpc_id      = data.aws_subnet.instance.vpc_id

  ingress {
    description     = "Worker API, from the control node only (RCE surface)"
    from_port       = var.workers.api_port
    to_port         = var.workers.api_port
    protocol        = "tcp"
    security_groups = [aws_security_group.server.id]
  }

  ingress {
    description     = "Proxied preview processes (OS-assigned ports), from the control node only"
    from_port       = 1024
    to_port         = 65535
    protocol        = "tcp"
    security_groups = [aws_security_group.server.id]
  }

  egress {
    description = "All egress: image pulls and artifact hydration"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.tags, { Name = "${var.name}-worker" })
}

# How workers reach the shared dependency stack: the stack publishes its
# ports on the server's (static) private IP, and this group is the only
# ingress to them.
#
# A separate security group attached to the same instance, NOT rules added to
# aws_security_group.server — that SG defines its rules inline, and Terraform
# treats an inline-rule SG as owning every rule on the group: standalone
# aws_vpc_security_group_ingress_rule resources attached to it get silently
# deleted the next time anything updates the SG in place (this happened in
# production — an instance retype wiped the deps ingress). One group per
# owner, never a mix.
resource "aws_security_group" "stack_ingress" {
  count = var.workers == null ? 0 : 1

  name        = "${var.name}-stack-ingress"
  description = "Shared dependency stack ports on ${var.name}, from the worker tier"
  vpc_id      = data.aws_subnet.instance.vpc_id

  dynamic "ingress" {
    for_each = toset([for p in var.workers.stack_ingress_ports : tostring(p)])

    content {
      description     = "Dependency stack port, from workers only"
      from_port       = tonumber(ingress.value)
      to_port         = tonumber(ingress.value)
      protocol        = "tcp"
      security_groups = [aws_security_group.worker[0].id]
    }
  }

  tags = merge(local.tags, { Name = "${var.name}-stack-ingress" })
}

# Elastic tier only: the control node's worker-registration listener, admitting
# the worker security group and nothing else. A dedicated group (attached to the
# control instance) rather than an inline rule on aws_security_group.server —
# that group's rules are inline, and Terraform treats an inline-rule group as
# owning every rule on it, so mixing in a standalone rule (or referencing the
# worker group inline both ways) would either be wiped on the next update or
# form a dependency cycle. This group references the worker group one-way
# (registration → worker → server), so there is no cycle.
resource "aws_security_group" "worker_registration" {
  count = local.worker_autoscale ? 1 : 0

  name        = "${var.name}-worker-registration"
  description = "Worker self-registration to the control node (${var.name} elastic tier)"
  vpc_id      = data.aws_subnet.instance.vpc_id

  ingress {
    description     = "Worker registration API, from workers only"
    from_port       = var.workers.control_port
    to_port         = var.workers.control_port
    protocol        = "tcp"
    security_groups = [aws_security_group.worker[0].id]
  }

  tags = merge(local.tags, { Name = "${var.name}-worker-registration" })
}

resource "aws_iam_role" "worker" {
  count = var.workers == null ? 0 : 1

  name               = "${var.name}-worker"
  assume_role_policy = data.aws_iam_policy_document.ec2_assume.json
  tags               = local.tags
}

resource "aws_iam_role_policy_attachment" "worker_ssm_core" {
  count = var.workers == null ? 0 : 1

  role       = aws_iam_role.worker[0].name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

data "aws_iam_policy_document" "worker_secrets" {
  count = var.workers == null ? 0 : 1

  statement {
    actions = ["ssm:GetParameter"]
    resources = [
      for name in local.worker_secret_ssm_parameters :
      "arn:aws:ssm:${data.aws_region.current.name}:*:parameter/${trimprefix(name, "/")}"
    ]
  }
}

resource "aws_iam_role_policy" "worker_secrets" {
  count = var.workers == null ? 0 : 1

  name   = "${var.name}-worker-secrets"
  role   = aws_iam_role.worker[0].id
  policy = data.aws_iam_policy_document.worker_secrets[0].json
}

resource "aws_iam_instance_profile" "worker" {
  count = var.workers == null ? 0 : 1

  name = "${var.name}-worker"
  role = aws_iam_role.worker[0].name
  tags = local.tags
}

resource "aws_instance" "worker" {
  count = var.workers == null ? 0 : length(var.workers.private_ips)

  ami                    = local.ami_id
  instance_type          = var.workers.instance_type
  subnet_id              = local.subnet_id
  private_ip             = var.workers.private_ips[count.index]
  iam_instance_profile   = aws_iam_instance_profile.worker[0].name
  vpc_security_group_ids = [aws_security_group.worker[0].id]
  # Public IP for egress only (image pulls, S3 hydration): the default VPC has
  # no NAT, and the security group admits nothing inbound but the control node.
  associate_public_ip_address = true

  # Spot, when asked: persistent + stop is the combination that keeps the
  # rest of the design intact — only persistent-request spot instances accept
  # user-initiated stops (the business-hours schedule), an interruption stops
  # rather than terminates (the pinned private IP and instance id survive,
  # and AWS restarts it when capacity returns), and Terraform keeps managing
  # the same instance throughout.
  dynamic "instance_market_options" {
    for_each = var.workers.spot ? [1] : []

    content {
      market_type = "spot"

      spot_options {
        spot_instance_type             = "persistent"
        instance_interruption_behavior = "stop"
      }
    }
  }

  user_data_base64 = base64gzip(templatefile("${path.module}/worker-data.sh.tftpl", {
    aws_region      = data.aws_region.current.name
    image           = var.image
    data_dir        = var.data_dir
    private_ip      = var.workers.private_ips[count.index]
    api_port        = var.workers.api_port
    max_warm        = var.workers.max_warm
    warm_per_gb     = var.workers.warm_per_gb
    warm_reserve_gb = var.workers.warm_reserve_gb
    server_args     = join(" ", [for a in var.workers.extra_server_args : "\"${a}\""]) # systemd-quoted, like the control node's
    worker_ssm      = var.workers.secret_ssm_parameter
    secret_env      = var.secret_env_ssm_parameters
    warm_images     = var.workers.warm_images
    # Static workers are hand-listed on the control node via --worker-endpoints,
    # so they don't self-register. Only the elastic tier sets this.
    control_endpoint = ""
  }))
  user_data_replace_on_change = true

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
    # Hop limit 1 keeps IMDS reachable from the host and from host-network
    # containers (the orchestrator runs `--network host`) but NOT from preview
    # containers on the docker bridge — those are one hop further out. Without
    # this, arbitrary branch code in a preview could reach IMDSv2 and assume
    # this instance's role (S3 artifacts + the SSM PREVIEW_SECRET_* deps
    # credentials). Previews never need IMDS: they reach the deps by private IP.
    http_put_response_hop_limit = 1
  }

  root_block_device {
    volume_size           = var.workers.root_volume_size_gb
    volume_type           = "gp3"
    encrypted             = true
    delete_on_termination = true
  }

  tags = merge(local.tags, { Name = "${var.name}-worker-${count.index}" })
}

# ---------------------------------------------------------------------------
# Elastic worker tier. Same image and --role=worker as the static instances
# above, but with no fixed address: the launch template's user_data carries no
# per-instance value (which keeps user_data_replace_on_change honest), and each
# worker reads its own private IP from IMDS at boot and self-registers with the
# control node's --control-listen API. The Auto Scaling group's desired count
# is owned at runtime — by a scaling policy and/or the caller's scheduled
# actions — so it can sit at 0 and scale up on demand.
# ---------------------------------------------------------------------------

resource "aws_launch_template" "worker" {
  count = local.worker_autoscale ? 1 : 0

  name_prefix   = "${var.name}-worker-"
  image_id      = local.ami_id
  instance_type = var.workers.instance_type

  iam_instance_profile {
    name = aws_iam_instance_profile.worker[0].name
  }

  # Public IP for egress only (image pulls, S3 hydration): the default VPC has
  # no NAT, and the security group admits nothing inbound but the control node.
  network_interfaces {
    associate_public_ip_address = true
    security_groups             = [aws_security_group.worker[0].id]
    delete_on_termination       = true
  }

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
    # Hop limit 1: IMDS reachable from the host and host-network containers (the
    # worker orchestrator runs --network host and reads its own private IP here
    # at boot) but NOT from preview containers on the docker bridge, one hop
    # further out. See the static instance for the full rationale.
    http_put_response_hop_limit = 1
  }

  block_device_mappings {
    device_name = "/dev/xvda"

    ebs {
      volume_size           = var.workers.root_volume_size_gb
      volume_type           = "gp3"
      encrypted             = true
      delete_on_termination = true
    }
  }

  # No instance_market_options here: a launch template used by a mixed-instances
  # policy may not request spot itself (AWS rejects it). Spot is driven by the
  # ASG's instances_distribution instead (on_demand_percentage_above_base = 0),
  # which yields ordinary spot that terminates on interruption — the ASG
  # relaunches to hold desired capacity, and there is no pinned address to
  # preserve (unlike the static tier's persistent/stop request).

  user_data = base64gzip(templatefile("${path.module}/worker-data.sh.tftpl", {
    aws_region      = data.aws_region.current.name
    image           = var.image
    data_dir        = var.data_dir
    api_port        = var.workers.api_port
    max_warm        = var.workers.max_warm
    warm_per_gb     = var.workers.warm_per_gb
    warm_reserve_gb = var.workers.warm_reserve_gb
    server_args     = join(" ", [for a in var.workers.extra_server_args : "\"${a}\""]) # systemd-quoted, like the control node's
    worker_ssm      = var.workers.secret_ssm_parameter
    secret_env      = var.secret_env_ssm_parameters
    warm_images     = var.workers.warm_images
    # No fixed address — the worker reads its private IP from IMDS at boot.
    private_ip = ""
    # Self-register with the control node's pinned private address.
    control_endpoint = "http://${var.private_ip}:${var.workers.control_port}"
  }))

  tag_specifications {
    resource_type = "instance"
    tags          = merge(local.tags, { Name = "${var.name}-worker" })
  }

  # A launch-template change publishes a new version; the ASG tracks $Latest.
  update_default_version = true

  lifecycle {
    precondition {
      condition     = var.private_ip != null
      error_message = "workers.autoscale requires private_ip: elastic workers self-register to the control node's pinned private address."
    }
  }

  tags = local.tags
}

resource "aws_autoscaling_group" "worker" {
  count = local.worker_autoscale ? 1 : 0

  name = "${var.name}-worker"

  min_size         = var.workers.autoscale.min_size
  max_size         = var.workers.autoscale.max_size
  desired_capacity = var.workers.autoscale.desired_capacity

  # Same subnet (hence AZ) as the control node: workers reach the shared
  # dependency stack on its private IP, and a co-located AZ keeps that hop
  # in-zone.
  vpc_zone_identifier = [local.subnet_id]

  # Workers aren't behind the ALB; the control node health-checks them over the
  # worker API itself. EC2 status is all the ASG should act on.
  health_check_type = "EC2"

  # Spot: proactively replace an instance AWS is about to reclaim.
  capacity_rebalance = var.workers.spot ? true : null

  # Mixed instances over several types, not a single-type launch_template: one
  # spot pool (one type × one AZ) goes thin and AWS floods the group with
  # rebalance recommendations, so capacity_rebalance churns instances every
  # few minutes. Spreading across comparable types gives price-capacity-
  # optimized several pools to choose from — it launches into the deepest,
  # cheapest one and rebalances far less. The launch template (below) still
  # carries all the per-instance config; only the type is overridden here.
  mixed_instances_policy {
    launch_template {
      launch_template_specification {
        launch_template_id = aws_launch_template.worker[0].id
        version            = "$Latest"
      }

      dynamic "override" {
        for_each = local.worker_instance_types
        content {
          instance_type = override.value
        }
      }
    }

    instances_distribution {
      # 100% spot when spot is asked for (base 0, 0% on-demand above base),
      # else 100% on-demand. price-capacity-optimized weighs both spot price
      # and pool depth, which is what actually reduces interruptions/churn
      # versus lowest-price alone.
      on_demand_base_capacity                  = 0
      on_demand_percentage_above_base_capacity = var.workers.spot ? 0 : 100
      spot_allocation_strategy                 = "price-capacity-optimized"
    }
  }

  # desired_capacity may be 0, so don't block apply waiting for an instance
  # that isn't coming.
  wait_for_capacity_timeout = "0"

  # desired_capacity is owned at runtime by the scaling policy and the caller's
  # scheduled actions; Terraform sets only the initial value and then stops
  # managing it, or every apply would snap a live-scaled fleet back to the
  # static number.
  lifecycle {
    ignore_changes = [desired_capacity]
  }

  tag {
    key                 = "Name"
    value               = "${var.name}-worker"
    propagate_at_launch = true
  }

  dynamic "tag" {
    # local.tags already carries Name = var.name; the explicit tag above wins
    # for workers, so drop it here to avoid a duplicate-key error.
    for_each = { for k, v in local.tags : k => v if k != "Name" }

    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = true
    }
  }
}

# ---------------------------------------------------------------------------
# Worker autoscaling policies + alarms. The control node publishes two custom
# metrics (namespace local.metrics_namespace, dimension AutoScalingGroupName):
#
#   UnservedDemand — count of placement attempts that found no fresh worker.
#     The scale-FROM-ZERO signal: LoadRatio is degenerate at zero workers (idle
#     and saturated both read as full), so a request arriving with nowhere to
#     run is what launches the first node.
#   FleetLoad — utilization %, published only while >=1 worker is fresh. Drives
#     scale-out under load and scale-in when idle.
#
# desired_capacity is owned by these policies at runtime (the ASG ignores
# changes to it), so the fleet rests at min_size and grows on demand.
# ---------------------------------------------------------------------------

# Scale out on unmet demand — including from zero. A single +1 step, on
# purpose: UnservedDemand counts previews waiting for capacity (deduplicated
# per publish interval), and once ANY worker registers, placement stops
# returning ErrNoWorker entirely (a full fleet falls back to least-loaded), so
# every waiting preview lands on that first node the moment it arrives. Sizing
# the response to the demand VALUE double-provisions: the first live run of a
# proportional step ladder launched three nodes for one deploy's pre-warm and
# drained two of them idle minutes later. Sustained pressure beyond one node is
# load_high's job — it has real utilization to act on.
resource "aws_autoscaling_policy" "scale_out_demand" {
  count = local.worker_autoscale ? 1 : 0

  name                    = "${var.name}-worker-scale-out-demand"
  autoscaling_group_name  = aws_autoscaling_group.worker[0].name
  policy_type             = "StepScaling"
  adjustment_type         = "ChangeInCapacity"
  metric_aggregation_type = "Maximum"
  # Longer than a worker's worst-case boot-to-registration (spot fulfillment +
  # dnf + image pull + self-register ≈ 2–4 min): demand stays >=1 the whole
  # time a node boots (the pre-warm retry keeps re-registering it), and the
  # warmup window is what stops the still-breaching alarm from stacking a
  # second node onto a first that just hasn't finished booting.
  estimated_instance_warmup = 300

  step_adjustment {
    metric_interval_lower_bound = 0
    scaling_adjustment          = 1
  }
}

resource "aws_cloudwatch_metric_alarm" "demand" {
  count = local.worker_autoscale ? 1 : 0

  alarm_name          = "${var.name}-worker-unserved-demand"
  alarm_description   = "A preview request found no worker to place on — scale the fleet out (from zero if need be)."
  namespace           = local.metrics_namespace
  metric_name         = "UnservedDemand"
  dimensions          = { AutoScalingGroupName = aws_autoscaling_group.worker[0].name }
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = 1
  # The metric is published every interval (0 when there's no demand), so a gap
  # means the control node is down — not that demand is zero. Don't scale on it.
  treat_missing_data = "notBreaching"
  alarm_actions      = [aws_autoscaling_policy.scale_out_demand[0].arn]
  tags               = local.tags
}

# Scale out when the running fleet is hot, so demand doesn't have to go fully
# unserved before more capacity arrives.
resource "aws_autoscaling_policy" "scale_out_load" {
  count = local.worker_autoscale ? 1 : 0

  name                      = "${var.name}-worker-scale-out-load"
  autoscaling_group_name    = aws_autoscaling_group.worker[0].name
  policy_type               = "StepScaling"
  adjustment_type           = "ChangeInCapacity"
  metric_aggregation_type   = "Average"
  estimated_instance_warmup = 180

  step_adjustment {
    metric_interval_lower_bound = 0
    scaling_adjustment          = 1
  }
}

resource "aws_cloudwatch_metric_alarm" "load_high" {
  count = local.worker_autoscale ? 1 : 0

  alarm_name          = "${var.name}-worker-load-high"
  alarm_description   = "Fleet utilization is high — add a worker."
  namespace           = local.metrics_namespace
  metric_name         = "FleetLoad"
  dimensions          = { AutoScalingGroupName = aws_autoscaling_group.worker[0].name }
  statistic           = "Average"
  period              = 60
  evaluation_periods  = 2
  comparison_operator = "GreaterThanThreshold"
  threshold           = 70
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_autoscaling_policy.scale_out_load[0].arn]
  tags                = local.tags
}

# Scale in when the fleet sits idle. Busy workers are scale-in-protected by the
# control node, so the ASG reclaims an idle node rather than killing a preview;
# a long evaluation window keeps warm-but-idle capacity around a while before
# giving it up (the on-demand-from-zero cost model — a little warmth retained
# against the next request).
resource "aws_autoscaling_policy" "scale_in" {
  count = local.worker_autoscale ? 1 : 0

  name                    = "${var.name}-worker-scale-in"
  autoscaling_group_name  = aws_autoscaling_group.worker[0].name
  policy_type             = "StepScaling"
  adjustment_type         = "ChangeInCapacity"
  metric_aggregation_type = "Average"

  # Below the threshold (upper bound 0 == threshold): remove one node.
  step_adjustment {
    metric_interval_upper_bound = 0
    scaling_adjustment          = -1
  }
}

resource "aws_cloudwatch_metric_alarm" "load_low" {
  count = local.worker_autoscale ? 1 : 0

  alarm_name          = "${var.name}-worker-load-low"
  alarm_description   = "Fleet utilization is low for a sustained window — reclaim an idle worker."
  namespace           = local.metrics_namespace
  metric_name         = "FleetLoad"
  dimensions          = { AutoScalingGroupName = aws_autoscaling_group.worker[0].name }
  statistic           = "Average"
  period              = 60
  evaluation_periods  = 10
  comparison_operator = "LessThanThreshold"
  threshold           = 20
  # No data means zero workers (load isn't published at zero) — nothing to
  # scale in, so a gap must not breach.
  treat_missing_data = "notBreaching"
  alarm_actions      = [aws_autoscaling_policy.scale_in[0].arn]
  tags               = local.tags
}

# Reclaim an EMPTY worker during working hours. The load-low alarm cannot: the
# min-warm floor keeps 10+ processes running, so with two workers utilization
# sits near 37% — above any scale-in threshold that would not oscillate against
# the 70% scale-out line once the fleet is back to one node (12/16 = 75%). A
# worker with zero running processes is a different fact entirely: its
# scale-in protection is already released, so the shared -1 policy can only
# ever take exactly that node, disturbing no preview. 15 sustained minutes so
# a worker that just went idle gets a grace window to be re-placed onto first.
resource "aws_cloudwatch_metric_alarm" "idle_worker" {
  count = local.worker_autoscale ? 1 : 0

  alarm_name          = "${var.name}-worker-idle"
  alarm_description   = "A worker is serving nothing — reclaim it without waiting for fleet load to collapse."
  namespace           = local.metrics_namespace
  metric_name         = "IdleWorkers"
  dimensions          = { AutoScalingGroupName = aws_autoscaling_group.worker[0].name }
  statistic           = "Average"
  period              = 60
  evaluation_periods  = 15
  comparison_operator = "GreaterThanThreshold"
  threshold           = 0
  # Not published at zero workers (nothing to reclaim) — a gap must not breach.
  treat_missing_data = "notBreaching"
  alarm_actions      = [aws_autoscaling_policy.scale_in[0].arn]
  tags               = local.tags
}
