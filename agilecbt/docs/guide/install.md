# Install

## From source

You need Go 1.26+ and [Bun](https://bun.sh).

```sh
cd agilecbt
bun install --cwd web --frozen-lockfile && bun run --cwd web build
go build -tags embed -o agilecbt .
```

Put `agilecbt` on your `PATH`.

## Docker

```sh
APP_SECRET='something long' docker compose up -d --build
```

The image doesn't include the `claude` CLI, so `compose.yaml` defaults to
`APP_LLM=ollama` and connects to Ollama on the host at
`host.docker.internal:11434`. Set `APP_LLM=anthropic` with `ANTHROPIC_API_KEY`,
or `APP_LLM=none`, to change that. To use your Claude subscription, run the
binary directly on a machine where `claude` is logged in.

Data is kept in the `agilecbt-data` volume. Back it up with
`agilecbt export` (see the [CLI](/reference/cli)).
