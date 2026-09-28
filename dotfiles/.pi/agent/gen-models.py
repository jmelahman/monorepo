#!/usr/bin/env python3
"""Generate models.json from the output of `ollama list`.

Usage: ./gen-models.py [--max-context N] [--stdout]
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import subprocess
from typing import NamedTuple

OUTPUT = Path(__file__).resolve().parent / "models.json"
DEFAULT_MAX_CONTEXT = 131072


class Model(NamedTuple):
    id: str
    name: str
    context_window: int

    def to_json(self) -> dict[str, str | int]:
        return {"id": self.id, "name": self.name, "contextWindow": self.context_window}


def ollama(*args: str) -> str:
    return subprocess.run(  # noqa: S603
        ["ollama", *args], check=True, capture_output=True, text=True
    ).stdout


def list_models() -> list[str]:
    lines = ollama("list").splitlines()[1:]  # skip header
    return [line.split()[0] for line in lines if line.strip()]


def context_length(model: str) -> int | None:
    match = re.search(r"^\s*context length\s+(\d+)", ollama("show", model), re.MULTILINE)
    return int(match.group(1)) if match else None


def display_name(model: str) -> str:
    # "qwen3.6:35b-a3b" -> "Qwen 3.6 35B-A3B", "gemma4:e4b" -> "Gemma 4 E4B"
    family, _, tag = model.partition(":")
    match = re.fullmatch(r"([a-z-]+?)([\d.]+)?", family, re.IGNORECASE)
    if match:
        parts = [match.group(1).capitalize()]
        if match.group(2):
            parts.append(match.group(2))
    else:
        parts = [family]
    if tag and tag != "latest":
        parts.append(tag.upper())
    return " ".join(parts)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--max-context",
        type=int,
        default=DEFAULT_MAX_CONTEXT,
        help=f"cap contextWindow at this value (default: {DEFAULT_MAX_CONTEXT})",
    )
    parser.add_argument(
        "--stdout", action="store_true", help="print instead of writing models.json"
    )
    args = parser.parse_args()

    models = [
        Model(
            id=model,
            name=display_name(model),
            context_window=min(context_length(model) or args.max_context, args.max_context),
        )
        for model in sorted(list_models())
    ]

    config = {
        "providers": {
            "ollama": {
                "baseUrl": "http://ollama:11434/v1",
                "api": "openai-completions",
                "apiKey": "ollama",
                "models": [model.to_json() for model in models],
            }
        }
    }
    text = json.dumps(config, indent=2) + "\n"

    if args.stdout:
        print(text, end="")
    else:
        OUTPUT.write_text(text)
        print(f"Wrote {len(models)} models to {OUTPUT}")


if __name__ == "__main__":
    main()
