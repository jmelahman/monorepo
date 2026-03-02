target "default" {
  context = "."
  dockerfile = "Dockerfile"
  cache_from = [
    "docker.io/lahmanja/archlinux:latest",
  ]
  cache_to = [
    "inline",
  ]
  tags = [
    "lahmanja/archlinux:2026-03-01",
    "lahmanja/archlinux:latest",
  ]
}
