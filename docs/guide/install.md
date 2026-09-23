# Install

## PyPI (recommended)

```sh
uv tool install fullstack-template
```

This installs the `app` binary to `~/.local/bin`. Make sure that directory is
on your `PATH`.

## GitHub releases

Download a prebuilt archive for your platform from the
[releases page](https://github.com/jmelahman/fullstack-template/releases) and
place the `app` binary on your `PATH`.

## Docker

```sh
docker run -d --name app \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v app-data:/data \
  lahmanja/fullstack-template:latest
```

## From source

```sh
git clone https://github.com/jmelahman/fullstack-template
cd fullstack-template
npm --prefix web ci && npm --prefix web run build
go build -tags embed -o app .
```
