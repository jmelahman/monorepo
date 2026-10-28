#!/usr/bin/env python3
"""Glue between Claude's security review and GitHub code scanning.

  alerts  open alerts in the files a diff touches, for the prompt
  sarif   Claude's findings plus the open alerts it didn't revisit, as SARIF
  usage   the run's token usage, as a step summary and usage.json

Code scanning closes every open alert of a category that a new analysis
leaves out. A review sees only the diff, so `sarif` carries the open alerts
forward: an alert closes only when Claude says the diff fixed it, or its file
was deleted.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import sys
from typing import Any

TOOL = "claude-security-review"
MIN_CONFIDENCE = 0.8
# SARIF level and code scanning's security-severity score for each severity.
LEVELS = {"HIGH": ("error", "8.0"), "MEDIUM": ("warning", "5.5"), "LOW": ("note", "3.0")}
ALERT_SEVERITY = {"critical": "HIGH", "high": "HIGH", "medium": "MEDIUM", "low": "LOW"}
# Claude Code credentials; the findings are public, so none may end up there.
SECRET = re.compile(r"sk-ant-[A-Za-z0-9_-]{8,}")


def load(path: str) -> Any:
    return json.loads(Path(path).read_text())


def lines(path: str) -> set[str]:
    return {line for line in Path(path).read_text().splitlines() if line}


def relative(path: str) -> str | None:
    """`path` relative to the checkout, or None if it isn't a file in it."""
    root = Path.cwd().resolve()
    resolved = (root / path).resolve()
    if not resolved.is_relative_to(root) or not resolved.is_file():
        return None
    return resolved.relative_to(root).as_posix()


def fingerprint(rule_id: str, path: str, title: str) -> str:
    return hashlib.sha256(f"{rule_id}\0{path}\0{title}".encode()).hexdigest()


def alerts(args: argparse.Namespace) -> None:
    changed = lines(args.changed)
    out = [
        {
            "number": a["number"],
            "rule_id": a["rule"]["id"],
            "file": a["most_recent_instance"]["location"]["path"],
            "line": a["most_recent_instance"]["location"]["start_line"],
            "message": a["most_recent_instance"]["message"]["text"],
        }
        for a in load(args.alerts)
        if a["most_recent_instance"]["location"]["path"] in changed
    ]
    print(json.dumps(out, indent=2))


def result(rule_id: str, severity: str, location: dict[str, Any], text: str) -> dict[str, Any]:
    """A SARIF result. `location` is shaped like an alert's: path, start_line, end_line."""
    path, start = location["path"], location["start_line"]
    end = max(start, location.get("end_line") or start)
    title = text.split("\n", 1)[0]
    return {
        "ruleId": rule_id,
        "level": LEVELS[severity][0],
        "message": {"text": text},
        "locations": [
            {
                "physicalLocation": {
                    "artifactLocation": {"uri": path, "uriBaseId": "%SRCROOT%"},
                    "region": {"startLine": start, "endLine": end},
                }
            }
        ],
        "partialFingerprints": {f"{TOOL}/v1": fingerprint(rule_id, path, title)},
        "properties": {"severity": severity},
    }


def sarif(args: argparse.Namespace) -> None:
    review = load(args.result).get("structured_output")
    if not review:
        sys.exit("the review returned no structured output")
    # Only the alerts it was shown can be resolved, not any number it names.
    shown = {a["number"] for a in load(args.shown)}
    resolved = set(review["resolved_alerts"]) & shown
    deleted = lines(args.deleted)

    results: list[dict[str, Any]] = []
    for f in review["findings"]:
        if f["confidence"] < MIN_CONFIDENCE:
            continue
        path = relative(f["file"])
        if path is None:
            print(f"dropping {f['rule_id']}: {f['file']} is not a file in the checkout")
            continue
        text = "\n\n".join(
            [
                f["title"],
                f["description"],
                f"**Exploit:** {f['exploit_scenario']}",
                f"**Fix:** {f['recommendation']}",
                f"Confidence: {f['confidence']:.0%}",
            ]
        )
        location = {"path": path, "start_line": f["line"], "end_line": f.get("end_line")}
        results.append(result(f["rule_id"], f["severity"], location, text))
    seen = {r["partialFingerprints"][f"{TOOL}/v1"] for r in results}
    carried = 0
    for a in load(args.alerts):
        loc = a["most_recent_instance"]["location"]
        if a["number"] in resolved or loc["path"] in deleted:
            continue
        severity = ALERT_SEVERITY.get(a["rule"].get("security_severity_level") or "", "MEDIUM")
        r = result(a["rule"]["id"], severity, loc, a["most_recent_instance"]["message"]["text"])
        if r["partialFingerprints"][f"{TOOL}/v1"] not in seen:
            results.append(r)
            carried += 1

    # A rule's severity is the highest of its results'.
    rules: dict[str, str] = {}
    for r in results:
        current = rules.get(r["ruleId"], "LOW")
        rules[r["ruleId"]] = max(current, r["properties"]["severity"], key=list(LEVELS)[::-1].index)
    doc = {
        "version": "2.1.0",
        "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
        "runs": [
            {
                "tool": {
                    "driver": {
                        "name": TOOL,
                        "informationUri": "https://code.claude.com/docs/en/github-actions",
                        "rules": [
                            {
                                "id": rule_id,
                                "shortDescription": {"text": rule_id.replace("-", " ")},
                                "defaultConfiguration": {"level": LEVELS[severity][0]},
                                "properties": {
                                    "tags": ["security"],
                                    "security-severity": LEVELS[severity][1],
                                },
                            }
                            for rule_id, severity in sorted(rules.items())
                        ],
                    }
                },
                "results": results,
            }
        ],
    }
    out = json.dumps(doc, indent=2)
    if SECRET.search(out):
        sys.exit("refusing to upload: the findings contain a Claude Code credential")
    Path(args.out).write_text(out)
    new = len(results) - carried
    print(f"{new} new or updated, {carried} carried forward, {len(resolved)} resolved")


def usage(args: argparse.Namespace) -> None:
    run = load(args.result)
    u = run.get("usage", {})
    record: dict[str, Any] = {
        "sha": args.sha,
        "base": args.base,
        "num_turns": run.get("num_turns"),
        "duration_ms": run.get("duration_ms"),
        "total_cost_usd": run.get("total_cost_usd"),
        "usage": {
            k: u.get(k)
            for k in (
                "input_tokens",
                "cache_creation_input_tokens",
                "cache_read_input_tokens",
                "output_tokens",
            )
        },
        "model_usage": run.get("modelUsage", {}),
    }
    Path(args.out).write_text(json.dumps(record, indent=2))

    rows = [
        "| Model | Input | Cache write | Cache read | Output | API-equivalent cost |",
        "|---|--:|--:|--:|--:|--:|",
    ]
    for model, m in record["model_usage"].items():
        rows.append(
            f"| {model} | {m.get('inputTokens', 0):,} | {m.get('cacheCreationInputTokens', 0):,}"
            f" | {m.get('cacheReadInputTokens', 0):,} | {m.get('outputTokens', 0):,}"
            f" | ${m.get('costUSD', 0):.2f} |"
        )
    summary = "\n".join(
        [
            f"### Security review of `{args.base[:12]}..{args.sha[:12]}`",
            "",
            *rows,
            "",
            f"{record['num_turns']} turns in {(record['duration_ms'] or 0) / 1000:.0f}s.",
            "On a subscription the cost is what the tokens would cost on the API, not a charge.",
            "",
        ]
    )
    if path := os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(path, "a") as f:
            f.write(summary)
    else:
        print(summary)


def main() -> None:
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    sub = parser.add_subparsers(required=True)

    p = sub.add_parser("alerts")
    p.add_argument("alerts")
    p.add_argument("changed")
    p.set_defaults(func=alerts)

    p = sub.add_parser("sarif")
    p.add_argument("--result", required=True)
    p.add_argument("--alerts", required=True)
    p.add_argument("--shown", required=True)
    p.add_argument("--deleted", required=True)
    p.add_argument("--out", required=True)
    p.set_defaults(func=sarif)

    p = sub.add_parser("usage")
    p.add_argument("--result", required=True)
    p.add_argument("--sha", required=True)
    p.add_argument("--base", required=True)
    p.add_argument("--out", required=True)
    p.set_defaults(func=usage)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
