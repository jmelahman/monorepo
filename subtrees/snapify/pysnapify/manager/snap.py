import enum
import logging
import os
import socket
import subprocess
from typing import Optional

import requests
import urllib3

from .base import PackageManager


class SnapdConnection(urllib3.connection.HTTPConnection):
    def __init__(self) -> None:
        super().__init__("localhost")

    def connect(self) -> None:
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.connect("/run/snapd.socket")


class SnapdConnectionPool(urllib3.connectionpool.HTTPConnectionPool):
    def __init__(self) -> None:
        super().__init__("localhost")

    def _new_conn(self) -> SnapdConnection:
        return SnapdConnection()


class SnapdAdapter(requests.adapters.HTTPAdapter):
    def get_connection(
        self,
        url: str,
        proxies: Optional[dict[str, str]] = None,
    ) -> SnapdConnectionPool:
        return SnapdConnectionPool()


class SnapdConfinement(enum.Enum):
    CLASSIC = "classic"
    STRICT = "strict"


class Snapd(PackageManager):
    def __init__(
        self,
        noninteractive: bool,
        ignored_packages: list[str],
        name: str = "snap",
    ) -> None:
        super().__init__(noninteractive, ignored_packages, name)
        self._not_available = self._not_available + ["snapd"]
        self._available_packages = self.get_available_packages()
        self._installed_packages = self.get_installed_packages()
        self._session = requests.Session()
        self._session.mount("http://snapd/", SnapdAdapter())

    def get_installed_packages(self) -> list[str]:
        if self._installed_packages != []:
            return self._installed_packages
        snap_list = (
            subprocess.check_output([self._bin, "list"]).decode().rstrip().split("\n")
        )
        snap_list.pop(0)  # Remove header
        self._installed_packages = [package.split(" ")[0] for package in snap_list]
        return self._installed_packages

    @staticmethod
    def _names_exists(names_file: str) -> bool:
        return os.path.exists(names_file)

    def get_available_packages(self) -> list[str]:
        names_file = "/var/cache/snapd/names"
        if not self._names_exists(names_file):
            logging.info(
                f"'{names_file}' does not exist. "
                "Checking for available snaps will be slower than usual. "
            )
            return []
        with open(names_file, "rb") as snap_names:
            return [package.decode().strip() for package in snap_names.readlines()]

    def has_available(self, package_name: str) -> bool:
        if package_name in self._not_available:
            return False
        if self._available_packages:
            return package_name in self._available_packages
        return not subprocess.run(
            [self._bin, "info", package_name], stderr=subprocess.DEVNULL
        ).returncode

    def has_installed(self, package_name: str) -> bool:
        return package_name in self.get_installed_packages()

    def _get_confinement(self, package_name: str) -> SnapdConfinement:
        response = self._session.get("http://snapd/v2/find", params={"q": package_name})
        for result in response.json()["result"]:
            if package_name != result["name"]:
                continue
            return SnapdConfinement(result["confinement"])
        raise RuntimeError(f"Unknown confinement for {package_name}")

    def install(self, packages: list[str], purge: bool = False) -> None:
        confinement_groups: dict[SnapdConfinement, list[str]] = {
            confinement: [] for confinement in SnapdConfinement
        }
        for package in packages:
            if self.has_installed(package):
                continue
            confinement = self._get_confinement(package)
            confinement_groups[confinement].append(package)
        for group, items in confinement_groups.items():
            if group == SnapdConfinement.CLASSIC:
                for item in items:
                    subprocess.check_call(
                        [self._sudo, self._bin, "install", "--classic", item]
                    )
            else:
                subprocess.check_call([self._sudo, self._bin, "install", *packages])

    def filter_removeable(self, packages: list[str]) -> list[str]:
        return packages

    def remove(self, packages: list[str], purge: bool = False) -> None:
        raise NotImplementedError("TODO")
