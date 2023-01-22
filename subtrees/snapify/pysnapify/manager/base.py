import abc
import typing

from . import utils


class PackageManager(abc.ABC):
    def __init__(
        self, noninteractive: bool, ignored_packages: list[str], name: str
    ) -> None:
        self.name = name
        self._not_available = ignored_packages
        self._noninteractive = noninteractive
        self._installed_packages: list[str] = []
        self._bin = utils.get_executable(name)
        self._sudo = utils.get_executable("sudo")

    @abc.abstractmethod
    def get_installed_packages(self) -> list[str]:
        raise NotImplementedError

    @abc.abstractmethod
    def filter_removeable(self, packages: list[str]) -> list[str]:
        raise NotImplementedError

    @abc.abstractmethod
    def has_available(self, package_name: str) -> bool:
        raise NotImplementedError

    @abc.abstractmethod
    def has_installed(self, package_name: str) -> bool:
        raise NotImplementedError

    @abc.abstractmethod
    def install(self, packages: list[str]) -> None:
        raise NotImplementedError

    @abc.abstractmethod
    def remove(self, packages: list[str], purge: bool = False) -> None:
        raise NotImplementedError
