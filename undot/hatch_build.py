from __future__ import annotations

import os
import shutil
import subprocess
import sys
from pathlib import Path

from hatchling.builders.hooks.plugin.interface import BuildHookInterface

import manygo

# Central pin registry for the build toolchain, so builds are as hermetic
# as PEP 517 allows: the same compiler and transpiler versions everywhere,
# each pinned in exactly one place.
#
# - The release workflow extracts SOLOD_VERSION and ZIGLANG_PIN with sed
#   instead of repeating them (.github/workflows/release.yml).
# - scripts/check-version.sh asserts SOLOD_VERSION matches so/go.mod and
#   so/gotest.mod.
# - The static backend pins (hatchling, hatch-vcs, manygo) live in
#   pyproject.toml [build-system].requires — they must be installed before
#   this file can be imported.
SOLOD_VERSION = "v0.3.0"
GO_BIN_PIN = "go-bin==1.26.6"
ZIGLANG_PIN = "ziglang==0.16.0"

# solod's os package needs POSIX, so Windows is not a supported target.
ZIG_TARGETS = {
    ("linux", "amd64"): "x86_64-linux-musl",
    ("linux", "arm64"): "aarch64-linux-musl",
    ("darwin", "amd64"): "x86_64-macos-none",
    ("darwin", "arm64"): "aarch64-macos-none",
}


class SolodBinaryBuildHook(BuildHookInterface):
    def dependencies(self) -> list[str]:
        """Wheel builds always use the pinned Go and zig toolchains.

        Never probing PATH keeps builds hermetic: the binary is produced by
        the same compiler versions on every machine. The sdist carries only
        sources, so it needs neither.
        """
        if self.target_name != "wheel":
            return []
        return [GO_BIN_PIN, ZIGLANG_PIN]

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

        # Always rebuild: a leftover binary from another target would be
        # packaged into a wheel tagged for the wrong platform.
        print(f"Building solod binary '{binary_name}'...")
        so = _ensure_so(self.root)
        env = os.environ.copy()
        env["PYTHON"] = sys.executable  # lets scripts/zigcc find ziglang
        # Compile with the pinned zig everywhere (native builds included) so
        # the toolchain doesn't vary by machine. An explicit $CC still wins
        # as a deliberate escape hatch.
        if "CC" not in env:
            env["CC"] = str(Path(self.root) / "scripts" / "zigcc")
        cflags = "-Os"
        if goos and goarch:
            try:
                target = ZIG_TARGETS[(goos, goarch)]
            except KeyError:
                raise RuntimeError(f"unsupported target {goos}/{goarch}") from None
            cflags = f"--target={target} -Os"
        env["CFLAGS"] = cflags
        subprocess.check_call(  # noqa: S603
            [so, "build", "-panic=exit", "-o", binary_name, "./so"],
            cwd=self.root,
            env=env,
        )

        build_data["shared_scripts"] = {binary_name: binary_name}


def _ensure_so(root: str) -> str:
    """Return a path to the pinned solod `so` tool, installing it if needed.

    Always installs the pinned version rather than trusting a `so` from
    PATH, into a version-keyed cache so re-builds are fast and a version
    bump can't reuse a stale binary. Uses `go` from PATH — inside pip's
    build isolation that is the pinned go-bin package.
    """
    gobin = Path(root) / ".so-bin" / SOLOD_VERSION
    so = gobin / "so"
    if so.exists():
        return str(so)
    gobin.mkdir(parents=True, exist_ok=True)
    go = shutil.which("go")
    if go is None:
        raise RuntimeError("go is required to install the solod toolchain")
    env = os.environ.copy()
    env["GOBIN"] = str(gobin)
    # GOOS/GOARCH describe the wheel's target; the so tool itself must be a
    # host binary (and `go install` refuses cross-compiles with GOBIN set).
    env.pop("GOOS", None)
    env.pop("GOARCH", None)
    print(f"Installing solod.dev/cmd/so@{SOLOD_VERSION}...")
    subprocess.check_call(  # noqa: S603
        [go, "install", f"solod.dev/cmd/so@{SOLOD_VERSION}"],
        env=env,
    )
    return str(so)
