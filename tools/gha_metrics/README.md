# CI Metrics Tool

A tool to collect and analyze CI metrics from GitHub Actions workflows, tracking build times, wait times, failure rates, retries, and throughput.

## Build

```bash
go build .
```

## Usage

```bash
./gha_metrics -owner <org> -repo <repo> [-csv path]
```

### Options

- `-owner`: GitHub organization or username (required)
- `-repo`: Repository name (required)
- `-token`: GitHub token (optional, defaults to `GITHUB_TOKEN` env var or `gh auth token`)
- `-csv`: Optional path to export data as CSV

### Examples

```bash
# Basic usage
./gha_metrics -owner onyx-dot-app -repo onyx

# With CSV export
./gha_metrics -owner onyx-dot-app -repo onyx -csv metrics.csv

# With explicit token
./gha_metrics -owner onyx-dot-app -repo onyx -token $GITHUB_TOKEN
```

## Metrics Collected

- **Overall metrics**: Build times, wait times, success/failure rates, retry counts
- **Per-job metrics**: Build times, wait times, and failure rates for each job
- **Weekly throughput**: Number of builds per week

