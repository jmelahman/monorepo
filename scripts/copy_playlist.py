#!/usr/bin/env python3
from __future__ import annotations

import concurrent.futures
import glob
import os
import shutil

MUSIC_DIR = (
    "/run/user/1000/gvfs/mtp:host=Xiaomi_Mi_MIX_2_1cd7acd5/Internal shared storage/"
)


def copy_file(filename: str) -> None:
    source_path = os.path.join(os.path.expanduser("~"), "Music", filename)
    dest_path = os.path.join(MUSIC_DIR, "Music", filename)
    os.makedirs(os.path.dirname(dest_path), exist_ok=True)
    if os.path.exists(dest_path):
        return
    shutil.copyfile(source_path, dest_path)
    print("Copied file:", source_path)


def normalize_path(path: str) -> str:
    filename = os.path.basename(path)
    _, dir2 = os.path.split(os.path.dirname(path))
    _, dir1 = os.path.split(os.path.dirname(os.path.dirname(path)))
    return os.path.join(dir1, dir2, filename)


def get_existing_files() -> set:
    print("Getting existing files...")
    return {
        normalize_path(path)
        for path in glob.glob(
            os.path.join(MUSIC_DIR, "Music", "**", "**", "*"),
        )
    }


if __name__ == "__main__":
    existing_files = get_existing_files()
    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as executor:
        with open("/home/jamison/Documents/Artists.m3u") as f:
            for line in f:
                sline = line.strip()
                if sline.startswith("#"):
                    continue
                if sline in existing_files:
                    continue
                futures = [executor.submit(copy_file, sline)]
        done, _ = concurrent.futures.wait(futures)
    for future in done:
        future.result()
