from __future__ import annotations

import logging
import os
import subprocess

from pybazel.api.protocol import ApiProtocol
from pybazel.errors import PyBazelException
from pybazel.models.info import InfoKey
from pybazel.models.label import Label

log = logging.getLogger(__name__)


class APIClient(ApiProtocol):
    def __init__(
        self, bazel_options: list[str] | None = None, workspace: str | None = None
    ) -> None:
        self.bazel_options = bazel_options or []
        self.which_bazel = "bazel"
        self.workspace = workspace or self._get_inferred_workspace()

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
    def workspace(self, value: str) -> None:
        self._workspace = value

    def _get_inferred_workspace(self) -> str:
        build_workspace = os.getenv("BUILD_WORKSPACE_DIRECTORY", "")
        if build_workspace:
            workspace = build_workspace
        else:
            # Infer the workspace from the current directory.
            self._workspace = os.getcwd()
            # TODO: Once InfoKey is an enum.
            # value = self.info(InfoKey.workspace)
            workspace = self.info(InfoKey("workspace"))
        if not workspace:
            raise PyBazelException("Unable to infer workspace.")
        return workspace

    # TODO: -> dict[InfoKey, str]
    def info(
        self: ApiProtocol,
        key: InfoKey | None = None,
        configuration_options: list[str] | None = None,
    ) -> str:
        info_command = [self.which_bazel, *self.bazel_options, "info"]
        info_command += [key.value] if key else []
        info_command += configuration_options or []
        info = (
            subprocess.check_output(
                info_command, cwd=self.workspace, stderr=subprocess.DEVNULL
            )
            .decode()
            .rstrip()
        )
        return info

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
