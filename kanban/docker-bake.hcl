target "default" {
  context = "."
  dockerfile = "Dockerfile"
  cache_from = [
    "docker.io/lahmanja/kanban:latest",
  ]
  tags = [
    "lahmanja/kanban:latest",
  ]
  args = {
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1
    BUILDKIT_INLINE_CACHE = 1
  }
}
