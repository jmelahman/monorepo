#!/usr/bin/env python3
from __future__ import annotations

import pathlib
import tomllib

README_CONTENTS_TPL = """
[![Arch User Repsoitory](https://img.shields.io/aur/version/{package})](https://aur.archlinux.org/packages/{package})
[![Github Release](https://img.shields.io/github/v/release/{repository})](https://github.com/{repository})
"""


def main() -> None:
    with pathlib.Path("nvchecker.toml").open("rb") as f:
        data = tomllib.load(f)
    for package, info in data.items():
        if info.get("source") != "github":
            continue
        readme = README_CONTENTS_TPL.format(package=package, repository=info["github"])
        (pathlib.Path(package) / "README.md").write_text(readme.lstrip())

if __name__ == "__main__":
    main()
