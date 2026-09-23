---
name: security-audit
description: Comprehensive security audit of the Agentic Kanban repo. Posts every finding as a ticket on the kanban "Security" board via the kanban MCP, auto-creating the board on first run and diffing against existing tickets on subsequent runs so resolved findings move to Done.
disable-model-invocation: true
---

# Security Audit — Agentic Kanban

You perform a security audit of this repository and post every finding as a
ticket on the `Security` kanban board via the `mcp__kanban__*` tools. On
re-runs, diff against existing tickets: resolved findings move to Done with
a resolution note, regressed findings get re-opened, and net-new findings
get added to Backlog. **Never delete a ticket** — Done is the audit trail.

**Scope:** `$ARGUMENTS` (default: `full` — all categories). Restrictions:
`go`, `frontend`, `docker`, `sessions`, `mcp`, `api`, `auth`, `dependencies`,
`config`, `network`, `ci`.

**Language:** parse `$ARGUMENTS` for `lang=XX` (e.g. `lang=de`). Default
English. The `lang` parameter can combine with a scope, e.g.
`/security-audit docker lang=de`.

---

## Phase 0 — Bootstrap the Security board

1. Call `mcp__kanban__list_boards`. If a board whose `name` or `slug`
   equals `Security` / `security` (case-insensitive) exists, capture its
   `id` and `slug`. Otherwise call `mcp__kanban__create_board` with:
   - `name`: `"Security"`
   - `repo_path`: `"/workspace"`

   Leave `base_branch`, `branch_prefix`, `worktree_root`,
   `git_author_*` unset. The server seeds columns `Backlog`,
   `In Progress`, `Review`, `Done`.
2. Call `mcp__kanban__board_state` with the captured `id` or slug to
   snapshot existing tickets (id, title, body, column). You will diff
   against this snapshot in Phase 2.

---

## Phase 1 — Analysis

Examine the codebase across the categories below. **Stay inside
`/workspace`** — outbound HTTP is firewalled (see CLAUDE.md "Network
notes"), and pasting code into external tools risks exfiltrating a
private repo. Read statically; do not start `wgo run . serve` to "test"
findings. If a runtime check is genuinely required, stop and ask the
user.

### 1.1 Categories tailored to this repo

| Category | What to check | Where to look |
|----------|---------------|---------------|
| **Go source** | SQL injection in `database/sql` (look for `fmt.Sprintf` + query), command injection in `os/exec`, SSRF in HTTP clients, path traversal in worktree/repo-path handling, missing `context` timeouts, the `sessions` row multi-writer race documented in CLAUDE.md, unchecked errors on auth paths, hardcoded secrets | `main.go`, `cmd/server/`, `internal/{api,db,docker,git,github,harness,mcp,session,tasks}/` |
| **Frontend** | `dangerouslySetInnerHTML`, `eval`/`new Function`, secret leakage via console or SSE payloads, missing CSP, unsafe URL construction, tokens in `localStorage` | `web/src/`, especially `web/src/api/`, `web/src/storage.ts`, `web/src/components/` |
| **HTTP API** | Missing auth on state-changing `/api/*` routes, missing CSRF, missing rate limits, permissive CORS, SSE auth, request body size limits, JSON depth limits, `Host`-header trust | `internal/api/`, `internal/server*`, look for `http.HandleFunc` / router registrations |
| **MCP server** | Are `mcp__kanban__*` tools authenticated at the transport? Do they validate `repo_path` and worktree paths against an allowlist? Can a malicious agent escape the intended board? | `internal/mcp/`, `.mcp.json` |
| **Docker / session containers** | Root inside session containers, missing `--cap-drop`, missing `--security-opt no-new-privileges`, missing read-only rootfs, forwarding `~/.claude` (secret material) into untrusted sessions, host-path leakage via `KANBAN_HOST_WORKSPACE` / `KANBAN_HOST_HOME`, docker socket exposure | `internal/docker/`, `internal/session/`, `.devcontainer/`, `Dockerfile` |
| **CI/CD** | Secret exposure in workflow logs, `pull_request_target` misuse, third-party actions pinned by tag instead of SHA, missing image scans, missing SAST | `.github/workflows/{docs,release,test}.yml` |
| **Dependencies** | Known CVEs in `go.sum` / `web/package-lock.json`; abandoned packages; suspicious transitive pulls | `go.mod`, `go.sum`, `web/package.json`, `web/package-lock.json` |
| **Configuration** | Default-on insecure flags (`--claude-config` forwarding into untrusted sessions, in-memory mode discarding audit), TLS enforcement, `.kanban.toml` defaults, env var handling for `KANBAN_*` | `.kanban.toml`, `cmd/server/`, `internal/config/`, `internal/kanbantoml/`, `docs/guide/configuration.md` |
| **Network** | Unbounded read/write timeouts on `http.Server`, DNS rebinding on `Host`-trusting endpoints, firewall allowlist drift, body size limits | `.devcontainer/` firewall config, HTTP server setup in `internal/server*` or `internal/api/` |

### 1.2 Procedure

1. Read `CLAUDE.md`, top-level `README.md` (if present), `go.mod`, and
   `web/package.json` for orientation.
2. Recursively grep `internal/`, `cmd/`, `main.go` for risk patterns:
   - SQL: `db\.(Query|Exec).*fmt\.Sprintf`, `+` string concatenation in
     SQL strings
   - Command injection: `exec\.Command(.*\+`, `exec\.CommandContext`
     with user-controlled args
   - SSRF: `http\.(Get|Post|Do)` on URLs derived from request input
   - Path traversal: `filepath\.Join` with un-cleaned user input,
     `os\.Open(F|file)` on user-controlled paths
3. Recursively grep `web/src/` for `dangerouslySetInnerHTML`, `eval(`,
   `new Function`, `localStorage\.setItem`, `window\.location =`,
   `innerHTML\s*=`.
4. Read `Dockerfile`, every file under `.devcontainer/`, and any
   `docker-compose*.yml`.
5. Read every file in `.github/workflows/`.
6. Skim `web/package-lock.json` and `go.sum` for known-bad versions of
   common packages (e.g. `golang.org/x/net` < patched, `lodash`, etc.).
   Don't fetch CVE feeds — flag suspicious versions for the user to
   verify and link the package name + version in the finding.

### 1.3 Finding classification

| Severity | Criteria | ID prefix |
|----------|----------|-----------|
| **CRITICAL** | Directly exploitable RCE, credential theft, container/host escape, `~/.claude` exposure to untrusted code | `C1`, `C2`, … |
| **HIGH** | Exploitable with preconditions, data loss possible, auth bypass, session-container escape with preconditions | `H1`, `H2`, … |
| **MEDIUM** | Defense-in-depth gap, best-practice violation with concrete risk | `M1`, `M2`, … |
| **LOW** | Hardening only, minimal direct impact | `L1`, `L2`, … |

Each finding records:

- **ID** (stable across runs — reuse if the finding already has a ticket)
- **Title** — short, imperative
- **File and line** (e.g. `internal/api/sessions.go:142`)
- **Description** — what's wrong, why it matters *in this codebase*
- **Current code** excerpt
- **Recommended fix** — concrete code or step list
- **OWASP Top 10** mapping (if applicable, e.g. `A01:2021 – Broken Access Control`)
- **CWE** number (if applicable, e.g. `CWE-89`)
- **Acceptance criteria** — testable

---

## Phase 2 — Diff against existing tickets

Walk the Phase 0 board snapshot. Every ticket the skill creates carries a
`Finding: <ID>` marker on the first line of its body — use it as the
diff anchor. For each existing ticket:

- **Fixed** (finding no longer reproduces) — call
  `mcp__kanban__update_ticket` to prepend
  `✓ RESOLVED (<YYYY-MM-DD>) — <one-line resolution>` to the body, then
  call `mcp__kanban__done_ticket` to move it to Done. **Never delete.**
- **Open and unchanged** — leave it alone.
- **Worsened or scope changed** — `mcp__kanban__update_ticket` with a
  refreshed body and a `**Re-evaluation (<YYYY-MM-DD>):**` section
  describing what changed. If severity changed, update the title prefix
  (`[CRITICAL]` / `[HIGH]` / `[MEDIUM]` / `[LOW]`); the ticket stays in
  Backlog.
- **New** — create a fresh ticket in Phase 3.

Allocate new IDs by scanning existing tickets for the highest used
`C<n>`/`H<n>`/`M<n>`/`L<n>` per severity and incrementing. IDs are
permanent — even if a finding is resolved and a regression brings it
back, reuse the original ID.

---

## Phase 3 — Post tickets

For each new finding, call `mcp__kanban__create_ticket` with:

- **board**: the `slug` returned by Phase 0 (typically `"security"`)
- **column**: omit — defaults to Backlog
- **title**: `[CRITICAL|HIGH|MEDIUM|LOW] <ID> — <short title>`
  e.g. `[HIGH] H3 — /api/sessions/{id}/exec has no auth check`
- **body**: the markdown template below

For each worsened/changed existing finding, call
`mcp__kanban__update_ticket` with the same body template (preserving the
ticket's original `Finding: <ID>` line and adding the
`**Re-evaluation (<date>):**` block under Description).

### Ticket body template

````markdown
Finding: <ID>
Severity: <CRITICAL|HIGH|MEDIUM|LOW>
File: `<path>:<lines>`
OWASP: <A0X:2021 – Name>   <!-- or `n/a` -->
CWE: CWE-<XXX>             <!-- or `n/a` -->

## Description

<what's wrong and why it matters in this codebase — reference the
relevant CLAUDE.md note if applicable>

## Current code

```<language>
<excerpt>
```

## Recommended fix

<concrete code diff or numbered step list>

## Acceptance criteria

- [ ] <testable criterion 1>
- [ ] <testable criterion 2>
````

The `Finding: <ID>` line **must** be the first line of the body — it's
the diff anchor for future runs.

---

## Phase 4 — Summary

After all kanban writes complete, print a single block to the user:

```
Security audit completed.

Board: Security (id=<n>, slug=<slug>)
- Critical: <open> open, <resolved> resolved this run
- High:     <open> open, <resolved> resolved this run
- Medium:   <open> open, <resolved> resolved this run
- Low:      <open> open, <resolved> resolved this run
- New this run:          <n>
- Re-evaluated this run: <n>

Score: <A|B+|B|C|D> (<score>/100)
  Score = 100 − (Critical × 20) − (High × 10) − (Medium × 4) − (Low × 1)
  clamped 0–100. Counts include all currently-open tickets on the board.
```

Score → rating:

- 90–100 → **A**
- 75–89  → **B+**
- 60–74  → **B**
- 40–59  → **C**
-  0–39  → **D**

Also call out **positive findings** (e.g. "input validation present in
`internal/api/board.go`", "Biome lint passes on `web/`", "`prek` hook
enforces formatting") in the summary text — do **not** create kanban
tickets for positives. The board is for actionable work only.

---

## Rules

- Stay inside `/workspace`. Don't paste code excerpts into web tools or
  external pastebins.
- Read statically. Don't start the backend or frontend to probe
  findings; if a runtime check is genuinely required, stop and ask.
- Don't run `git` write commands. Don't create commits, branches, or
  worktrees.
- Don't modify source files. The skill's only writes are MCP calls to
  the Security board.
- Resolved tickets stay in Done. They are the audit trail.
- If `mcp__kanban__list_boards` or `mcp__kanban__create_board` fails,
  stop and report — don't fall back to writing markdown files.
