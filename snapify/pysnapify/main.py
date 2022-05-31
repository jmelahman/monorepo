#!/usr/bin/env python3
import logging
import os

from .snapifier import Snapifier
from .constants import NONINTERACTIVE_DEFAULT

logging.basicConfig(
    format="%(levelname)s: %(message)s", level=os.environ.get("LOGLEVEL", "INFO")
)


def main(noninteractive: bool = NONINTERACTIVE_DEFAULT) -> None:
    snapifier = Snapifier(noninteractive)
    host_packages = snapifier.manager.get_installed_packages()
    portable_packages = [
        package for package in host_packages if snapifier.snap.has_available(package)
    ]
    if not portable_packages:
        return
    removeable_packages = snapifier.manager.filter_removeable(portable_packages)
    if not removeable_packages:
        return
    snapifier.snap.install(removeable_packages)
    snapifier.manager.remove(removeable_packages)
