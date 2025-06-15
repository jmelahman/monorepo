target "default" {
  context = "."
  dockerfile = "Dockerfile"
  cache_from = [
    "docker.io/lahmanja/arch:latest",
    "registry.lahman.dev/lahmanja/arch:latest",
  ]
  cache_to = [
    "type=registry,ref=registry.lahman.dev/lahmanja/arch:latest,mode=max",
  ]
  tags = [
    "lahmanja/arch:v1.4.2",
    "lahmanja/arch:latest",
    "registry.lahman.dev/lahmanja/arch:latest",
  ]
}
