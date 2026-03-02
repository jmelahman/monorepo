target "default" {
  context = "."
  dockerfile = "Dockerfile"
  cache_from = [
    "docker.io/lahmanja/arch:latest",
  ]
  cache_to = [
    "inline",
  ]
  tags = [
    "lahmanja/arch:2026-03-01",
    "lahmanja/arch:latest",
  ]
}
