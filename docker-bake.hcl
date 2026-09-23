// VERSION is populated from a host env var (see compose.yaml comment for the
// recommended `git describe --tags --always --dirty` invocation). Resolved on
// the host because .git is dockerignored and worktrees only store a gitdir
// pointer.
variable "VERSION" { default = "dev" }

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
    BUILDKIT_INLINE_CACHE = 1
    VERSION = "${VERSION}"
  }
}

target "devcontainer" {
  context = ".devcontainer"
  dockerfile = "Dockerfile"
  cache_from = [
    "docker.io/lahmanja/devcontainer:latest",
  ]
  tags = [
    "lahmanja/devcontainer:latest",
  ]
  args = {
    BUILDKIT_INLINE_CACHE = 1
  }
}
