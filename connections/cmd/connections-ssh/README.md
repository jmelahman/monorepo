# Connections Over SSH

This serves the [NYT Connections TUI](https://github.com/jmelahman/connections) over SSH.

## Build

From the root of this repository,

```shell
go build ./cmd/connections-ssh
```

## Running Locally

In one terminal, start the server from the root of this repository,

```shell
$ go run ./cmd/connections-ssh
2025/08/07 22:55:16 Starting SSH server on :2222
```

In a separate terminal, connect to the server,

```shell
$ ssh -p 2222 localhost
```

By default, the server looks for an [SSH key](https://wiki.archlinux.org/title/SSH_keys) at `~/.ssh/id_rsa`.
This can be overridden with the `--key-file` flag.

Moreover, if running on port `22` is desired, you'll likely need elevated privileges (not recommended),

```shell
$ sudo connections-ssh --port 22
2025/08/07 22:55:16 Starting SSH server on :22
```

## Install

**go:**

```shell
go install github.com/jmelahman/connections/cmd/connections-ssh@latest
```

**docker:**

```shell
docker run \
  --rm \
  -p 2222:2222 \
  -v $HOME/.ssh:/.ssh \
  lahmanja/connections-ssh:latest
```

## Deploying

This deploys separately from `connections` releases, which only publish the `connections` binary.
The image has its own version, set in [`docker-bake.hcl`](docker-bake.hcl) and [`terraform/main.tf`](terraform/main.tf).

Build the image from the root of this repository,

```shell
docker buildx create --name builder --use  # once
docker buildx bake -f cmd/connections-ssh/docker-bake.hcl
```

Then point the container instance at it,

```shell
cd cmd/connections-ssh/terraform
terraform apply
```
