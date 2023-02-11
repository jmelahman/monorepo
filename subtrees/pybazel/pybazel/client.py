from __future__ import annotations

import os
import subprocess
from typing import Iterable, Sequence

from pybazel.errors import PyBazelException
from pybazel.models.info import InfoKey
from pybazel.models.label import Label
from pybazel.utils import logger

log = logger.getLogger(__name__)


class BazelClient:
    def __init__(
        self,
        bazel_options: list[str] | None = None,
        workspace: str | None = None,
        output_base: str | None = None,
    ) -> None:
        self.output_base = output_base
        self.bazel_options = bazel_options or []
        self.which_bazel = "bazel"
        self.workspace = workspace or self._get_inferred_workspace()

    @property
    def bazel_options(self) -> list[str]:
        bazel_options = self._bazel_options
        if self.output_base:
            bazel_options.append(f"--output_base={self.output_base}")
        return bazel_options

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
    def output_base(self) -> str | None:
        return self._output_base

    @output_base.setter
    def output_base(self, value: str | None) -> None:
        self._output_base = value

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

    def build(
        self,
        labels: Iterable[Label | str],
        build_options: list[str] | None = None,
    ) -> None:
        build_command = [self.which_bazel, *self.bazel_options, "build"]
        build_command += build_options or []
        build_command += list(labels)
        print(build_command)
        subprocess.run(
            build_command,
            cwd=self.workspace,
            check=True,
        )

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
        query_options: Sequence[str] | None = None,
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
