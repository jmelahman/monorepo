#!/usr/bin/env python3
from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    import io

import click

from buildprint._run import _BUILDKITE
from buildprint._run import _SUPPORTED_PLATFORMS
from buildprint._run import run
from buildprint._version import __version__
from buildprint._version import __version_info__


@click.version_option(__version__)
@click.option(
    "--platform",
    help="CI platform.",
    default=_BUILDKITE,
    type=click.Choice([c for c in _SUPPORTED_PLATFORMS]),
)
@click.argument(
    "blueprint",
    type=click.File("rb"),
)
@click.command()
def main(blueprint: io.BufferedReader, platform: str | None) -> None:
    run(blueprint, platform)


if __name__ == "__main__":
    main()
