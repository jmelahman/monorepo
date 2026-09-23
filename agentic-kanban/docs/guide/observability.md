# Observability

The kanban backend exposes a Prometheus exposition endpoint at `GET /metrics`
covering five signals:

- **HTTP server** — `kanban_http_requests_total{route,method,status}` and
  `kanban_http_request_duration_seconds{route,method}`. `route` is the Go
  1.22+ `ServeMux` pattern, so dynamic IDs collapse into a single label.
- **GitHub REST API** — `kanban_github_api_requests_total{endpoint,method,status}`,
  `kanban_github_api_request_duration_seconds{endpoint,method}`, and three
  gauges sourced from `X-RateLimit-*` response headers:
  `kanban_github_rate_limit_remaining{resource}`,
  `kanban_github_rate_limit_limit{resource}`, and
  `kanban_github_rate_limit_reset_timestamp_seconds{resource}`. `resource`
  mirrors GitHub's bucketing (`core`, `search`, `graphql`, ...).
- **Local git CLI** — `kanban_git_command_duration_seconds{operation}` and
  `kanban_git_command_errors_total{operation}`. `operation` is a small fixed
  set naming the git command the server shelled out to: `worktree_add`
  (creating a session worktree), `worktree_remove` (tearing one down), `diff`
  (the full uncommitted-state patch behind the session diff view), and `fetch`
  (the best-effort `git fetch origin <base>` run before a worktree is created).
  The `diff` timing is the whole multi-step pipeline (rev-parse → merge-base →
  staging into a throwaway index → `diff --cached`), not the individual git
  invocations. These measure *local* git, distinct from the **GitHub REST API**
  signal above, which is HTTP calls to github.com.
- **SQLite queries** — `kanban_db_query_duration_seconds{op,target}` and
  `kanban_db_query_errors_total{op,target}`. `op` is the SQL verb
  (`select`, `insert`, `update`, `delete`, `tx`, `pragma`, `ddl`, ...) and
  `target` is the first identifier after the verb (table name for DML,
  pragma name for `PRAGMA`, etc.).
- **Go runtime / process** — standard collectors from `prometheus/client_golang`.

## Frontend errors in `docker logs`

The web UI reports uncaught exceptions, unhandled promise rejections, React
render errors, React Query 5xx responses, and `≥2s` long tasks to
`POST /api/errors`. The handler writes a one-line summary plus the (truncated)
stack to the server's default logger, so a deployment running under
`docker compose` surfaces them via:

```
docker compose logs kanban | grep "client error"
```

This is independent of the [`[errors]` ticket reporter](./configuration);
even with `[errors] enabled = false` (the default), the log line is still
emitted. Enable the ticket reporter when you want the same events filed onto
a dedicated kanban board for triage in addition to the log stream.

## Bundled Prometheus service

`compose.yaml` includes a `prometheus` service on `kanban-net` that scrapes
`kanban:7474/metrics` every 15s. The TSDB persists in the `prometheus-data`
named volume so a `docker compose restart` keeps history.

The Prometheus UI is **not** published as a host port. Instead, the kanban
backend reverse-proxies it under `/prometheus/`, so the UI is reachable at:

```
http://localhost:7474/prometheus/
```

The proxy target is `$KANBAN_PROMETHEUS_URL` (set to `http://prometheus:9090`
in `compose.yaml`). When the env var is unset (e.g. `wgo run . serve` in
dev) `/prometheus/` returns `503 Service Unavailable` — the `/metrics`
endpoint still works on its own, so a one-off `curl localhost:7474/metrics`
works without Prometheus running.

## Diagnosing GitHub rate-limit issues

Both the PR poller (`internal/github`) and the build-cop poller
(`internal/buildcop`) flow through the instrumented transport, so a query
like:

```promql
sum by (endpoint) (rate(kanban_github_api_requests_total[5m]))
```

shows which endpoint is burning budget, and:

```promql
kanban_github_rate_limit_remaining{resource="core"}
```

surfaces the absolute remaining budget against the per-hour cap.

If the poll cadence is too aggressive for the install:

- **Build-cop** (the heavier of the two pollers) accepts
  `[buildcop].interval` in `.kanban.toml` (default `2m`). The jobs-per-run
  endpoint is the main offender on this poller — completed workflow runs
  are immutable, so they're cached in memory after the first fetch and
  re-used until they age out of the rolling window.
- **PR poller** ticks every 30s and is bounded to 2 pages of 100 PRs per
  repo per tick.

## Diagnosing slow git operations

Worktree creation, the session diff, and the pre-worktree fetch each show up
under their own `operation` label. Mean latency per operation over the last
5 minutes:

```promql
sum by (operation) (rate(kanban_git_command_duration_seconds_sum[5m]))
  / sum by (operation) (rate(kanban_git_command_duration_seconds_count[5m]))
```

Tail latency (p95) for a single operation — e.g. the diff view feeling slow:

```promql
histogram_quantile(0.95,
  sum by (le) (rate(kanban_git_command_duration_seconds_bucket{operation="diff"}[5m])))
```

`fetch` is best-effort and capped at a 10s timeout, so a rising
`rate(kanban_git_command_errors_total{operation="fetch"}[5m])` usually points
at a flaky or slow remote rather than a kanban bug — new worktrees still get
created off the local base when it fails.

## Recording rules

`deploy/recording_rules.yml` pre-computes the audit-friendly expressions
every `evaluation_interval` (15s) and stores the result as a new metric,
so dashboards and ad-hoc queries don't have to re-derive histograms each
time. The shipped rules:

| Recorded metric | Expression |
| --- | --- |
| `kanban:db_query_duration_seconds:p99_5m` | `histogram_quantile(0.99, … kanban_db_query_duration_seconds_bucket …)` by `op,target` |
| `kanban:db_query_duration_seconds:mean_5m` | mean over `op,target` |
| `kanban:http_request_duration_seconds:p95_5m` | `histogram_quantile(0.95, …)` by `method,route` |
| `kanban:http_request_duration_seconds:mean_5m` | mean over `method,route` |
| `kanban:github_api_request_duration_seconds:mean_5m` | mean over `method,endpoint` |
| `kanban:github_api_requests:rate_5m` | request rate by `method,endpoint` |

Each is queryable directly. For example, the slowest DB ops by tail
latency:

```promql
topk(10, kanban:db_query_duration_seconds:p99_5m)
```

The rules file is mounted into the Prometheus container at
`/etc/prometheus/rules/recording_rules.yml`; edit on disk and
`docker compose restart prometheus` to reload.

## Customising the scrape config

`deploy/prometheus.yml` defines a single static job that scrapes the kanban
backend. Mount additional rules or extend the file when adding new
exporters (e.g. node_exporter, cadvisor). The file is mounted read-only
into the container; edit on disk and `docker compose restart prometheus`
to reload.
