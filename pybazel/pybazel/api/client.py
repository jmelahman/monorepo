from __future__ import annotations

import logging
import os
import subprocess

from ..errors import PyBazelException
from ..models.info import InfoKey
from ..models.label import Label
from .protocol import ApiProtocol

log = logging.getLogger(__name__)


class APIClient(ApiProtocol):
    def __init__(
        self, bazel_options: list[str] | None = None, workspace: str | None = None
    ) -> None:
        self.bazel_options = bazel_options or []
        self.which_bazel = "bazel"
        self.workspace = workspace  # type: ignore[assignment] # https://github.com/python/mypy/issues/3004

    @property
    def bazel_options(self) -> list[str]:
        return self._bazel_options

    @bazel_options.setter
    def bazel_options(self, value: list[str]) -> None:
        self._bazel_options = value

    @property
    def which_bazel(self) -> str:
        return self._which_bazel

    @which_bazel.setter
    def which_bazel(self, value: str) -> None:
        self._which_bazel = value

    @property
    def workspace(self) -> str:
        return self._workspace

    @workspace.setter
    def workspace(self, value: str | None) -> None:
        if not value:
            build_workspace = os.getenv("BUILD_WORKSPACE_DIRECTORY")
            if build_workspace:
                value = build_workspace
            else:
                # Infer the workspace from the current directory.
                self._workspace = os.getcwd()
                # Should this not invoke info() and instead parse the file tree?
                value = self.info(InfoKey.workspace)
                logging
        if not value:
            raise PyBazelException("Unable to infer workspace.")
        self._workspace = value

    def info(
        self: ApiProtocol,
        key: InfoKey | None = None,
        configuration_options: list[str] | None = None,
    ) -> str:
        info_command = [self.which_bazel, *self.bazel_options, "info"]
        info_command += [key.value] if key else []
        info_command += configuration_options or []
        return (
            subprocess.check_output(
                info_command, cwd=self.workspace, stderr=subprocess.DEVNULL
            )
            .decode()
            .rstrip()
        )

    def query(
        self,
        query_string: str,
        query_options: list[str] | None = None,
    ) -> list[Label]:
        labels: list[Label] = []
        query_command = [self.which_bazel, *self.bazel_options, "query"]
        query_command += query_options or []
        query_command += [query_string]
        output = (
            subprocess.check_output(
                query_command, cwd=self.workspace, stderr=subprocess.DEVNULL
            )
            .decode()
            .rstrip()
        )
        for line in output.rsplit("\n"):
            labels.append(Label(line))
        return labels
