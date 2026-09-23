from __future__ import annotations

import os
import shutil
import subprocess
from pathlib import Path

from hatchling.builders.hooks.plugin.interface import BuildHookInterface

import manygo


class GoBinaryBuildHook(BuildHookInterface):
    def initialize(self, version, build_data) -> None:  # noqa: ANN001, ARG002
        build_data["pure_python"] = False
        goos = os.getenv("GOOS")
        goarch = os.getenv("GOARCH")
        if manygo.is_goos(goos) and manygo.is_goarch(goarch):
            build_data["tag"] = "py3-none-" + manygo.get_platform_tag(goos=goos, goarch=goarch)
        binary_name = self.config["binary_name"]
        version = os.getenv("VERSION") or _resolve_version()

        web_dir = Path(self.root) / "web"
        dist_dir = web_dir / "dist"
        if not dist_dir.exists() or not any(dist_dir.iterdir()):
            print("Building frontend...")
            npm = shutil.which("npm")
            if npm is None:
                raise RuntimeError("npm is required to build the kanban frontend")
            install_cmd = "ci" if (web_dir / "package-lock.json").exists() else "install"
            subprocess.check_call([npm, install_cmd], cwd=web_dir)  # noqa: S603
            subprocess.check_call([npm, "run", "build"], cwd=web_dir)  # noqa: S603

        if not os.path.exists(binary_name):
            print(f"Building Go binary '{binary_name}'...")
            devcontainer_image = (
                os.getenv("KANBAN_DEVCONTAINER_IMAGE")
                or "lahmanja/kanban-devcontainer:latest"
            )
            ldflags = (
                f"-X github.com/jmelahman/kanban/cmd/server.version={version} "
                f"-X github.com/jmelahman/kanban/internal/docker.BuiltinImage={devcontainer_image} "
                "-s -w"
            )
            subprocess.check_call(  # noqa: S603
                [
                    "go",
                    "build",
                    "-tags=embed",
                    "-trimpath",
                    f"-ldflags={ldflags}",
                    "-o",
                    binary_name,
                ],
            )

        build_data["shared_scripts"] = {binary_name: binary_name}


def _resolve_version() -> str:
    """Pick a single human-readable version string for the Go ldflag.

    Prefers the GitHub Actions context (GITHUB_REF_NAME for tag pushes, or
    "<branch>-<shortsha>" otherwise). Falls back to `git describe` so local
    `pip install .` / hatch builds also self-describe. "dev" if nothing
    works.
    """
    ref = os.getenv("GITHUB_REF_NAME")
    sha = os.getenv("GITHUB_SHA")
    if ref and sha:
        if os.getenv("GITHUB_REF_TYPE") == "tag":
            return ref
        return f"{ref}-{sha[:7]}"
    try:
        out = subprocess.check_output(  # noqa: S603
            ["git", "describe", "--tags", "--always", "--dirty"],  # noqa: S607
            stderr=subprocess.DEVNULL,
        )
        return out.decode().strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return "dev"
