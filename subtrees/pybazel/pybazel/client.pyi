from typing import Iterable, Sequence

from pybazel.models.info import InfoKey as InfoKey
from pybazel.models.label import Label as Label

class BazelClient:
    def __init__(
        self,
        bazel_options: list[str] | None = ...,
        workspace: str | None = ...,
        output_base: str | None = ...,
    ) -> None: ...
    @property
    def bazel_options(self) -> list[str]: ...
    @bazel_options.setter
    def bazel_options(self, value: list[str]) -> None: ...
    @property
    def which_bazel(self) -> str: ...
    @which_bazel.setter
    def which_bazel(self, value: str) -> None: ...
    @property
    def output_base(self) -> str | None: ...
    @output_base.setter
    def output_base(self, value: str | None) -> None: ...
    @property
    def workspace(self) -> str: ...
    @workspace.setter
    def workspace(self, value: str) -> None: ...
    def build(
        self,
        labels: Iterable[Label | str],
        build_options: list[str] | None = ...,
    ) -> None: ...
    def info(
        self,
        key: InfoKey | None = ...,
        configuration_options: list[str] | None = ...,
    ) -> str: ...
    def query(
        self,
        query_string: str,
        query_options: Sequence[str] | None = ...,
    ) -> list[Label]: ...
