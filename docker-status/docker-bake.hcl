target "default" {
  context = "."
  dockerfile = "Dockerfile"
  cache_from = [
    "docker.io/lahmanja/docker-status:latest",
    "registry.lahman.dev/lahmanja/docker-status:latest",
  ]
  cache_to = [
    "type=registry,ref=registry.lahman.dev/lahmanja/docker-status:latest,mode=max",
  ]
  tags = [
    "lahmanja/docker-status:latest",
    "registry.lahman.dev/lahmanja/docker-status:latest",
  ]
  args = {
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1
    BUILDKIT_INLINE_CACHE = 1
  }
}
