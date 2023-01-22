# Buildkite

## Docker image

Available on [Dockerhub](https://hub.docker.com/repository/docker/lahmanja/buildkite/).

Built with source from [buildkite/agent](https://github.com/buildkite/agent/) by running,

```shell
.buildkite/steps/build-docker-image.sh alpine lahmanja/buildkite:latest stable 3.41.0 arm64
```

## Running

First, initialize the swarm,

```shell
docker swarm init
```

Next, deploy the stack,

```shell
docker stack deploy -c docker-compose.yml buildkite
```

### Secrets

Make sure to update the `docker-compose.yaml` file with the agent token from [Buildkite](https://buildkite.com/organizations/jamison-lahman/agents).
I couldn't figure out how to pass a `docker secret` as part of the `command`.
