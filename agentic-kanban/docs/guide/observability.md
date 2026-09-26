# Observability

Kanban serves Prometheus metrics at `GET /metrics`.

| Area         | Metrics                                                                                                              | Labels                    |
| ------------ | -------------------------------------------------------------------------------------------------------------------- | ------------------------- |
| HTTP server  | `kanban_http_requests_total`, `kanban_http_request_duration_seconds`                                                 | `route`, `method`, `status` |
| GitHub API   | `kanban_github_api_requests_total`, `kanban_github_api_request_duration_seconds`                                     | `endpoint`, `method`, `status` |
| GitHub quota | `kanban_github_rate_limit_remaining`, `kanban_github_rate_limit_limit`, `kanban_github_rate_limit_reset_timestamp_seconds` | `resource` (`core`, `search`, `graphql`, ...) |
| Local git    | `kanban_git_command_duration_seconds`, `kanban_git_command_errors_total`                                             | `operation`               |
| SQLite       | `kanban_db_query_duration_seconds`, `kanban_db_query_errors_total`                                                   | `op`, `target`            |

It also exports the standard Go runtime and process metrics.

`route` is the URL pattern, such as `/api/tickets/{id}`, so IDs don't create new series. The git `operation` label is one of:

- `worktree_add` and `worktree_remove`: creating and removing ticket worktrees.
- `diff`: building the ticket's diff.
- `fetch`: fetching the base branch before creating a worktree.

For SQLite, `op` is the SQL verb and `target` is the table.

## Prometheus with Docker Compose

The repo's `compose.yaml` includes a Prometheus service that scrapes kanban every 15 seconds and keeps its data in a named volume. Kanban proxies its UI at <http://localhost:7474/prometheus/>. Without `$KANBAN_PROMETHEUS_URL` set, that page returns 503, but `/metrics` still works.

The config lives in `deploy/prometheus.yml` and `deploy/recording_rules.yml`. After editing either, run `docker compose restart prometheus`.

The recording rules precompute a few common queries:

| Metric                                               | Value                              |
| ---------------------------------------------------- | ---------------------------------- |
| `kanban:http_request_duration_seconds:p95_5m`        | p95 latency by `method`, `route`   |
| `kanban:http_request_duration_seconds:mean_5m`       | Mean latency by `method`, `route`  |
| `kanban:db_query_duration_seconds:p99_5m`            | p99 latency by `op`, `target`      |
| `kanban:db_query_duration_seconds:mean_5m`           | Mean latency by `op`, `target`     |
| `kanban:github_api_requests:rate_5m`                 | Request rate by `method`, `endpoint` |
| `kanban:github_api_request_duration_seconds:mean_5m` | Mean latency by `method`, `endpoint` |

For example, the ten slowest queries:

```promql
topk(10, kanban:db_query_duration_seconds:p99_5m)
```

## GitHub rate limits

Two things call GitHub: the pull request poller (every 30 seconds) and [Build Cop](./configuration#build-cop). To see which endpoints use the most requests:

```promql
sum by (endpoint) (rate(kanban_github_api_requests_total[5m]))
```

And how many requests remain this hour:

```promql
kanban_github_rate_limit_remaining{resource="core"}
```

Build Cop usually uses the most. Raise `[buildcop].interval` to slow it down.

## Slow git operations

Average time per git operation:

```promql
sum by (operation) (rate(kanban_git_command_duration_seconds_sum[5m]))
  / sum by (operation) (rate(kanban_git_command_duration_seconds_count[5m]))
```

p95 time for the diff view:

```promql
histogram_quantile(0.95,
  sum by (le) (rate(kanban_git_command_duration_seconds_bucket{operation="diff"}[5m])))
```

`fetch` gives up after 10 seconds, and the worktree is created from the local branch instead. Errors there usually mean a slow or unreachable remote.

## Frontend errors

The web UI reports uncaught errors and freezes of 2 seconds or more to the server, which logs them. To find them:

```sh
docker compose logs kanban | grep "client error"
```
