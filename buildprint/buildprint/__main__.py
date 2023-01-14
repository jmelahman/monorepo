#!/usr/bin/env python3
from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    import io

import click

from buildprint._run import run
from buildprint._version import __version__
from buildprint._version import __version_info__


@click.version_option(__version__)
@click.argument(
    "blueprint",
    type=click.File("rb"),
)
@click.command()
def main(blueprint: io.BufferedReader) -> int:
    run(blueprint)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
