#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "nvchecker[pypi]>=2.20",
# ]
# ///
from __future__ import annotations

import argparse
import json
import logging
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from typing import NamedTuple

# AUR package name to upstream package name.
ENTRY_TO_UPSTREAM: dict[str, str] = {
    # "python-example": "example",
}
UPSTREAM_TO_ENTRY = {v: k for k, v in ENTRY_TO_UPSTREAM.items()}


class Package(NamedTuple):
    name: str
    version: str
    revision: str | None
    gitref: str | None


def run_nvchecker(entry: str | None = None) -> list[str]:
    """Run nvchecker for one entry, or for every entry in the config when None."""
    cmd = [sys.executable, "-m", "nvchecker", "--logger=json", "-c", "nvchecker.toml"]
    if entry is not None:
        cmd += ["--entry", ENTRY_TO_UPSTREAM.get(entry, entry)]

    token = os.environ.get("GITHUB_TOKEN")
    if not token:
        logging.warning("GITHUB_TOKEN is not set; GitHub API requests will be rate limited")
        result = subprocess.run(cmd, check=True, text=True, stdout=subprocess.PIPE)  # noqa: S603
        return result.stdout.splitlines()

    # Passed via a 0600 keyfile rather than argv so the token never shows up in `ps`.
    with tempfile.NamedTemporaryFile(mode="w", suffix=".toml", delete_on_close=False) as f:
        f.write(f"[keys]\ngithub = {json.dumps(token)}\n")
        f.close()
        result = subprocess.run(  # noqa: S603
            [*cmd, "--keyfile", f.name], check=True, text=True, stdout=subprocess.PIPE
        )
    return result.stdout.splitlines()


def parse_nvchecker_output(lines: list[str]) -> tuple[list[Package], bool]:
    """Return the packages nvchecker resolved and whether any entry failed."""
    packages = []
    failed = False
    for line in lines:
        data = json.loads(line)
        name = UPSTREAM_TO_ENTRY.get(data["name"], data["name"])
        if "version" in data:
            packages.append(
                Package(
                    name=name,
                    version=data["version"],
                    revision=data.get("revision"),
                    gitref=data.get("rich_result", {}).get("gitref"),
                )
            )
        elif data.get("level") == "error" and data["event"] != "no-result":
            # nvchecker logs the failure itself and then a "no-result" line for the same entry.
            logging.error("nvchecker failed for '%s': %s", name, data.get("error") or data["event"])
            failed = True
    return packages, failed


def extract_git_url(content: str) -> str | None:
    match = re.search(r'["\'](?:[^"\']*::)?git\+([^#"\']+)', content)
    if not match:
        return None
    git_url = match.group(1)
    if "$url" in git_url:
        url_match = re.search(r'(?m)^url=["\']?([^"\'\n]+?)["\']?$', content)
        if not url_match:
            return None
        git_url = git_url.replace("${url}", url_match.group(1)).replace("$url", url_match.group(1))
    return git_url


def resolve_revision(git_url: str, gitref: str) -> str | None:
    """Resolve a ref to a commit sha, peeling annotated tags."""
    result = subprocess.run(  # noqa: S603
        ["git", "ls-remote", git_url, gitref, f"{gitref}^{{}}"],  # noqa: S607
        check=True,
        text=True,
        stdout=subprocess.PIPE,
    )
    refs = {}
    for line in result.stdout.splitlines():
        sha, _, ref = line.partition("\t")
        refs[ref] = sha
    return refs.get(f"{gitref}^{{}}") or refs.get(gitref)


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

    if re.search(r"(?m)^_commit=", updated_content):
        # Prefer resolving the ref ourselves: nvchecker's revision is the tag
        # object id for annotated tags, not the commit it points to.
        revision = None
        if package.gitref:
            git_url = extract_git_url(updated_content)
            if git_url:
                revision = resolve_revision(git_url, package.gitref)
        revision = revision or package.revision
        if not revision:
            msg = f"Could not determine commit for {package.name} {package.version}"
            raise RuntimeError(msg)
        updated_content = re.sub(r"(?m)^_commit=(.+)$", f"_commit='{revision}'", updated_content)

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


def main() -> int:
    parser = argparse.ArgumentParser(description="Process package version updates")
    parser.add_argument(
        "packages", nargs="*", type=_directory, help="Packages to process (default: every entry in nvchecker.toml)"
    )
    args = parser.parse_args()

    if args.packages:
        packages, failed = [], False
        for entry in args.packages:
            found, entry_failed = parse_nvchecker_output(run_nvchecker(entry))
            packages += [p for p in found if p.name == entry]
            failed |= entry_failed
    else:
        packages, failed = parse_nvchecker_output(run_nvchecker())

    for package in packages:
        if not os.path.isdir(package.name):
            logging.warning("Skipping '%s': no such package directory", package.name)
            continue
        try:
            process_package(package)
        except (RuntimeError, subprocess.CalledProcessError):
            logging.exception("Failed to process '%s'", package.name)
            failed = True
    return int(failed)


if __name__ == "__main__":
    raise SystemExit(main())
