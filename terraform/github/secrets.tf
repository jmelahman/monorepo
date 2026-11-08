# Actions secrets. Terraform only sees each value encrypted with the
# repository's public key, so the state holds ciphertext that only GitHub can
# open. Encrypt a value with:
#
#   claude setup-token    # interactive; copy the token it prints
#   read -rs TOKEN        # paste it
#   export TF_VAR_claude_code_oauth_token=$(printf %s "$TOKEN" \
#     | gh secret set CLAUDE_CODE_OAUTH_TOKEN --no-store -R jmelahman/monorepo)
#
# Plans don't need the variable once the secret exists. To rotate, export the
# new value and run
# `terraform apply -replace='github_actions_secret.claude_code_oauth_token'`.

variable "claude_code_oauth_token" {
  description = "CLAUDE_CODE_OAUTH_TOKEN for monorepo, encrypted by `gh secret set --no-store`."
  type        = string
  sensitive   = true
  # The provider wants a value on every plan, but ignores it once the secret
  # exists. A precondition keeps this placeholder from being uploaded.
  default = "AA=="
}

data "github_actions_secrets" "monorepo" {
  name = github_repository.repo["monorepo"].name
}

data "github_actions_public_key" "monorepo" {
  repository = github_repository.repo["monorepo"].name
}

# The security-review workflow runs Claude Code on the subscription.
resource "github_actions_secret" "claude_code_oauth_token" {
  repository      = github_repository.repo["monorepo"].name
  secret_name     = "CLAUDE_CODE_OAUTH_TOKEN"
  key_id          = data.github_actions_public_key.monorepo.key_id
  value_encrypted = var.claude_code_oauth_token

  lifecycle {
    precondition {
      condition = (
        var.claude_code_oauth_token != "AA==" ||
        contains(data.github_actions_secrets.monorepo.secrets[*].name, "CLAUDE_CODE_OAUTH_TOKEN")
      )
      error_message = "Set TF_VAR_claude_code_oauth_token to create the secret; see the top of secrets.tf."
    }
    # The variable is unset after the first apply, so rotation goes through
    # -replace rather than `gh secret set`.
    ignore_changes = [key_id, value_encrypted]
  }
}

variable "ghcr_token" {
  description = "GHCR_TOKEN for monorepo, encrypted by `gh secret set --no-store`."
  type        = string
  sensitive   = true
  # Same placeholder scheme as claude_code_oauth_token.
  default = "AA=="
}

# The devcontainer workflow's unreal job logs in to ghcr.io with this to pull
# Epic's base image, which GITHUB_TOKEN can't, and to push ours. It is a
# personal access token with read:packages and write:packages from an account
# in the EpicGames organization. Encrypt it as above:
#
#   read -rs TOKEN
#   export TF_VAR_ghcr_token=$(printf %s "$TOKEN" \
#     | gh secret set GHCR_TOKEN --no-store -R jmelahman/monorepo)
resource "github_actions_secret" "ghcr_token" {
  repository      = github_repository.repo["monorepo"].name
  secret_name     = "GHCR_TOKEN"
  key_id          = data.github_actions_public_key.monorepo.key_id
  value_encrypted = var.ghcr_token

  lifecycle {
    precondition {
      condition = (
        var.ghcr_token != "AA==" ||
        contains(data.github_actions_secrets.monorepo.secrets[*].name, "GHCR_TOKEN")
      )
      error_message = "Set TF_VAR_ghcr_token to create the secret; see secrets.tf."
    }
    ignore_changes = [key_id, value_encrypted]
  }
}
