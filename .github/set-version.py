"""Rewrite the `[package] version` in Cargo.toml, so releases match the git tag."""

from __future__ import annotations

import pathlib
import re
import subprocess
import sys

version = sys.argv[1]
manifest = pathlib.Path("Cargo.toml")
source = manifest.read_text()

updated, count = re.subn(
    r'^version = ".*"$', f'version = "{version}"', source, count=1, flags=re.MULTILINE
)
if count != 1:
    sys.exit("failed to locate the package version in Cargo.toml")

manifest.write_text(updated)
# Keep Cargo.lock consistent with the bumped version without touching dependencies.
subprocess.check_call(["cargo", "update", "--workspace"])
