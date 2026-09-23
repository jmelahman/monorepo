# Install

The server manages git repos in-process (no `git` binary needed) and runs
your projects' own build commands, so whatever toolchains your targets build
with — e.g. `go`, `npm` — must be on the `PATH`.

## PyPI (recommended)

```sh
uv tool install local-preview
```

This installs the `preview` binary to `~/.local/bin`. Make sure that
directory is on your `PATH`.

## GitHub releases

Download a prebuilt archive for your platform from the
[releases page](https://github.com/jmelahman/local-preview/releases) and
place the `preview` binary on your `PATH`.

## Docker

```sh
docker run -d --name preview \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v preview-data:/data \
  lahmanja/local-preview:latest
```

::: warning
The published image bundles no build toolchains (`git`, `go`, `npm`, …),
so on its own it can only build targets whose build commands need nothing
beyond a shell. Builds for repos that declare an `image` in their manifest
(see [Build images](../reference/preview-toml.md#build-images)) can instead
run on the host's Docker daemon — see below. Backend **run** commands still
execute inside the server's container either way, so backends must be
self-contained binaries (or need nothing beyond a shell) when the server
runs in Docker. Run the binary on the host when your targets need more.
:::

### Container builds from Compose

The repo's `compose.yaml` gives the composed server what it needs to hand
`image`-declared build steps to the host's Docker daemon:

- the daemon socket mounted at `/var/run/docker.sock` (source overridable
  via `DOCKER_SOCK_PATH`), and
- the data directory bind-mounted at the **same absolute path** on the
  host and in the container. The daemon resolves bind sources against its
  own filesystem, so the scratch paths the server passes it must mean the
  same thing on both sides — a named volume can't provide that.

The service runs as container root so the default invocation just works —
holding the docker socket is root-equivalent on the daemon anyway:

```sh
docker compose up -d
```

To keep the data dir's files owned by your host user instead, run
non-root and grant the supplementary GID that owns the socket:

```sh
mkdir -p ~/.local/share/local-preview
DOCKER_SOCK_GID=$(stat -c '%g' /var/run/docker.sock) \
PREVIEW_UID=$(id -u) PREVIEW_GID=$(id -g) \
  docker compose up -d
```

Rootless daemon — point at the rootless socket; container root maps to
your host user, so the default user is already right:

```sh
DOCKER_SOCK_PATH=$XDG_RUNTIME_DIR/docker.sock docker compose up -d
```

Repos registered by a local path (rather than a clone URL) must also be
bind-mounted into the container at that same path — see the commented
example in `compose.yaml`.

## From source

```sh
git clone https://github.com/jmelahman/local-preview
cd local-preview
npm --prefix web ci && npm --prefix web run build
go build -tags embed -o preview .
```
