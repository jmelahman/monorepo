#!/usr/bin/env python3
"""Triage the open Dependabot PRs of the current repository.

    triage.py            print every PR with its bucket, then the plan
    triage.py --log PR   print the failing hooks' output for one PR
    triage.py --master   print the latest run of each workflow on master

Everything here is read-only: it calls `gh` and prints. The buckets and the
wave are the mechanical part of the dependabot skill, kept in code so they
come out the same every time.
"""

from __future__ import annotations

import json
import re
import subprocess
import sys

# gh prints the escape byte as a literal "^[" when its output is piped.
ANSI = re.compile(r"(?:\x1b|\^\[)\[[0-9;]*m")
# "<job>\t<step>\t<timestamp> <text>" as printed by `gh run view --log`.
LOG_PREFIX = re.compile(r"^[^\t]*\t[^\t]*\t\S+Z ?")
NOISE = re.compile(r"^\S+Z\s+(TRACE|DEBUG|INFO) ")
VERSIONS = re.compile(r" from (\S+) to (\S+)")
# One line per dependency in the body of a grouped PR.
GROUPED = re.compile(r"^Updates `[^`]+` from (\S+) to (\S+)", re.MULTILINE)
# Worst first.
JUMPS = ["unknown", "major", "0.x-minor", "minor", "patch"]


def gh(*args: str) -> str:
    return subprocess.run(  # noqa: S603
        ["gh", *args], check=True, capture_output=True, text=True
    ).stdout


def jump(pr: dict) -> str:
    """The worst semver distance among the bumps a PR makes."""
    pairs = VERSIONS.findall(pr["title"]) or GROUPED.findall(pr["body"])
    # Digest and action-SHA bumps name no versions anywhere.
    found = [distance(old, new) for old, new in pairs] or ["unknown"]
    return min(found, key=JUMPS.index)


def distance(old_version: str, new_version: str) -> str:
    old, new = (re.findall(r"\d+", v)[:3] for v in (old_version, new_version))
    if not old or not new:
        return "unknown"
    if old[0] != new[0]:
        return "major"
    if old[1:2] != new[1:2]:
        return "0.x-minor" if old[0] == "0" else "minor"
    return "patch"


def state(pr: dict) -> str:
    if pr["mergeable"] == "CONFLICTING":
        return "conflict"
    results = {c.get("conclusion") or c.get("status") for c in pr["statusCheckRollup"]}
    if results & {"FAILURE", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED"}:
        return "red"
    if not results or results - {"SUCCESS", "SKIPPED", "NEUTRAL"}:
        return "pending"
    return "green"


def failed_log(pr: dict) -> list[str]:
    """The failing hooks' own output, without prek's trace."""
    lines: list[str] = []
    for check in pr["statusCheckRollup"]:
        m = re.search(r"/runs/(\d+)", check.get("detailsUrl") or "")
        if check.get("conclusion") != "FAILURE" or not m:
            continue
        keep = False
        for raw in gh("run", "view", m.group(1), "--log-failed").splitlines():
            line = ANSI.sub("", LOG_PREFIX.sub("", raw))
            if "Prek verbose logs" in line:
                keep = False
            elif re.search(r"\.{3,}Failed$", line):
                keep = True
            if keep and not NOISE.match(line):
                lines.append(line.rstrip())
    return lines


def hooks(log: list[str]) -> str:
    ids = [m.group(1) for line in log if (m := re.search(r"- hook id: (\S+)", line))]
    return ",".join(dict.fromkeys(ids)) or "see --log"


def bump(title: str) -> str:
    return re.sub(r" in /.*", "", re.sub(r"^Bump (the )?", "", title))


def master_runs() -> None:
    runs = json.loads(
        gh(
            "run",
            "list",
            "--branch",
            "master",
            "--limit",
            "60",
            "--json",
            "workflowName,status,conclusion,headSha,displayTitle",
        )
    )
    seen: set[str] = set()
    for run in runs:
        name = run["workflowName"]
        # GitHub's own dependency jobs say nothing about the tree.
        if name in ("Dependabot Updates", "Dependency Graph") or name in seen:
            continue
        seen.add(name)
        result = (run["conclusion"] or run["status"]).lower()
        print(f"{name:14}{result:13}{run['headSha'][:8]}  {run['displayTitle'][:60]}")


def open_prs() -> list[dict]:
    prs = json.loads(
        gh(
            "pr",
            "list",
            "--author",
            "app/dependabot",
            "--limit",
            "100",
            "--json",
            "number,title,body,mergeable,statusCheckRollup,files",
        )
    )
    return sorted(prs, key=lambda p: p["number"])


def nums(prs: list[dict]) -> str:
    return " ".join(str(p["number"]) for p in prs) or "-"


def plan(prs: list[dict]) -> None:
    for pr in prs:
        pr["state"] = state(pr)
        pr["jump"] = jump(pr)
        pr["paths"] = {f["path"] for f in pr["files"]}
        pr["why"] = hooks(failed_log(pr)) if pr["state"] == "red" else ""

    rows = [
        (
            f"#{p['number']}",
            p["state"],
            p["jump"],
            p["why"],
            bump(p["title"]),
            " ".join(sorted(p["paths"])),
        )
        for p in sorted(prs, key=lambda p: (bump(p["title"]), p["number"]))
    ]
    widths = [max(len(r[i]) for r in rows) for i in range(5)] if rows else []
    for r in rows:
        print("  ".join(c.ljust(w) for c, w in zip(r[:5], widths, strict=True)), r[5], sep="  ")

    # At most one PR per file per wave: once a PR merges, any other PR
    # touching the same file was checked against a base that is gone.
    wave: list[dict] = []
    later: list[dict] = []
    ask: list[dict] = []
    taken: set[str] = set()
    for pr in prs:
        if pr["state"] != "green":
            continue
        if pr["jump"] not in ("patch", "minor"):
            ask.append(pr)
        elif pr["paths"] & taken:
            later.append(pr)
        else:
            wave.append(pr)
            taken |= pr["paths"]

    print()
    print("MERGE NOW  :", nums(wave))
    print("NEXT WAVE  :", nums(later), "(rebase after this wave merges)")
    print("ASK FIRST  :", nums(ask), "(major, 0.x minor, or unknown jump)")
    for label in ("red", "conflict", "pending"):
        print(f"{label.upper():11}:", nums([p for p in prs if p["state"] == label]))


def main() -> None:
    if sys.argv[1:2] == ["--master"]:
        master_runs()
    elif sys.argv[1:2] == ["--log"]:
        want = int(sys.argv[2])
        pr = next(p for p in open_prs() if p["number"] == want)
        print("\n".join(failed_log(pr)) or "no failed check on this PR")
    else:
        plan(open_prs())


if __name__ == "__main__":
    main()
