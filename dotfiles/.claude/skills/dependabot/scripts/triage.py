#!/usr/bin/env python3
"""Triage the open Dependabot PRs of the current repository.

    triage.py                  print every PR with its bucket, then the plan
    triage.py --log PR         print the failing hooks' output for one PR
    triage.py --log master     the same, for the last commit on master
    triage.py --master         print each workflow's run on that commit
    triage.py --master --wait  the same, once those runs have finished

Everything here is read-only: it calls `gh` and prints. The buckets and the
wave are the mechanical part of the dependabot skill, kept in code so they
come out the same every time.
"""

from __future__ import annotations

import json
import re
import subprocess
import sys
import time

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
# GitHub's own dependency jobs say nothing about the tree.
NOT_CHECKS = ("Dependabot Updates", "Dependency Graph")
# How long --wait polls before it prints what it has, in seconds. It stays
# under the ten minutes a single command is allowed.
WAIT = 540
POLL = 20
UNFINISHED = ("queued", "in_progress", "waiting", "pending", "requested")


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


def failed_runs(pr: dict) -> list[str]:
    ids = []
    for check in pr["statusCheckRollup"]:
        m = re.search(r"/runs/(\d+)", check.get("detailsUrl") or "")
        if check.get("conclusion") == "FAILURE" and m:
            ids.append(m.group(1))
    return list(dict.fromkeys(ids))


def failed_log(run_ids: list[str]) -> list[str]:
    """The failing hooks' own output, without prek's trace."""
    lines: list[str] = []
    for run_id in run_ids:
        keep = False
        kept: list[str] = []
        rest: list[str] = []
        for raw in gh("run", "view", run_id, "--log-failed").splitlines():
            line = ANSI.sub("", LOG_PREFIX.sub("", raw))
            if "Prek verbose logs" in line:
                keep = False
            elif re.search(r"\.{3,}Failed$", line):
                keep = True
            if NOISE.match(line):
                continue
            (kept if keep else rest).append(line.rstrip())
        # A job that isn't prek, like the mirror, has no hook to cut out.
        lines += kept or rest[-30:]
    return lines


def hooks(log: list[str]) -> str:
    ids = [m.group(1) for line in log if (m := re.search(r"- hook id: (\S+)", line))]
    return ",".join(dict.fromkeys(ids)) or "see --log"


def bump(title: str) -> str:
    return re.sub(r" in /.*", "", re.sub(r"^Bump (the )?", "", title))


def master_runs() -> list[dict]:
    """One row per workflow for the last commit on master."""
    sha = gh("api", "repos/{owner}/{repo}/commits/master", "--jq", ".sha").strip()
    fields = "databaseId,workflowName,status,conclusion"
    runs = json.loads(gh("run", "list", "--commit", sha, "--limit", "50", "--json", fields))
    latest: dict[str, dict] = {}
    for run in runs:
        latest.setdefault(run["workflowName"], run)
    names = gh("workflow", "list", "--json", "name", "--jq", ".[].name").split("\n")
    rows = []
    for name in sorted(n for n in names if n and n not in NOT_CHECKS):
        run = latest.get(name)
        rows.append(
            {
                "name": name,
                "sha": sha,
                # A workflow with a path filter starts no run for most commits.
                "result": (run["conclusion"] or run["status"]).lower() if run else "not triggered",
                "id": str(run["databaseId"]) if run else "",
            }
        )
    return rows


def master(*, wait: bool) -> None:
    deadline = time.monotonic() + WAIT
    while True:
        rows = master_runs()
        # Runs take a few seconds to appear after a push.
        started = any(r["id"] for r in rows)
        running = [r for r in rows if r["result"] in UNFINISHED]
        if not wait or (started and not running) or time.monotonic() > deadline:
            break
        time.sleep(POLL)

    print("master", rows[0]["sha"][:8] if rows else "")
    for r in rows:
        why = ""
        if r["result"] == "failure":
            why = "hook: " + hooks(failed_log([r["id"]]))
        print(f"{r['name']:14}{r['result']:15}{r['id']:13}{why}".rstrip())
    if wait and (running or not started):
        print("still running: run this command again")


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
        pr["why"] = hooks(failed_log(failed_runs(pr))) if pr["state"] == "red" else ""

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
        master(wait="--wait" in sys.argv)
    elif sys.argv[1:3] == ["--log", "master"]:
        ids = [r["id"] for r in master_runs() if r["result"] == "failure"]
        print("\n".join(failed_log(ids)) or "no failed run on this commit")
    elif sys.argv[1:2] == ["--log"]:
        want = int(sys.argv[2])
        pr = next(p for p in open_prs() if p["number"] == want)
        print("\n".join(failed_log(failed_runs(pr))) or "no failed check on this PR")
    else:
        plan(open_prs())


if __name__ == "__main__":
    main()
