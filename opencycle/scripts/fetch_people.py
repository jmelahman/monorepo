#!/usr/bin/env python3
"""Fetch the people the crowd is baked from: Microsoft Rocketbox (MIT), pinned to one commit.

Every file is checked against the git hash that commit records for it. Only what the bake
uses is fetched, since a whole avatar is several hundred megabytes of TGA.
"""

import argparse
import hashlib
import json
import sys
import urllib.request
from pathlib import Path

REPO = "microsoft/Microsoft-Rocketbox"
COMMIT = "0943055db6ec570bcef9f2c8b41c9e5467c808f9"
ROOT = Path(__file__).resolve().parent.parent / "assets" / "bake-sources" / "rocketbox"
AVATARS = (
    "Female_Adult_01",
    "Female_Adult_02",
    "Female_Adult_05",
    "Female_Adult_08",
    "Female_Adult_11",
    "Female_Adult_14",
    "Female_Party_01",
    "Male_Adult_01",
    "Male_Adult_03",
    "Male_Adult_05",
    "Male_Adult_08",
    "Male_Adult_11",
    "Male_Adult_14",
    "Male_Adult_17",
)
# Walks with their travel taken out, so they walk on the spot; standing about needs no such thing.
WALKS = "Assets/Animations/all_animations_max_motextr_xy"
STANDS = "Assets/Animations/all_animations_max_motextr_static"
CLIPS = {
    "f_walk_neutral_01": WALKS,
    "f_walk_neutral_02": WALKS,
    "f_walk_stroll_01": WALKS,
    "m_walk_neutral_01": WALKS,
    "m_walk_neutral_02": WALKS,
    "m_walk_stroll_01": WALKS,
    "f_idle_neutral_01": STANDS,
    "m_idle_neutral_01": STANDS,
    "m_cell_phone_textmessage": STANDS,
}
# Normal, specular and wrinkle maps would change nothing at the size people are drawn.
WANTED_TEXTURES = ("_color.tga",)


def wanted(path: str, avatars: tuple[str, ...]) -> str | None:
    """Where a repo path belongs under ROOT, or None if the bake doesn't use it."""
    for clip, folder in CLIPS.items():
        if path == f"{folder}/{clip}.max.fbx":
            return f"clips/{clip}.fbx"
    parts = path.split("/")
    if len(parts) != 6 or parts[:2] != ["Assets", "Avatars"] or parts[3] not in avatars:
        return None
    name = parts[3]
    if parts[4] == "Export" and parts[5] == f"{name}.fbx":
        return f"{name}/Export/{parts[5]}"
    if parts[4] == "Textures" and parts[5].endswith(WANTED_TEXTURES) and "wrinkle" not in parts[5]:
        return f"{name}/Textures/{parts[5]}"
    return None


def git_hash(data: bytes) -> str:
    return hashlib.sha1(b"blob %d\0" % len(data) + data).hexdigest()


def fetch(url: str) -> bytes:
    with urllib.request.urlopen(url, timeout=120) as response:
        return response.read()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--only", nargs="*", help="fetch just these avatars, to try something")
    arguments = parser.parse_args()
    avatars = tuple(arguments.only) if arguments.only else AVATARS
    api = f"https://api.github.com/repos/{REPO}/git/trees/{COMMIT}?recursive=1"
    tree = json.loads(fetch(api))["tree"]
    fetched = 0
    for entry in tree:
        target = wanted(entry["path"], avatars) if entry["type"] == "blob" else None
        if target is None:
            continue
        path = ROOT / target
        if path.exists() and git_hash(path.read_bytes()) == entry["sha"]:
            continue
        data = fetch(f"https://raw.githubusercontent.com/{REPO}/{COMMIT}/{entry['path']}")
        if git_hash(data) != entry["sha"]:
            print(f"people: {entry['path']} does not match its hash", file=sys.stderr)
            return 1
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
        fetched += 1
    print(f"people: {fetched} fetched")
    return 0


if __name__ == "__main__":
    sys.exit(main())
