from __future__ import annotations

import os
import subprocess


class BazelClient:
    def __init__(
        self, workspace_path: str = "", bazel_options: list[str] | None = None
    ) -> None:
        self.bazel_options = bazel_options or []
        self.workspace_path = workspace_path or os.environ["BUILD_WORKSPACE_DIRECTORY"]
        self._bazel_bin = "bazel"

    def sync(self, sync_options: list[str], check: bool =True) -> subprocess.CompletedProcess:
        cmd = [
            self._bazel_bin,
            *self.bazel_options,
            "sync",
            *sync_options,
        ]
        return subprocess.run(cmd, cwd=self.workspace_path, check=check)
