target "default" {
  context = "."
  dockerfile = "Dockerfile"
  tags = [
    "lahmanja/arch:v1.4.1",
    "lahmanja/arch:latest",
  ]
}
