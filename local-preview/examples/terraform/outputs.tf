output "public_ip" {
  description = "Elastic IP of the server. Point DNS here when route53_zone_id isn't set."
  value       = aws_eip.server.public_ip
}

output "instance_id" {
  description = "EC2 instance id."
  value       = aws_instance.server.id
}

output "dashboard_url" {
  description = "Dashboard URL. Only resolves once the preview_domain record exists."
  value = var.enable_tls || var.http_port == 80 ? (
    "${local.scheme}://${var.preview_domain}/"
  ) : "http://${var.preview_domain}:${var.http_port}/"
}

output "alb_dns_name" {
  description = "Load balancer hostname, or null when enable_tls is off."
  value       = var.enable_tls ? aws_lb.server[0].dns_name : null
}

output "certificate_arn" {
  description = "ACM certificate covering the domain and its wildcard, or null when enable_tls is off."
  value       = var.enable_tls ? aws_acm_certificate.previews[0].arn : null
}

output "sso_callback_url" {
  description = "The OAuth App's Authorization callback URL, which GitHub matches exactly. Null when sso isn't set."
  value       = local.sso_callback_url
}

output "session_command" {
  description = "Open a shell on the instance without SSH."
  value       = "aws ssm start-session --region ${data.aws_region.current.name} --target ${aws_instance.server.id}"
}

output "dns_records" {
  description = "Records that must exist, for zones this module doesn't manage. With enable_tls these are aliases in Route 53 terms; elsewhere, CNAMEs to the same target (the wildcard as a CNAME, the apex however your provider fakes it)."
  value = [
    for name in [var.preview_domain, "*.${var.preview_domain}"] : {
      name  = name
      type  = var.enable_tls ? "ALIAS" : "A"
      value = var.enable_tls ? aws_lb.server[0].dns_name : aws_eip.server.public_ip
    }
  ]
}

output "instance_role_name" {
  description = <<-EOT
    Name of the IAM role the server assumes. Attach an aws_iam_role_policy to
    it to grant the orchestrator anything the module does not itself provide —
    most usefully a bucket for the durable artifact tier, which the server
    reaches through this role rather than a configured keypair.
  EOT
  value       = aws_iam_role.instance.name
}

output "worker_role_name" {
  description = "IAM role name of the worker tier, or null without one. Attach the artifact bucket's read policy here — workers hydrate artifacts but never build, so read-only suffices."
  value       = var.workers == null ? null : aws_iam_role.worker[0].name
}

output "worker_instance_ids" {
  description = "EC2 instance ids of the STATIC worker tier — include them in any start/stop schedule that covers the server, or the control node routes to stopped workers. Empty for the elastic (autoscale) tier, which the ASG owns; drive that fleet with worker_asg_name instead."
  value       = aws_instance.worker[*].id
}

output "worker_asg_name" {
  description = "Name of the elastic worker Auto Scaling group, or null for a static (private_ips) tier. Attach scaling policies and scheduled actions here to drive the fleet's desired capacity."
  value       = local.worker_autoscale ? aws_autoscaling_group.worker[0].name : null
}
