#!/usr/bin/env python3
"""Validate that nvchecker.toml has an entry for every package."""

import os
import sys

try:
    import tomllib
except ModuleNotFoundError:
    import tomli as tomllib  # type: ignore[no-redef]

REPO_ROOT = os.path.dirname(os.path.abspath(__file__))

# Mapping from nvchecker section name → package directory name, for cases
# where they differ.
NVCHECKER_NAME_MAP: dict[str, str] = {
    # "upstream-name": "python-upstream-name",
}


def find_packages() -> set[str]:
    """Return the set of directory names that contain a PKGBUILD."""
    packages: set[str] = set()
    for entry in os.scandir(REPO_ROOT):
        if entry.is_dir() and os.path.isfile(os.path.join(entry.path, "PKGBUILD")):
            packages.add(entry.name)
    return packages


def get_nvchecker_sections() -> set[str]:
    """Return the set of section names in nvchecker.toml, mapped to package dir names."""
    toml_path = os.path.join(REPO_ROOT, "nvchecker.toml")
    with open(toml_path, "rb") as f:
        data = tomllib.load(f)

    sections: set[str] = set()
    for name in data:
        mapped = NVCHECKER_NAME_MAP.get(name, name)
        sections.add(mapped)
    return sections


def main() -> int:
    packages = find_packages()
    nvchecker_pkgs = get_nvchecker_sections()

    errors: list[str] = []

    # Check nvchecker coverage.
    missing_nvchecker = packages - nvchecker_pkgs
    for pkg in sorted(missing_nvchecker):
        errors.append(f"nvchecker.toml: missing entry for '{pkg}'")

    extra_nvchecker = nvchecker_pkgs - packages
    for pkg in sorted(extra_nvchecker):
        errors.append(f"nvchecker.toml: entry '{pkg}' has no matching package directory")

    if errors:
        print()
        for error in errors:
            print(f"ERROR: {error}")
        print(f"\n{len(errors)} error(s) found.")
        return 1

    print("\nAll packages are covered. ✓")
    return 0


if __name__ == "__main__":
    sys.exit(main())
