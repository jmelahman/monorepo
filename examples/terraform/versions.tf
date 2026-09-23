terraform {
  # 1.9 is the floor for variable validation that references another variable,
  # which is how allowed_ingress_cidrs checks whether sso/oidc is configured
  # before permitting 0.0.0.0/0.
  required_version = ">= 1.9.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
