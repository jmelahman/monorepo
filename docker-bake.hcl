target "default" {
  context = ".devcontainer"
  dockerfile = "Dockerfile"
  cache_from = [
    "docker.io/lahmanja/devcontainer:latest",
  ]
  tags = [
    "lahmanja/devcontainer:latest",
  ]
  args = {
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1
    BUILDKIT_INLINE_CACHE = 1
  }
}
