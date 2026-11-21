#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = ["huggingface_hub"]
# ///
"""Download a GGUF from Hugging Face into models/ and add its llama.cpp preset.

Usage:
    ./pull-model.py unsloth/Inkling-2-Mini-GGUF Q4_K_M
    ./pull-model.py unsloth/Inkling-2-Mini-GGUF Q4_K_M --id inkling-2-mini --mmproj

QUANT is matched against the file names in the repo. A sharded quant
(`-00001-of-0000N.gguf`) is downloaded whole and the preset points at the
first shard, which is how llama.cpp finds the rest. With --mmproj the repo's
mmproj file (F16 if there is a choice) is downloaded too, for vision models.

The preset is appended to models.ini; restart the service to pick it up:
    docker compose up -d llama
"""

from __future__ import annotations

import argparse
from pathlib import Path
import re
import sys

from huggingface_hub import hf_hub_download
from huggingface_hub import list_repo_files

HERE = Path(__file__).resolve().parent
MODELS = HERE.parent / "models"
PRESETS = HERE / "models.ini"


def pick_quant(files: list[str], quant: str) -> list[str]:
    """The files of one quant: a single GGUF, or all shards of a split one."""
    want = quant.lower()
    hits = [
        f for f in files if f.endswith(".gguf") and "mmproj" not in f.lower() and want in f.lower()
    ]
    if not hits:
        sys.exit(f"no {quant} GGUF in the repo; it has:\n  " + "\n  ".join(files))
    # Shards share a name up to the -0000N-of-0000M suffix.
    groups: dict[str, list[str]] = {}
    for f in hits:
        groups.setdefault(re.sub(r"-\d+-of-\d+(?=\.gguf$)", "", f), []).append(f)
    if len(groups) > 1:
        sys.exit(f"{quant} is ambiguous; be more specific:\n  " + "\n  ".join(groups))
    return sorted(next(iter(groups.values())))


def pick_mmproj(files: list[str]) -> str:
    projs = sorted(f for f in files if "mmproj" in f.lower() and f.endswith(".gguf"))
    if not projs:
        sys.exit("--mmproj given, but the repo has no mmproj file")
    return next((f for f in projs if "f16" in f.lower()), projs[0])


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("repo", help="Hugging Face repo, e.g. unsloth/Inkling-2-Mini-GGUF")
    ap.add_argument("quant", help="quant to fetch, e.g. Q4_K_M")
    ap.add_argument("--id", help="model id clients request (default: from the repo)")
    ap.add_argument("--mmproj", action="store_true", help="also fetch the vision projector")
    args = ap.parse_args()

    model_id = (
        args.id or re.sub(r"-GGUF$", "", args.repo.split("/")[-1], flags=re.IGNORECASE).lower()
    )
    if re.search(rf"^\[{re.escape(model_id)}\]$", PRESETS.read_text(), re.MULTILINE):
        sys.exit(f"{PRESETS.name} already has [{model_id}]; pick another --id")

    files = list_repo_files(args.repo)
    wanted = pick_quant(files, args.quant)
    proj = pick_mmproj(files) if args.mmproj else None

    # Flat in models/, so the container path is /models/<name>.
    paths = {}
    for f in wanted + ([proj] if proj else []):
        print(f"downloading {f}", flush=True)
        hf_hub_download(args.repo, f, local_dir=MODELS)
        paths[f] = f"/models/{f}"

    lines = [f"\n[{model_id}]", f"alias = {model_id}", f"model = {paths[wanted[0]]}"]
    if proj:
        lines.append(f"mmproj = {paths[proj]}")
    with PRESETS.open("a") as out:
        out.write("\n".join(lines) + "\n")

    print(f"\nadded [{model_id}] to {PRESETS.name}; now run:\n  docker compose up -d llama")


if __name__ == "__main__":
    main()
