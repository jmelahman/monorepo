#!/usr/bin/env python3
"""Glue between an agent's security review and GitHub code scanning.

  alerts    open alerts in the files a diff touches, for the prompt
  normalize OpenCode's NDJSON stream into the same result as Claude Code
  sarif     the agent's findings plus the open alerts it didn't revisit, as SARIF
  usage     the run's token usage, as a step summary and usage.json

Code scanning closes every open alert of a tool that a new analysis leaves
out. A review sees only the diff, so `sarif` carries the open alerts forward:
an alert closes only when the agent says the diff fixed it, or its file was
deleted. Each provider reviews under its own tool name, so one provider's
upload can never close or carry the other's alerts.
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

MIN_CONFIDENCE = 0.8
# SARIF level and code scanning's security-severity score for each severity.
LEVELS = {"HIGH": ("error", "8.0"), "MEDIUM": ("warning", "5.5"), "LOW": ("note", "3.0")}
ALERT_SEVERITY = {"critical": "HIGH", "high": "HIGH", "medium": "MEDIUM", "low": "LOW"}
# Agent credentials: Claude Code OAuth tokens (sk-ant-...) and OpenCode
# Console/Go API keys (oc_sk_...). The findings are public, so none may end up
# there.
SECRET = re.compile(r"sk-ant-[A-Za-z0-9_-]{8,}|oc_sk_[A-Za-z0-9_-]{8,}")
RULE_ID = re.compile(r"^[a-z0-9]+(-[a-z0-9]+)*$")
CWE = re.compile(r"^CWE-[0-9]+$")
# The finding fields schema.json requires, and their Python types.
FINDING_FIELDS = {
    "rule_id": str,
    "title": str,
    "severity": str,
    "confidence": (int, float),
    "file": str,
    "line": int,
    "description": str,
    "exploit_scenario": str,
    "recommendation": str,
}


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


def fingerprint(tool: str, rule_id: str, path: str, title: str) -> str:
    return hashlib.sha256(f"{tool}\0{rule_id}\0{path}\0{title}".encode()).hexdigest()


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


def result(
    tool: str, rule_id: str, severity: str, location: dict[str, Any], text: str
) -> dict[str, Any]:
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
        "partialFingerprints": {f"{tool}/v1": fingerprint(tool, rule_id, path, title)},
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
        results.append(result(args.tool, f["rule_id"], f["severity"], location, text))
    seen = {r["partialFingerprints"][f"{args.tool}/v1"] for r in results}
    carried = 0
    for a in load(args.alerts):
        loc = a["most_recent_instance"]["location"]
        if a["number"] in resolved or loc["path"] in deleted:
            continue
        severity = ALERT_SEVERITY.get(a["rule"].get("security_severity_level") or "", "MEDIUM")
        r = result(
            args.tool, a["rule"]["id"], severity, loc, a["most_recent_instance"]["message"]["text"]
        )
        if r["partialFingerprints"][f"{args.tool}/v1"] not in seen:
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
                        "name": args.tool,
                        "informationUri": "https://github.com/jmelahman/agentic-security-review",
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
        sys.exit("refusing to upload: the findings contain an agent credential")
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
            "Cost is API-equivalent: flat on a Claude subscription, per token with OpenCode.",
            "",
        ]
    )
    if path := os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(path, "a") as f:
            f.write(summary)
    else:
        print(summary)


def extract_stream(events: str) -> tuple[str, dict[str, int], float, int, str | None, int | None]:
    """The final text, usage, and any error from `opencode run --format json` NDJSON."""
    text_parts: list[str] = []
    tokens = {"input": 0, "output": 0, "cache_read": 0, "cache_write": 0}
    cost = 0.0
    turns = 0
    error: str | None = None
    timestamps: list[int] = []
    for line in Path(events).read_text().splitlines():
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(event.get("timestamp"), (int, float)):
            timestamps.append(int(event["timestamp"]))
        if event.get("type") == "error":
            err = event.get("error") or {}
            error = (
                err.get("message")
                or err.get("name")
                or (err.get("data") or {}).get("message")
                or "unknown error"
            )
        part = event.get("part") or {}
        if event.get("type") == "text" or part.get("type") == "text":
            if part.get("text"):
                text_parts.append(part["text"])
        elif event.get("type") == "step_finish" or part.get("type") == "step-finish":
            turns += 1
            t = part.get("tokens") or {}
            tokens["input"] += int(t.get("input") or 0)
            tokens["output"] += int(t.get("output") or 0)
            cache = t.get("cache") or {}
            tokens["cache_read"] += int(cache.get("read") or 0)
            tokens["cache_write"] += int(cache.get("write") or 0)
            if isinstance(part.get("cost"), (int, float)):
                cost += part["cost"]
    duration = (max(timestamps) - min(timestamps)) if timestamps else None
    return "".join(text_parts), tokens, cost, turns, error, duration


def extract_json(text: str) -> dict[str, Any]:
    """The JSON object the model was asked for, fences or prose notwithstanding."""
    text = text.strip()
    if text.startswith("```"):
        text = text.lstrip("`")
        if text.startswith("json"):
            text = text[4:].lstrip()
        text = text.removesuffix("```")
        text = text.strip()
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        pass
    start, end = text.find("{"), text.rfind("}")
    if start != -1 and end > start:
        return json.loads(text[start : end + 1])
    raise ValueError("no JSON object in the model's output")


def valid_finding(finding: Any) -> bool:
    """Whether a finding matches the schema.json contract closely enough to keep."""
    if not isinstance(finding, dict):
        return False
    for key, typ in FINDING_FIELDS.items():
        value = finding.get(key)
        if isinstance(value, bool) or not isinstance(value, typ):
            return False
    end_line_ok = "end_line" not in finding or (
        isinstance(finding["end_line"], int)
        and not isinstance(finding["end_line"], bool)
        and finding["end_line"] >= 1
    )
    cwe_ok = "cwe" not in finding or (
        isinstance(finding["cwe"], str) and CWE.fullmatch(finding["cwe"]) is not None
    )
    return (
        RULE_ID.fullmatch(finding["rule_id"]) is not None
        and finding["severity"] in LEVELS
        and 0 <= finding["confidence"] <= 1
        and finding["line"] >= 1
        and end_line_ok
        and cwe_ok
    )


def normalize(args: argparse.Namespace) -> None:
    """Turn OpenCode's stream into the same result.json Claude Code produces."""
    text, tokens, cost, turns, error, duration = extract_stream(args.events)
    if error:
        sys.exit(f"OpenCode reported an error: {error}")
    try:
        payload = extract_json(text)
    except (json.JSONDecodeError, ValueError) as exc:
        sys.exit(f"could not parse the model's output as JSON: {exc}")
    if not isinstance(payload, dict):
        sys.exit("the model's output is not a JSON object")
    findings = payload.get("findings")
    resolved = payload.get("resolved_alerts")
    if not isinstance(findings, list) or not isinstance(resolved, list):
        sys.exit("the model's output lacks `findings` or `resolved_alerts` lists")
    for finding in findings:
        if not valid_finding(finding):
            sys.exit(f"the model returned a malformed finding: {finding!r}")
    if any(not isinstance(n, int) or isinstance(n, bool) for n in resolved):
        sys.exit("the model returned a non-integer alert number")

    result_json = {
        "is_error": False,
        "structured_output": {"findings": findings, "resolved_alerts": resolved},
        "usage": {
            "input_tokens": tokens["input"],
            "cache_creation_input_tokens": tokens["cache_write"],
            "cache_read_input_tokens": tokens["cache_read"],
            "output_tokens": tokens["output"],
        },
        "num_turns": turns,
        "duration_ms": duration,
        "total_cost_usd": cost,
        "modelUsage": {
            args.model: {
                "inputTokens": tokens["input"],
                "cacheCreationInputTokens": tokens["cache_write"],
                "cacheReadInputTokens": tokens["cache_read"],
                "outputTokens": tokens["output"],
                "costUSD": cost,
            }
        },
    }
    Path(args.out).write_text(json.dumps(result_json, indent=2))
    print(f"{len(findings)} findings, {len(resolved)} resolved alerts")


def main() -> None:
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    sub = parser.add_subparsers(required=True)

    p = sub.add_parser("alerts")
    p.add_argument("alerts")
    p.add_argument("changed")
    p.set_defaults(func=alerts)

    p = sub.add_parser("normalize")
    p.add_argument("--events", required=True)
    p.add_argument("--model", required=True)
    p.add_argument("--out", required=True)
    p.set_defaults(func=normalize)

    p = sub.add_parser("sarif")
    p.add_argument("--result", required=True)
    p.add_argument("--alerts", required=True)
    p.add_argument("--shown", required=True)
    p.add_argument("--deleted", required=True)
    p.add_argument("--tool", required=True)
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
