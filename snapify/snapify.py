#!/usr/bin/env python3.10
import abc
import argparse
import enum
import json
import logging
import os
import socket
import subprocess
import shutil
import sys
import typing

import requests
import urllib3

logging.basicConfig(
    format="%(levelname)s: %(message)s", level=os.environ.get("LOGLEVEL", "INFO")
)


class SnapifyConfigError(Exception):
    __module__ = "builtins"


class SupportedDistro(enum.Enum):
    ARCH = "arch"
    MANJARO = "manjaro"


class PackageManager(abc.ABC):
    def __init__(
        self, noninteractive: bool, ignored_packages: typing.List[str], name: str
    ) -> None:
        def _get_executable(bin_name: str) -> str:
            executable = shutil.which(bin_name)
            assert isinstance(executable, str)
            return executable

        self.name = name
        self._not_available = ignored_packages
        self._noninteractive = noninteractive
        self._installed_packages: typing.List[str] = []
        self._bin = _get_executable(name)
        self._sudo = _get_executable("sudo")

    @abc.abstractmethod
    def get_installed_packages(self) -> typing.List[str]:
        raise NotImplementedError

    @abc.abstractmethod
    def filter_removeable(self, packages: typing.List[str]) -> typing.List[str]:
        raise NotImplementedError

    @abc.abstractmethod
    def has_available(self, package_name: str) -> bool:
        raise NotImplementedError

    @abc.abstractmethod
    def has_installed(self, package_name: str) -> bool:
        raise NotImplementedError

    @abc.abstractmethod
    def install(self, packages: typing.List[str]) -> None:
        raise NotImplementedError

    @abc.abstractmethod
    def remove(self, packages: typing.List[str], purge: bool = False) -> None:
        raise NotImplementedError


class Pacman(PackageManager):
    def __init__(
        self,
        noninteractive: bool,
        ignored_packages: typing.List[str],
        name: str = "pacman",
    ) -> None:
        super().__init__(noninteractive, ignored_packages, name)

    def get_installed_packages(self) -> typing.List[str]:
        if self._installed_packages != []:
            return self._installed_packages
        self._installed_packages = [
            package
            for package in subprocess.check_output([self._bin, "-Qq"])
            .decode()
            .strip()
            .split("\n")
            if package not in self._not_available
        ]
        return self._installed_packages

    def has_available(self, package_name: str) -> bool:
        raise NotImplementedError("TODO")

    def has_installed(self, package_name: str) -> bool:
        return package_name in self.get_installed_packages()

    def install(self, packages: typing.List[str]) -> None:
        raise NotImplementedError("TODO")

    def filter_removeable(self, packages: typing.List[str]) -> typing.List[str]:
        dependency_query = subprocess.run(
            [self._bin, "-Qqt", *packages], stdout=subprocess.PIPE
        )
        if dependency_query.returncode:
            logging.info(
                f"The following packages were unable to be snapified: {' '.join(packages)}"
            )
            return []
        return dependency_query.stdout.decode().strip().split("\n")

    def remove(self, packages: typing.List[str], purge: bool = False) -> None:
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
        ):  # Allow user to decline removal gracefully.
            sys.exit(1)


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
        proxies: typing.Optional[typing.Dict[str, str]] = None,
    ) -> SnapdConnectionPool:
        return SnapdConnectionPool()


class SnapdConfinement(enum.Enum):
    CLASSIC = "classic"
    STRICT = "strict"


class Snapd(PackageManager):
    def __init__(
        self,
        noninteractive: bool,
        ignored_packages: typing.List[str],
        name: str = "snap",
    ) -> None:
        super().__init__(noninteractive, ignored_packages, name)
        self._not_available = self._not_available + ["snapd"]
        self._available_packages = self.get_available_packages()
        self._installed_packages = self.get_installed_packages()
        self._session = requests.Session()
        self._session.mount("http://snapd/", SnapdAdapter())

    def get_installed_packages(self) -> typing.List[str]:
        if self._installed_packages != []:
            return self._installed_packages
        snap_list = subprocess.check_output([self._bin, "list"]).decode().split("\n")
        snap_list.pop(0)  # Remove header
        self._installed_packages = [package.split(" ")[0] for package in snap_list]
        return self._installed_packages

    def get_available_packages(self) -> typing.List[str]:
        names_file = "/var/cache/snapd/names"
        if not os.path.exists(names_file):
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

    def install(self, packages: typing.List[str], purge: bool = False) -> None:
        confinement_groups: typing.Dict[SnapdConfinement, typing.List[str]] = {
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

    def filter_removeable(self, packages: typing.List[str]) -> typing.List[str]:
        return packages

    def remove(self, packages: typing.List[str], purge: bool = False) -> None:
        raise NotImplementedError("TODO")


class Snapifier:
    def __init__(self, noninteractive: bool) -> None:
        self._noninteractive = noninteractive
        self._distro = self._check_supported_distro()
        self._config = self._read_user_config()
        self.manager = self.get_host_package_manager()
        self.snap = Snapd(self._noninteractive, [])

    @staticmethod
    def _read_user_config() -> typing.Dict[SupportedDistro, typing.List[str]]:
        config: typing.Dict[SupportedDistro, typing.List[str]] = {}
        config_path = os.path.join(os.path.expanduser("~/.config/snapify"), "config")
        if not os.path.exists(config_path):
            return config
        with open(config_path) as json_config:
            raw_config = json.load(json_config)
            for distro, ignorelist in raw_config.items():
                if not any([distro == d.value for d in SupportedDistro]):
                    raise SnapifyConfigError(
                        "'{distro}' in Snapify config is not a supported distro: {supported}".format(
                            distro=distro,
                            supported=" ".join([d.value for d in SupportedDistro]),
                        )
                    )
                if not isinstance(ignorelist, list):
                    raise SnapifyConfigError(
                        f"Ignore list for '{distro}' is not a list."
                    )
                for package in ignorelist:
                    if not isinstance(package, str):
                        raise SnapifyConfigError(
                            f"Ignored package '{package}' for '{distro}' must be a string."
                        )
                config[SupportedDistro(distro)] = ignorelist
        return config

    @staticmethod
    def _check_supported_distro() -> SupportedDistro:
        os_id = None
        with open("/etc/os-release", "rb") as release:
            for line in release.readlines():
                if line.startswith(b"ID"):
                    os_id = line.split(b"=")[1].strip().decode()
                    break
        if not os_id:
            raise RuntimeError("Unable to determine host distro")
        return SupportedDistro(os_id)

    def _get_ignored_packages(self) -> typing.List[str]:
        return self._config.get(self._distro, [])

    def get_host_package_manager(self) -> PackageManager:
        ignored_packages = self._get_ignored_packages()
        if self._distro == SupportedDistro.ARCH:
            return Pacman(self._noninteractive, ignored_packages)
        elif self._distro == SupportedDistro.MANJARO:
            return Pacman(self._noninteractive, ignored_packages)
        raise RuntimeError(
            f"Unable register host package manager for: {self._distro.value}"
        )


def get_parsed_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--noninteractive",
        action="store_true",
        default=False,
        help="run in noninteractive mode",
    )
    return parser.parse_args()


def main() -> None:
    args = get_parsed_args()
    snapifier = Snapifier(args.noninteractive)
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


if __name__ == "__main__":
    main()
