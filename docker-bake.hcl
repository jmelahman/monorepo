group "default" {
  targets = ["devcontainer"]
}

target "devcontainer" {
  context = "templates/fullstack-template/.devcontainer"
  dockerfile = "Dockerfile"
  platforms = ["linux/amd64", "linux/arm64"]
  cache_from = [
    "ghcr.io/jmelahman/devcontainer:latest",
  ]
  tags = [
    "ghcr.io/jmelahman/devcontainer:latest",
  ]
  labels = {
    "org.opencontainers.image.source" = "https://github.com/jmelahman/monorepo"
  }
  args = {
    BUILDKIT_INLINE_CACHE = 1
  }
}
