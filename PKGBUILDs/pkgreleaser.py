#!/usr/bin/env python3
import argparse
import json
import re
import subprocess
import os
from pathlib import Path
from typing import NamedTuple

class Package(NamedTuple):
    name: str
    version: str
    revision: str

def run_nvchecker(entry: str) -> list[str]:
    result = subprocess.run(
        [
            "uv",
            "run",
            "--with=git+https://github.com/lilydjwg/nvchecker@2722ccc7fef71fccf9f031d8299bc3c36736fdda",
            "nvchecker",
            "--entry",
            entry,
            "--logger=json",
            "-c",
            "nvchecker.toml"
        ],
        check=True,
        text=True,
        capture_output=True,
    )
    return result.stdout.splitlines()

def parse_nvchecker_output(lines: list[str]) -> list[Package]:
    nv_data = []
    for line in lines:
        data = json.loads(line)
        nv_data.append(
            Package(name=data["name"], version=data["version"], revision=data["revision"])
        )
    return nv_data

def process_package(package: Package) -> None:
    dir_path = Path(package.name)
    pkgbuild_path = dir_path / "PKGBUILD"
    srcinfo_path = dir_path / ".SRCINFO"

    content = pkgbuild_path.read_text()
    if not content:
        raise RuntimeError(f"Failed to read PKGBUILD for package {package.name}")

    match = re.search(r"(?m)^pkgver=(.+)$", content)
    if not match:
        raise RuntimeError(f"pkgver not found in PKGBUILD for package {package.name}")

    current_version = match.group(1).strip()

    if current_version == package.version:
        return

    updated_content = re.sub(r"(?m)^pkgver=(.+)$", f"pkgver={package.version}", content)
    updated_content = re.sub(r"(?m)^pkgrel=(.+)$", f"pkgrel=1", updated_content)
    updated_content = re.sub(r"(?m)^_commit=(.+)$", f"_commit='{package.revision}'", updated_content)
    pkgbuild_path.write_text(updated_content)
    with srcinfo_path.open(mode="w") as f:
        subprocess.run(["makepkg", "--printsrcinfo"], stdout=f, check=True, cwd=dir_path)

    if "sums=('SKIP')" not in updated_content:
        subprocess.run(["updpkgsums"], check=True, capture_output=True, cwd=dir_path)

    print(f"Bump {package.name} from {current_version} to {package.version}")

def _directory(value):
    if not os.path.isdir(value):
        raise argparse.ArgumentTypeError(f"'{value}' is not a valid directory path.")
    return value


def main():
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
