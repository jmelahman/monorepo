#!/usr/bin/env python3
from __future__ import annotations

import os
import sys
import zipfile


def zip_dir(source_dir: str, zip_name: str) -> None:
    with zipfile.ZipFile(zip_name, "w", zipfile.ZIP_DEFLATED) as zf:
        for root, _, files in os.walk(source_dir):
            for file in files:
                path = os.path.join(root, file)
                arcname = os.path.relpath(path, start=source_dir)
                zf.write(path, arcname)
    print(f"Created archive: {zip_name}")


def main() -> None:
    base_dir = os.getcwd()
    target_dirs = []

    if len(sys.argv) > 1:
        arg = sys.argv[1]
        if os.path.isdir(arg):
            target_dirs = [arg]
        else:
            print(f"Error: '{arg}' is not a directory")
            sys.exit(1)
    else:
        # No argument: archive all directories in current folder
        target_dirs = [d for d in os.listdir(base_dir) if os.path.isdir(d)]

    for d in target_dirs:
        zip_name = f"{d}.xpi"
        zip_dir(d, zip_name)


if __name__ == "__main__":
    main()
