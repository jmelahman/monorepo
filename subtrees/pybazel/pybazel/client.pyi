from typing import Iterable, Sequence, Union

from _typeshed import Incomplete as Incomplete

from pybazel.models.info import InfoKey as InfoKey
from pybazel.models.label import Label as Label

log: Incomplete

class BazelClient:
    def __init__(
        self,
        bazel_options: Union[list[str], None] = ...,
        workspace: Union[str, None] = ...,
        output_base: Union[str, None] = ...,
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
    def output_base(self) -> Union[str, None]: ...
    @output_base.setter
    def output_base(self, value: Union[str, None]) -> None: ...
    @property
    def workspace(self) -> str: ...
    @workspace.setter
    def workspace(self, value: str) -> None: ...
    def build(
        self,
        labels: Iterable[Union[Label, str]],
        build_options: Union[list[str], None] = ...,
    ) -> None: ...
    def info(
        self,
        key: Union[InfoKey, None] = ...,
        configuration_options: Union[list[str], None] = ...,
    ) -> str: ...
    def query(
        self, query_string: str, query_options: Union[Sequence[str], None] = ...
    ) -> list[Label]: ...
