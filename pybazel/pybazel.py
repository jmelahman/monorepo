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

    def run(
        self, target: str, run_options: list[str] | None = None, check=True
    ) -> subproccess.CompletedProcess:
        run_options = run_options or []
        cmd = [
            self._bazel_bin,
            *self.bazel_options,
            "run",
            *run_options,
        ]
        return subprocess.run(cmd, cwd=self.workspace_path, check=check)

    def sync(
        self, sync_options: list[str] | None = None, check: bool = True
    ) -> subprocess.CompletedProcess:
        sync_options = sync_options or []
        cmd = [
            self._bazel_bin,
            *self.bazel_options,
            "sync",
            *sync_options,
        ]
        return subprocess.run(cmd, cwd=self.workspace_path, check=check)
