#!/usr/bin/env python3
#
#  Playlists are located in ~/.local/share/rhythmbox/playlists.xml
#  Playlists are converted to m3u using https://github.com/adrienverge/rhythmbox_playlist_to_m3u/blob/master/rhythmbox_playlist_to_m3u.py
#  For some reason, only mounting the device through Clementine works, not Dolphin for example.
#
from __future__ import annotations

import asyncio
import hashlib
import os
import shutil

MUSIC_DIR = (
    "/run/user/1000/gvfs/mtp:host=Xiaomi_Mi_MIX_2_1cd7acd5/Internal shared storage/"
)
# Too lazy to plumb from an argparser.
_CHECK_INTEGRITY = True


def normalize_path(path: str) -> str:
    filename = os.path.basename(path)
    _, dir2 = os.path.split(os.path.dirname(path))
    _, dir1 = os.path.split(os.path.dirname(os.path.dirname(path)))
    return os.path.join(dir1, dir2, filename)


# https://stackoverflow.com/a/44873382
async def sha1sum(filename: str) -> str:
    h = hashlib.sha1()
    b = bytearray(128 * 1024)
    mv = memoryview(b)
    with open(filename, "rb", buffering=0) as f:
        while n := f.readinto(mv):
            h.update(mv[:n])
    return h.hexdigest()


async def get_files_to_copy() -> dict[str, str]:
    print("Getting files to copy...")
    files_to_copy = {}
    with open("/home/jamison/Documents/Artists.m3u") as f:
        for line in f:
            sline = line.strip()
            if sline.startswith("#"):
                continue
            normalized_path = normalize_path(sline)
            files_to_copy[normalized_path] = await sha1sum(sline)
    print("Found {} files to copy.".format(len(files_to_copy)))
    return files_to_copy


async def maybe_copy_file(filename: str, sha: str) -> None:
    source_path = os.path.join(os.path.expanduser("~"), "Music", filename)
    dest_path = os.path.join(MUSIC_DIR, "Music", filename)
    os.makedirs(os.path.dirname(dest_path), exist_ok=True)
    if _CHECK_INTEGRITY:
        new_sha = await sha1sum(dest_path)
        if new_sha == sha:
            return
    elif os.path.exists(dest_path):
        return
    shutil.copyfile(source_path, dest_path)
    print("Copied file to:", dest_path)


async def copy_files(files_to_copy: dict[str, str]) -> None:
    print("Copying files...")
    tasks = []
    for path, sha in files_to_copy.items():
        tasks.append(asyncio.create_task(maybe_copy_file(path, sha)))
    await asyncio.gather(*tasks)


async def main() -> None:
    if not os.path.exists(MUSIC_DIR):
        raise FileNotFoundError("No such directory: " + MUSIC_DIR)
    files_to_copy = await get_files_to_copy()
    await copy_files(files_to_copy)


if __name__ == "__main__":
    asyncio.run(main())
