from __future__ import annotations

from typing import Protocol


class ApiProtocol(Protocol):
    @property
    def bazel_options(self) -> list[str]:
        ...

    @property
    def which_bazel(self) -> str:
        ...

    @property
    def workspace(self) -> str:
        ...
