from __future__ import annotations

import os
import subprocess


class BazelClient:
    def __init__(
        self, workspace_path: str = "", bazel_options: list[str] = None
    ) -> None:
        self.bazel_options = bazel_options or []
        self.workspace_path = workspace_path or os.environ["BUILD_WORKSPACE_DIRECTORY"]
        self._bazel_bin = "bazel"

    def sync(sync_options, check=True):
        cmd = [
            self._bazel_bin,
            *self.bazel_options,
            "sync",
            *sync_options,
        ]
        return subprocess.check_call(cmd, cwd=self.workspace_path)
