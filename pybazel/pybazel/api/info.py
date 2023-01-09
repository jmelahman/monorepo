from __future__ import annotations

import subprocess

from ..models.info import InfoKey
from .protocol import ApiProtocol


class InfoApiMixin(ApiProtocol):
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
