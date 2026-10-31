#!/usr/bin/env python3
"""A running record of how the street looks, to check on at any time from a browser.

    scripts/storybook.py serve [--port 8000]
    scripts/storybook.py add "Title" --note "What changed" [--images a.png b.png]

add renders the usual shots of the current working tree, or files the given images, as a
new chapter in captures/storybook, which is ignored by git. The page picks new chapters
up by itself.
"""

import argparse
import functools
import http.server
import json
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BOOK = ROOT / "captures" / "storybook"
PAGE = Path(__file__).with_suffix(".html")
BIOME = "neon"
# Seconds into the ride, and what the camera happens to be looking at then.
SHOTS = ((4, "Start of the ride"), (14, "Mid street"), (30, "Further along"))
RENDER_TIMEOUT = 300


def git(*args: str) -> str:
    return subprocess.run(
        ["git", *args], cwd=ROOT, capture_output=True, text=True, check=True
    ).stdout.strip()


def render(chapter: Path) -> list[dict[str, str]]:
    shots = []
    for at, label in SHOTS:
        out = chapter / f"at-{at:02d}.png"
        command = [
            "godot",
            "--path",
            str(ROOT),
            "--audio-driver",
            "Dummy",
            "--",
            f"--biome={BIOME}",
            f"--still={out}",
            f"--at={at}",
        ]
        run = subprocess.run(
            command, cwd=ROOT, capture_output=True, text=True, timeout=RENDER_TIMEOUT
        )
        # A broken shader still renders, as flat untextured faces, and exits 0.
        broken = sorted({line for line in run.stdout.splitlines() if "SHADER ERROR" in line})
        broken += sorted({line for line in run.stderr.splitlines() if "SHADER ERROR" in line})
        if broken:
            sys.exit("\n".join(broken))
        # Godot exits 0 on parse errors, so only the file says whether it worked.
        if not out.exists():
            sys.exit(f"No still came out for t={at}: run scripts/check.sh")
        shots.append({"file": out.name, "label": f"{label} ({at}s)"})
    return shots


def file_images(chapter: Path, images: list[Path]) -> list[dict[str, str]]:
    shots = []
    for image in images:
        shutil.copy2(image, chapter / image.name)
        shots.append({"file": image.name, "label": image.stem})
    return shots


def load_chapters() -> list[dict]:
    index = BOOK / "chapters.json"
    return json.loads(index.read_text()) if index.exists() else []


def add(title: str, note: str, images: list[Path] | None) -> None:
    stamp = time.strftime("%Y%m%d-%H%M%S")
    slug = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")[:40]
    chapter = BOOK / f"{stamp}-{slug}"
    chapter.mkdir(parents=True)
    try:
        shots = file_images(chapter, images) if images else render(chapter)
    except BaseException:
        shutil.rmtree(chapter)
        raise
    edited = bool(git("status", "--porcelain", "--", "."))
    chapters = load_chapters()
    chapters.append(
        {
            "title": title,
            "note": note,
            "when": time.strftime("%Y-%m-%d %H:%M"),
            "commit": git("rev-parse", "--short", "HEAD") + ("+edits" if edited else ""),
            "dir": chapter.name,
            "shots": shots,
        }
    )
    index = BOOK / "chapters.json"
    draft = index.with_suffix(".tmp")
    draft.write_text(json.dumps(chapters, indent=2))
    draft.replace(index)
    shutil.copy2(PAGE, BOOK / "index.html")
    print(chapter)


def serve(port: int) -> None:
    BOOK.mkdir(parents=True, exist_ok=True)
    shutil.copy2(PAGE, BOOK / "index.html")
    handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(BOOK))
    print(f"http://localhost:{port}")
    http.server.ThreadingHTTPServer(("127.0.0.1", port), handler).serve_forever()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    commands = parser.add_subparsers(dest="command", required=True)
    serving = commands.add_parser("serve")
    serving.add_argument("--port", type=int, default=8000)
    adding = commands.add_parser("add")
    adding.add_argument("title")
    adding.add_argument("--note", default="")
    adding.add_argument("--images", nargs="+", type=Path)
    args = parser.parse_args()
    if args.command == "serve":
        serve(args.port)
    else:
        add(args.title, args.note, args.images)


if __name__ == "__main__":
    main()
