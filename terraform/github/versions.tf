terraform {
  required_version = ">= 1.10"

  required_providers {
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
  }
}

# Authenticates with GITHUB_TOKEN, e.g. `GITHUB_TOKEN=$(gh auth token)`.
provider "github" {
  owner = "jmelahman"
}
