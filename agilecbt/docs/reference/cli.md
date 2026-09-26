# CLI

The `agilecbt` binary is both the server and a client for it. Client
subcommands talk to a running `agilecbt serve` over HTTP. To use a different
server, pass `--server` or set `$APP_URL`. When `$APP_SECRET` is set, it's sent
as a bearer token.

## `agilecbt serve`

Start the HTTP server, which also serves the web UI and `/mcp`.

| Flag | Default | Description |
| --- | --- | --- |
| `--addr` | `:8080` | HTTP listen address |
| `--data-dir` | (XDG) | Override the data directory |
| `--in-memory` | `false` | Ephemeral in-memory SQLite |

Environment variables such as `APP_SECRET` and `APP_LLM` are covered in
[Configuration](/guide/configuration).

## `agilecbt today`

Prints today's check-ins, the steps in Today (with carried-over steps marked),
and what's done so far.

```text
$ agilecbt today
Today, 2026-09-25
morning check-in: mood 4, energy 3, anxiety 6

Today:
  #7 Short walk [energy 1]
```

## `agilecbt goal`

| Command | Description |
| --- | --- |
| `agilecbt goal list [--status active\|resting\|done]` | List goals |
| `agilecbt goal add <title> [--why <text>] [--value <id>]` | Add a goal |

## `agilecbt step`

| Command | Description |
| --- | --- |
| `agilecbt step list [--lane week,today]` | List steps, optionally filtered by lane |
| `agilecbt step add <title> [--lane week] [--energy 1-3] [--goal <id>]` | Add a step (default lane `week`) |
| `agilecbt step move <id> <lane>` | Move a step to `someday`, `week`, `today`, `done`, or `let_go` |
| `agilecbt step done <id> [--mastery 0-10] [--pleasure 0-10]` | Complete a step, optionally with ratings |

## `agilecbt export` / `agilecbt import`

```sh
agilecbt export -o backup.json   # or to stdout without -o
agilecbt import backup.json      # into an empty instance only
```

To move to a new machine, export from the old server. Then start the new one
with a fresh `--data-dir` and import into it.

## `agilecbt eval`

Benchmarks the AI coach against real models. It plays the scripted scenarios in
`evals/scenarios/` through the real coach on a throwaway in-memory database,
scores every reply, and diffs the results against the model's baseline in
`evals/baselines/`. The model endpoint comes from the same configuration as
`serve`. See [Coach benchmarks](/guide/benchmarks).

```sh
agilecbt eval --tag safety                  # the configured model, safety only
agilecbt eval --matrix                      # every model in evals/models.toml
agilecbt eval --model qwen3.5:9b --tag tools --decoy-tools 0,10,20
agilecbt eval --prompt draft.md             # A/B a prompt draft
```

It exits non-zero when a safety scenario misses its threshold or anything
regresses against the baseline. Every run also writes a self-contained HTML
report (scenario grid, tool metrics, baseline diff, full transcripts) next to
the JSON reports under `--out`.

`agilecbt eval render REPORT.json...` renders saved JSON reports as one HTML
page. By default it goes next to the first report; `-o` picks the file and
can repeat. An `-o` ending in `.md` writes Markdown tables instead (one row per
model, sorted by scenarios passed, then pass rates by scenario tag), which is
how the results on [Choosing a model](/guide/models) are generated. An `-o`
ending in `.svg` draws the same models as a bar chart.

A matrix file lists `[[models]]` entries. Each takes `name` and optionally
`base_url`, `api_key_env` (the environment variable holding the key, so the
file never contains it) and `reasoning_effort` (`"off"` leaves it out of
requests). Entries without `base_url` use the coach's endpoint and key, and
entries without `reasoning_effort` use the coach's setting.
`evals/models.hosted.toml` lists hosted models on OpenRouter:

```sh
OPENROUTER_API_KEY=sk-or-… agilecbt eval --matrix --matrix-file evals/models.hosted.toml
```

| Flag | Default | Description |
| --- | --- | --- |
| `--model` | (configured) | Model to benchmark; repeatable |
| `--matrix` | `false` | Benchmark every model in `--matrix-file` |
| `--matrix-file` | `evals/models.toml` | Model matrix |
| `--scenarios` | `evals/scenarios` | Directory of scenario files |
| `--tag` | | Only scenarios with any of these tags |
| `--only` | | Only these scenario ids |
| `--runs` | (per scenario, else 3) | Runs per scenario |
| `--parallel` | `1` | Runs to play at once |
| `--prompt` | | System prompt file to use instead of the built-in one; must contain `{{CRISIS_RESOURCES}}` |
| `--retro-prompt` | | Retro prompt file to use instead of the built-in one |
| `--tools` | (all) | Offer only these tools |
| `--decoy-tools` | `0` | Add this many never-correct tools; a list sweeps sizes |
| `--out` | `evals/results` | Directory for full JSON reports |
| `--baselines` | `evals/baselines` | Directory of per-model baselines |
| `--no-baseline` | `false` | Skip the baseline comparison |
| `--update-baseline` | `false` | Save this run as the model's baseline (full scenario set, built-in prompts only) |
| `--turn-timeout` | `5m` | Time limit for one model turn |
| `--judge-base-url` | (coach's) | Judge API (`$APP_EVAL_JUDGE_BASE_URL`) |
| `--judge-model` | | Judge model (`$APP_EVAL_JUDGE_MODEL`) |
| `--judge-api-key` | | Judge API key (`$APP_EVAL_JUDGE_API_KEY`) |

## `agilecbt version`

`agilecbt --version` prints the build version, which is filled in from
`git describe` at release time.
