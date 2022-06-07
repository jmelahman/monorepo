import logging
import subprocess
import sys
from typing import Any

from .base import PackageManager


class Pacman(PackageManager):
    def __init__(
        self,
        noninteractive: bool,
        ignored_packages: list[str],
        name: str = "pacman",
    ) -> None:
        super().__init__(noninteractive, ignored_packages, name)

    def get_installed_packages(self) -> list[str]:
        if self._installed_packages != []:
            return self._installed_packages
        self._installed_packages = [
            package
            for package in subprocess.check_output([self._bin, "-Qq"])
            .decode()
            .rstrip()
            .split("\n")
            if package not in self._not_available
        ]
        return self._installed_packages

    def has_available(self, package_name: str) -> bool:
        if package_name in self._not_available:
            return False
        # TODO: This may need compiled with 're'.
        return not subprocess.run([self._bin, "-Qs", f"^{package_name}$"]).returncode

    def has_installed(self, package_name: str) -> bool:
        return package_name in self.get_installed_packages()

    def install(self, packages: list[str]) -> None:
        try:
            install_cmd = [
                self._sudo,
                self._bin,
                f"-S",
            ]
            if self._noninteractive:
                install_cmd.append("--noconfirm")
            subprocess.check_call(install_cmd + packages)
        except (
            subprocess.CalledProcessError,
            KeyboardInterrupt,
        ):  # Allow user to decline gracefully.
            sys.exit(1)

    def filter_removeable(self, packages: list[str]) -> list[str]:
        # TODO: Might need to run each package individually and aggregate afterwards.
        dependency_query = subprocess.run(
            [self._bin, "-Qqt", *packages], stdout=subprocess.PIPE
        )
        if dependency_query.returncode:
            logging.info(
                f"The following packages were unable to be snapified: {' '.join(packages)}"
            )
            return []
        return dependency_query.stdout.decode().strip().split("\n")

    def remove(self, packages: list[str], purge: bool = False) -> None:
        logging.info(f"Removing the following packages: {' '.join(packages)}")
        try:
            remove_cmd = [
                self._sudo,
                self._bin,
                f"-Rs{'n' if purge else ''}",
            ]
            if self._noninteractive:
                remove_cmd.append("--noconfirm")
            subprocess.check_call(remove_cmd + packages)
        except (
            subprocess.CalledProcessError,
            KeyboardInterrupt,
        ):  # Allow user to decline gracefully.
            sys.exit(1)
