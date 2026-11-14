"""Runs reproduce.toml's stages: the declared steps that rebuild the world from pinned inputs.

    uv run -q --no-project python -I scripts/reproduce.py [--list] [--dry-run]
        [--from STAGE] [--to STAGE] [--only STAGE,...]

Each stage's commands run in order under `bash -c` in $ROOT, with the file's [env] and the stage's
own env; the first failure stops the run. Python 3.11+, for tomllib.
"""

import argparse
import os
import subprocess
import sys
import tomllib
from pathlib import Path

FILE = Path(__file__).resolve().parent.parent / "reproduce.toml"


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--list", action="store_true", help="name each stage and stop")
    ap.add_argument("--dry-run", action="store_true", help="print the commands, run nothing")
    ap.add_argument("--from", dest="first", help="start at this stage")
    ap.add_argument("--to", dest="last", help="stop after this stage")
    ap.add_argument("--only", help="a comma list of stages to run")
    args = ap.parse_args()

    spec = tomllib.loads(FILE.read_text())
    stages = spec["stage"]
    names = [s["name"] for s in stages]
    for n in filter(None, [args.first, args.last, *(args.only or "").split(",")]):
        if n not in names:
            sys.exit(f"no stage {n!r}; the stages are {', '.join(names)}")
    if args.list:
        for s in stages:
            print(f"{s['name']:<10} {' '.join(s['doc'].split())}")
        return 0
    lo = names.index(args.first) if args.first else 0
    hi = names.index(args.last) + 1 if args.last else len(names)
    only = set(filter(None, (args.only or "").split(",")))
    chosen = [s for s in stages[lo:hi] if not only or s["name"] in only]

    base = {**os.environ, **spec.get("env", {})}
    for s in chosen:
        env = {**base, **s.get("env", {})}
        print(f"== {s['name']}: {' '.join(s['doc'].split())}", flush=True)
        for cmd in s["run"]:
            print(f"$ {cmd}", flush=True)
            if args.dry_run:
                continue
            code = subprocess.run(["bash", "-c", cmd], cwd=env["ROOT"], env=env).returncode
            if code:
                print(f"stage {s['name']} failed ({code}): {cmd}", file=sys.stderr)
                return code
    return 0


if __name__ == "__main__":
    sys.exit(main())
