#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
from typing import NamedTuple

# AUR package name to upstream package name.
ENTRY_TO_UPSTREAM = {
    "python-e3-core": "e3-core",
    "python-e3-testsuite": "e3-testsuite",
}


class Package(NamedTuple):
    name: str
    version: str
    revision: str


def run_nvchecker(entry: str) -> list[str]:
    result = subprocess.run(  # noqa: S603
        [
            "uvx",
            "--with=nvchecker[pypi]",
            "nvchecker",
            "--entry",
            ENTRY_TO_UPSTREAM.get(entry, entry),
            "--logger=json",
            "-c",
            "nvchecker.toml",
        ],
        check=True,
        text=True,
        stdout=subprocess.PIPE,
    )
    return result.stdout.splitlines()


def parse_nvchecker_output(lines: list[str]) -> list[Package]:
    nv_data = []
    for line in lines:
        data = json.loads(line)
        nv_data.append(
            Package(
                name=ENTRY_TO_UPSTREAM.get(data["name"]) or data["name"],
                version=data["version"],
                revision=data["revision"],
            )
        )
    return nv_data


def process_package(package: Package) -> None:
    dir_path = Path(package.name)
    pkgbuild_path = dir_path / "PKGBUILD"
    srcinfo_path = dir_path / ".SRCINFO"

    content = pkgbuild_path.read_text()
    if not content:
        msg = f"Failed to read PKGBUILD for package {package.name}"
        raise RuntimeError(msg)

    match = re.search(r"(?m)^pkgver=(.+)$", content)
    if not match:
        msg = f"pkgver not found in PKGBUILD for package {package.name}"
        raise RuntimeError(msg)

    current_version = match.group(1).strip()

    if current_version == package.version:
        return

    updated_content = re.sub(r"(?m)^pkgver=(.+)$", f"pkgver={package.version}", content)
    updated_content = re.sub(r"(?m)^pkgrel=(.+)$", "pkgrel=1", updated_content)
    updated_content = re.sub(
        r"(?m)^_commit=(.+)$", f"_commit='{package.revision}'", updated_content
    )
    pkgbuild_path.write_text(updated_content)

    if "sums=('SKIP')" not in updated_content:
        # TODO: Hint to the user if this is uninstalled it's from the 'pacman-contrib' package.
        subprocess.run(["updpkgsums"], check=True, stdout=subprocess.DEVNULL, cwd=dir_path)

    with srcinfo_path.open(mode="w") as f:
        subprocess.run(["makepkg", "--printsrcinfo"], stdout=f, check=True, cwd=dir_path)

    print(f"Bump {package.name} from {current_version} to {package.version}")


def _directory(value: str) -> str:
    if not os.path.isdir(value):
        msg = f"'{value}' is not a valid directory path."
        raise argparse.ArgumentTypeError(msg)
    return value


def main() -> None:
    parser = argparse.ArgumentParser(description="Process package version updates")
    parser.add_argument("package", type=_directory, help="The name of the package to process")
    args = parser.parse_args()

    lines = run_nvchecker(args.package)
    packages = parse_nvchecker_output(lines)

    package = next((p for p in packages if p.name == args.package), None)

    if package:
        process_package(package)


if __name__ == "__main__":
    main()
