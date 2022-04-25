#!/usr/bin/env python3.10
import abc
import enum
import logging
import os
import socket
import subprocess
import shutil
import sys
import typing

# TODO(jamison): Library stubs not installed for "requests" (or incompatible with Python 3.10)  [import]
import requests  # type: ignore[import]

# TODO(jamison): Cannot find implementation or library stub for module named "urllib3"  [import]
import urllib3  # type: ignore[import]

logging.basicConfig(
    format="%(levelname)s: %(message)s", level=os.environ.get("LOGLEVEL", "INFO")
)


class SupportedDistro(enum.Enum):
    ARCH = "arch"


class PackageManager(abc.ABC):
    def __init__(self, name: str) -> None:
        def _get_executable(bin_name: str) -> str:
            executable = shutil.which(bin_name)
            assert isinstance(executable, str)
            return executable

        self.name = name
        self._bin = _get_executable(name)
        self._sudo = _get_executable("sudo")

    @abc.abstractmethod
    def get_installed_packages(self) -> typing.List[str]:
        raise NotImplementedError

    @abc.abstractmethod
    def has_available(self, package_name: str) -> bool:
        raise NotImplementedError

    @abc.abstractmethod
    def install(self, packages: typing.List[str]) -> None:
        raise NotImplementedError

    @abc.abstractmethod
    def remove(
        self, packages: typing.List[str], purge: bool = False
    ) -> typing.List[str]:
        raise NotImplementedError


class Pacman(PackageManager):
    def __init__(self, name: str = "pacman") -> None:
        super().__init__(name)

    def get_installed_packages(self) -> typing.List[str]:
        return subprocess.check_output([self._bin, "-Qq"]).decode().strip().split("\n")

    def has_available(self, package_name: str) -> bool:
        raise NotImplementedError("TODO")

    def install(self, packages: typing.List[str]) -> None:
        raise NotImplementedError("TODO")

    def remove(
        self, packages: typing.List[str], purge: bool = False
    ) -> typing.List[str]:
        dependency_query = subprocess.run(
            [self._bin, "-Qqt", *packages], stdout=subprocess.PIPE
        )
        if dependency_query.returncode:
            logging.info(
                f"The following packages were unable to be snapified: {' '.join(packages)}"
            )
            return []
        removed_packages = dependency_query.stdout.decode().strip().split("\n")
        logging.info(f"Removing the following packages: {' '.join(removed_packages)}")
        try:
            subprocess.check_call(
                [
                    self._sudo,
                    self._bin,
                    f"-Rs{'n' if purge else ''}",
                    *removed_packages,
                ],
            )
        except subprocess.CalledProcessError:  # Allow user to decline removal gracefully.
            sys.exit(1)
        return removed_packages


# TODO(jamison): Base type HTTPConnection becomes "Any" due to an unfollowed import  [no-any-unimported]
class SnapdConnection(urllib3.connection.HTTPConnection):  # type: ignore[no-any-unimported]
    def __init__(self) -> None:
        super().__init__("localhost")

    def connect(self) -> None:
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.connect("/run/snapd.socket")


# TODO(jamison): Base type HTTPConnectionPool becomes "Any" due to an unfollowed import  [no-any-unimported]
class SnapdConnectionPool(urllib3.connectionpool.HTTPConnectionPool):  # type: ignore[no-any-unimported]
    def __init__(self) -> None:
        super().__init__("localhost")

    def _new_conn(self) -> SnapdConnection:
        return SnapdConnection()


# TODO(jamison): Base type HTTPAdapter becomes "Any" due to an unfollowed import  [no-any-unimported]
class SnapdAdapter(requests.adapters.HTTPAdapter):  # type: ignore[no-any-unimported]
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
    def __init__(self, name: str = "snap") -> None:
        super().__init__(name)
        self._never_available = ["snapd"]
        self._available_packages = self.get_available_packages()
        self._session = requests.Session()
        self._session.mount("http://snapd/", SnapdAdapter())

    def get_installed_packages(self) -> typing.List[str]:
        raise NotImplementedError("TODO")

    def get_available_packages(self) -> typing.List[str]:
        names_file = "/var/cache/snapd/names"
        if not os.path.exists(names_file):
            logging.info(
                f"{names_file} does not exist."
                "Checking for available snaps will be slower than usual."
            )
            return []
        with open(names_file, "rb") as snap_names:
            return [package.decode().strip() for package in snap_names.readlines()]

    def has_available(self, package_name: str) -> bool:
        if package_name in self._never_available:
            return False
        if self._available_packages:
            return package_name in self._available_packages
        return not subprocess.run(
            [self._bin, "info", package_name], stderr=subprocess.DEVNULL
        ).returncode

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

    def remove(
        self, packages: typing.List[str], purge: bool = False
    ) -> typing.List[str]:
        raise NotImplementedError("TODO")


def check_supported_distro() -> SupportedDistro:
    os_id = None
    with open("/etc/os-release", "rb") as release:
        for line in release.readlines():
            if line.startswith(b"ID"):
                os_id = line.split(b"=")[1].strip().decode()
                break
    if not os_id:
        raise RuntimeError("Unable to determine host distro")
    return SupportedDistro(os_id)


def get_host_package_manager(distro: SupportedDistro) -> PackageManager:
    if distro == SupportedDistro.ARCH:
        return Pacman()
    raise RuntimeError(f"Unable register host package manager for: {distro.value}")


def main() -> None:
    distro = check_supported_distro()
    host_manager = get_host_package_manager(distro)
    snap = Snapd()
    host_packages = host_manager.get_installed_packages()
    portable_packages = [
        package for package in host_packages if snap.has_available(package)
    ]
    if not portable_packages:
        return
    removed_packages = host_manager.remove(portable_packages)
    if not removed_packages:
        return
    snap.install(removed_packages)


if __name__ == "__main__":
    main()
