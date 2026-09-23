from __future__ import annotations

import os
import shutil
import subprocess
from pathlib import Path

from hatchling.builders.hooks.plugin.interface import BuildHookInterface

import manygo

# Central pin for the Go toolchain, so builds are as hermetic as PEP 517
# allows: the same compiler version everywhere, pinned in exactly one place.
# The static backend pins (hatchling, hatch-vcs, manygo) live in
# pyproject.toml [build-system].requires.
GO_BIN_PIN = "go-bin==1.27.1"

# Where the binary's version string is stamped: not `main`, since main.go is a
# shim. .goreleaser.yml stamps the same symbol.
VERSION_SYMBOL = "github.com/jmelahman/pkglint/internal/cli.version"


class GoBinaryBuildHook(BuildHookInterface):
    def dependencies(self) -> list[str]:
        """Wheel builds always use the pinned Go toolchain.

        Never probing PATH keeps builds hermetic: the binary is produced by
        the same compiler version on every machine. The sdist carries only
        sources, so it doesn't need one.
        """
        if self.target_name != "wheel":
            return []
        return [GO_BIN_PIN]

    def initialize(self, version, build_data) -> None:  # noqa: ANN001, ARG002
        if self.target_name != "wheel":
            # The sdist carries sources only; the binary is built when the
            # sdist is turned into a wheel on the installing machine.
            return
        build_data["pure_python"] = False
        goos = os.getenv("GOOS")
        goarch = os.getenv("GOARCH")
        if manygo.is_goos(goos) and manygo.is_goarch(goarch):
            build_data["tag"] = "py3-none-" + manygo.get_platform_tag(goos=goos, goarch=goarch)
        else:
            # Native build: let hatchling tag the wheel for this platform.
            build_data["infer_tag"] = True
        binary_name = self.config["binary_name"]

        go = shutil.which("go")
        if go is None:
            raise RuntimeError("go is required to build the pkglint binary")
        env = os.environ.copy()
        # Pure Go: static binaries on every target, no C toolchain needed.
        env["CGO_ENABLED"] = "0"

        # Always rebuild: a leftover binary from another target would be
        # packaged into a wheel tagged for the wrong platform.
        print(f"Building Go binary '{binary_name}'...")
        subprocess.check_call(  # noqa: S603
            [
                go, "build", "-trimpath",
                # `version` lives in internal/cli, behind the root shim, so
                # -X names that package and not main.
                "-ldflags",
                f"-s -w -X {VERSION_SYMBOL}={self.metadata.version}",
                "-o", str(Path(self.root) / binary_name),
                ".",
            ],
            cwd=self.root,
            env=env,
        )

        build_data["shared_scripts"] = {binary_name: binary_name}
