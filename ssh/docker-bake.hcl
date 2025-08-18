target "default" {
  context = "."
  dockerfile = "Dockerfile"
  cache_from = [
    "docker.io/lahmanja/connections-ssh:latest",
    "registry.home/lahmanja/connections-ssh:latest",
  ]
  cache_to = [
    "type=registry,ref=registry.home/lahmanja/connections-ssh:latest,mode=max",
  ]
  tags = [
    "lahmanja/connections-ssh:latest",
  ]
  platforms = ["linux/amd64", "linux/arm64"]
}
