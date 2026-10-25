# State lives in the terraform-state R2 bucket, which is created by hand
# rather than by Terraform (e.g. `wrangler r2 bucket create terraform-state`).
#
# The "r2" profile supplies the account's endpoint and mints short-lived keys
# with r2-credentials.sh, from a `cf auth login` session. Its argument is the
# access key ID of an R2 API token with Object Read & Write on the bucket; the
# token must exist, but its secret is not needed.
#
#   # ~/.aws/config
#   [profile r2]
#   region = auto
#   endpoint_url = https://d6fc045f8fedf22fae2f862897a89a76.r2.cloudflarestorage.com
#   credential_process = /path/to/terraform/github/r2-credentials.sh <access key id>
terraform {
  backend "s3" {
    bucket  = "terraform-state"
    key     = "github/terraform.tfstate"
    profile = "r2"
    region  = "auto"

    use_lockfile                = true
    use_path_style              = true
    skip_credentials_validation = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    skip_s3_checksum            = true
  }
}
