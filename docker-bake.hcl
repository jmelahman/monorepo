group "default" {
  targets = ["devcontainer", "godot-devcontainer", "android-builder"]
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

# The base plus Godot, for templates/godot-template and the games built from
# it. `contexts` points its FROM at the target above rather than the registry,
# so both build from the same commit in one bake.
target "godot-devcontainer" {
  context = "templates/godot-template"
  dockerfile = ".devcontainer/Dockerfile"
  contexts = {
    "ghcr.io/jmelahman/devcontainer:latest" = "target:devcontainer"
  }
  platforms = ["linux/amd64", "linux/arm64"]
  cache_from = [
    "ghcr.io/jmelahman/godot-devcontainer:latest",
  ]
  tags = [
    "ghcr.io/jmelahman/godot-devcontainer:latest",
  ]
  labels = {
    "org.opencontainers.image.source" = "https://github.com/jmelahman/monorepo"
  }
  args = {
    BUILDKIT_INLINE_CACHE = 1
  }
}

target "android-builder" {
  context = "tools/containers/android"
  dockerfile = "Dockerfile"
  platforms = ["linux/amd64", "linux/arm64"]
  cache_from = [
    "ghcr.io/jmelahman/android-builder:latest",
  ]
  tags = [
    "ghcr.io/jmelahman/android-builder:latest",
  ]
  labels = {
    "org.opencontainers.image.source" = "https://github.com/jmelahman/monorepo"
  }
  args = {
    BUILDKIT_INLINE_CACHE = 1
  }
}
