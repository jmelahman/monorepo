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
AGILECBT_SECRET='something long' docker compose up -d --build
```

`compose.yaml` points the coach at the host's llama.cpp server
(`http://host.docker.internal:18080/v1`). Set `AGILECBT_LLM_BASE_URL`,
`AGILECBT_LLM_API_KEY`, and `AGILECBT_MODEL` to use another OpenAI-compatible
API such as OpenRouter, or `AGILECBT_LLM=none` to turn the coach off.

Data is kept in the `agilecbt-data` volume. Back it up with
`agilecbt export` (see the [CLI](/reference/cli)).
