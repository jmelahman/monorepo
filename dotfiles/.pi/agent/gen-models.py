#!/usr/bin/env python3
"""Generate models.json from a llama.cpp server's /v1/models listing.

Usage: ./gen-models.py [--url http://llama:18080] [--stdout]
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
from typing import NamedTuple
import urllib.request

OUTPUT = Path(__file__).resolve().parent / "models.json"
DEFAULT_URL = "http://llama:18080"
# Unloaded presets report no n_ctx; models.ini serves every model at this size.
DEFAULT_CONTEXT = 65536


class Model(NamedTuple):
    id: str
    name: str
    context_window: int

    def to_json(self) -> dict[str, str | int]:
        return {"id": self.id, "name": self.name, "contextWindow": self.context_window}


def list_models(base_url: str) -> list[Model]:
    # The URL is a local llama.cpp endpoint the user passes in.
    with urllib.request.urlopen(f"{base_url}/v1/models", timeout=30) as response:  # noqa: S310
        data = json.load(response)["data"]

    models = []
    for entry in data:
        # The router lists quant-normalized ids ("gemma4:26B"); the preset
        # aliases carry the intended model id.
        model_id = (entry.get("aliases") or [entry["id"]])[0]
        context = entry.get("meta", {}).get("n_ctx")
        models.append(
            Model(
                id=model_id,
                name=display_name(model_id),
                context_window=int(context or DEFAULT_CONTEXT),
            )
        )
    return sorted(models, key=lambda m: m.id)


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
        "--url",
        default=DEFAULT_URL,
        help=f"llama.cpp server base URL (default: {DEFAULT_URL})",
    )
    parser.add_argument(
        "--stdout", action="store_true", help="print instead of writing models.json"
    )
    args = parser.parse_args()

    base_url = args.url.rstrip("/")
    models = list_models(base_url)

    config = {
        "providers": {
            "llama": {
                "baseUrl": f"{base_url}/v1",
                "api": "openai-completions",
                "apiKey": "llama",
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
