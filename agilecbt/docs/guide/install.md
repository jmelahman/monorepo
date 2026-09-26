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

`compose.yaml` points the coach at Ollama on the host
(`http://host.docker.internal:11434/v1`). Set `APP_LLM_BASE_URL`,
`APP_LLM_API_KEY`, and `APP_MODEL` to use another OpenAI-compatible API such
as OpenRouter, or `APP_LLM=none` to turn the coach off.

Data is kept in the `agilecbt-data` volume. Back it up with
`agilecbt export` (see the [CLI](/reference/cli)).
