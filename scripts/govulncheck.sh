#!/usr/bin/env bash
# govulncheck wrapper that ignores a documented allowlist of advisories
# upstream has no fix for. Fails on any other finding.
#
# Each ignored ID must be paired with a comment: why it's safe to ignore,
# and what would prompt us to drop the entry (typically: upstream fix lands,
# or we migrate off the affected dep). Re-evaluate every time this list
# changes — never add an entry without an upstream link.
set -euo pipefail

# Allowlist: govulncheck OSV IDs we accept until upstream patches.
IGNORE=(
  # GO-2026-4887: Moby AuthZ plugin bypass on oversized request bodies.
  # https://pkg.go.dev/vuln/GO-2026-4887 — docker/docker: "Fixed in: N/A".
  # We use docker/docker only as a client; AuthZ plugin code paths are
  # unreachable at runtime (only pulled in via package init()). Drop when
  # docker/docker publishes a fix or we move to github.com/moby/moby/v2.
  "GO-2026-4887"
  # GO-2026-4883: Moby vulnerability with no current fix in docker/docker.
  # https://pkg.go.dev/vuln/GO-2026-4883 — same reasoning as above.
  "GO-2026-4883"
  # GO-2026-5746: `PUT /containers/{id}/archive` executes container binary
  # on the host. https://pkg.go.dev/vuln/GO-2026-5746 — docker/docker:
  # "Fixed in: N/A". The vulnerable handler is dockerd's archive endpoint
  # (daemon-side); we only use docker/docker as an API client against the
  # daemon, so it's unreachable here (flagged only via transitive package
  # init()). Drop when docker/docker publishes a fix or we move to
  # github.com/moby/moby/v2.
  "GO-2026-5746"
  # GO-2026-5668: docker cp race allows arbitrary empty file creation on
  # the host via symlink swap. https://pkg.go.dev/vuln/GO-2026-5668 — same
  # reasoning as GO-2026-5746 (daemon-side handler, we're client-only).
  "GO-2026-5668"
  # GO-2026-5617: docker cp race allows bind mount redirection to a host
  # path. https://pkg.go.dev/vuln/GO-2026-5617 — same reasoning as
  # GO-2026-5746 (daemon-side handler, we're client-only).
  "GO-2026-5617"
)

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
govulncheck -format json ./... >"$tmp" 2>/dev/null || true

python3 - "$tmp" "${IGNORE[@]}" <<'PY'
import json, sys

with open(sys.argv[1]) as f:
    raw = f.read()
ignore = set(sys.argv[2:])

# govulncheck -format json emits a stream of concatenated JSON objects.
decoder = json.JSONDecoder()
i, n = 0, len(raw)
findings = []
osvs = {}
while i < n:
    while i < n and raw[i].isspace():
        i += 1
    if i >= n:
        break
    obj, end = decoder.raw_decode(raw, i)
    i = end
    if "osv" in obj and isinstance(obj["osv"], dict):
        osvs[obj["osv"]["id"]] = obj["osv"]
    if "finding" in obj:
        findings.append(obj["finding"])

# Group findings by OSV id; only fail on symbol-level findings (actual call
# graph hits), matching govulncheck's own exit-code-3 trigger.
by_id = {}
for f in findings:
    osv_id = f.get("osv")
    if not osv_id:
        continue
    trace = f.get("trace") or []
    if not trace:
        continue
    # Symbol-level findings have a "function" in the leaf trace entry.
    if trace[0].get("function"):
        by_id.setdefault(osv_id, []).append(f)

unexpected = {k: v for k, v in by_id.items() if k not in ignore}
ignored_hit = {k: v for k, v in by_id.items() if k in ignore}

for osv_id, fs in sorted(ignored_hit.items()):
    o = osvs.get(osv_id, {})
    print(f"ignoring {osv_id}: {o.get('summary', '')}".rstrip())

if unexpected:
    print()
    print(f"govulncheck: {len(unexpected)} unignored vulnerabilit"
          f"{'y' if len(unexpected) == 1 else 'ies'} found:")
    for osv_id, fs in sorted(unexpected.items()):
        o = osvs.get(osv_id, {})
        print(f"  {osv_id}: {o.get('summary', '')}".rstrip())
        print(f"    https://pkg.go.dev/vuln/{osv_id}")
    sys.exit(1)
PY
