import os
import json
from typing import TYPE_CHECKING

from .errors import SnapifyConfigError
from .constants import SupportedDistro
from .manager.base import PackageManager
from .manager.pacman import Pacman
from .manager.snap import Snapd


class Snapifier:
    def __init__(self, noninteractive: bool) -> None:
        self._noninteractive = noninteractive
        self._distro = self._check_supported_distro()
        self._config = self._read_user_config()
        self.manager = self.get_host_package_manager()
        self.snap = Snapd(self._noninteractive, [])

    @staticmethod
    def _read_user_config() -> dict[SupportedDistro, list[str]]:
        config: dict[SupportedDistro, list[str]] = {}
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

    def _get_ignored_packages(self) -> list[str]:
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
