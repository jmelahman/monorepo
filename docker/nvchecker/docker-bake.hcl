target "default" {
  context = "."
  dockerfile = "Dockerfile"
  cache_from = [
    "docker.io/lahmanja/nvchecker:latest",
    "registry.lahman.dev/lahmanja/nvchecker:latest",
  ]
  cache_to = [
    "type=registry,ref=registry.lahman.dev/lahmanja/nvchecker:latest,mode=max",
  ]
  tags = [
    "lahmanja/nvchecker:372fce4445159ebd2cab8dab4f3e40e20a54ee9a",
    "lahmanja/nvchecker:latest",
    "registry.lahman.dev/lahmanja/nvchecker:latest",
  ]
}
