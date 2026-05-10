#!/usr/bin/env python3
"""Validate that .pre-commit-config.yaml and nvchecker.toml have entries for all packages."""

import os
import re
import sys

try:
    import tomllib
except ModuleNotFoundError:
    import tomli as tomllib  # type: ignore[no-redef]

REPO_ROOT = os.path.dirname(os.path.abspath(__file__))

# Packages that are known to be excluded from pre-commit (with reasons).
PRE_COMMIT_EXCLUDES: dict[str, str] = {
    "python-e3-testsuite": "depends on python-e3-core[aur]; no precedence for AUR deps",
}

# Mapping from nvchecker section name → package directory name, for cases
# where they differ.
NVCHECKER_NAME_MAP: dict[str, str] = {
    "e3-core": "python-e3-core",
    "e3-testsuite": "python-e3-testsuite",
}


def find_packages() -> set[str]:
    """Return the set of directory names that contain a PKGBUILD."""
    packages: set[str] = set()
    for entry in os.scandir(REPO_ROOT):
        if entry.is_dir() and os.path.isfile(os.path.join(entry.path, "PKGBUILD")):
            packages.add(entry.name)
    return packages


def get_precommit_ids() -> set[str]:
    """Return the set of hook ids defined in .pre-commit-config.yaml."""
    config_path = os.path.join(REPO_ROOT, ".pre-commit-config.yaml")
    with open(config_path) as f:
        content = f.read()

    # Match all '- id: <value>' lines (including commented-out ones are skipped
    # since they start with #).
    return set(re.findall(r"^\s+- id:\s*(.+)$", content, re.MULTILINE))


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
    precommit_ids = get_precommit_ids()
    nvchecker_pkgs = get_nvchecker_sections()

    errors: list[str] = []

    # Check pre-commit coverage.
    missing_precommit = packages - precommit_ids - set(PRE_COMMIT_EXCLUDES)
    for pkg in sorted(missing_precommit):
        errors.append(f".pre-commit-config.yaml: missing hook for '{pkg}'")

    extra_precommit = precommit_ids - packages
    for pkg in sorted(extra_precommit):
        errors.append(f".pre-commit-config.yaml: hook '{pkg}' has no matching package directory")

    # Check nvchecker coverage.
    missing_nvchecker = packages - nvchecker_pkgs
    for pkg in sorted(missing_nvchecker):
        errors.append(f"nvchecker.toml: missing entry for '{pkg}'")

    extra_nvchecker = nvchecker_pkgs - packages
    for pkg in sorted(extra_nvchecker):
        errors.append(f"nvchecker.toml: entry '{pkg}' has no matching package directory")

    # Report known exclusions for visibility.
    for pkg, reason in sorted(PRE_COMMIT_EXCLUDES.items()):
        if pkg in packages:
            print(f"INFO: '{pkg}' excluded from pre-commit: {reason}")

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
