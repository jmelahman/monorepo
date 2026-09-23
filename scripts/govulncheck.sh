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
IGNORE=()

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
