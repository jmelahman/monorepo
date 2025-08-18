target "default" {
  context = "."
  dockerfile = "Dockerfile"
  cache_from = [
    "docker.io/lahmanja/namcap:latest",
    "registry.lahman.dev/lahmanja/namcap:latest",
  ]
  cache_to = [
    "type=registry,ref=registry.lahman.dev/lahmanja/namcap:latest,mode=max",
  ]
  tags = [
    "lahmanja/namcap:v3.5.2",
    "lahmanja/namcap:latest",
    "registry.lahman.dev/lahmanja/namcap:latest",
  ]
}
