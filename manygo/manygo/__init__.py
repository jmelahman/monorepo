from __future__ import annotations

from typing import get_args, Literal, TypeGuard

# Supported Go operating systems with known platform tags
GOOS = Literal["darwin", "linux", "windows"]

# Supported Go architectures with known platform tags
GOARCH = Literal["amd64", "arm64", "386", "arm", "s390x", "ppc64le", "ppc64", "riscv64", "loong64"]

_GOOS_VALUES: tuple[str, ...] = get_args(GOOS)
_GOARCH_VALUES: tuple[str, ...] = get_args(GOARCH)

# Platform tags for non-Linux targets. macOS tags reflect the minimum OS
# version supported by current Go releases (Go 1.23+ requires macOS 11).
_PLATFORM_MAP: dict[tuple[str, str], str] = {
    ("darwin", "amd64"): "macosx_11_0_x86_64",
    ("darwin", "arm64"): "macosx_11_0_arm64",
    ("windows", "386"): "win32",
    ("windows", "arm64"): "win_arm64",
    ("windows", "amd64"): "win_amd64",
}

# Linux GOARCH -> (manylinux glibc version, Python machine name). The glibc
# version is the oldest release supporting the architecture, floored at 2.17.
# "arm" assumes GOARM=7 with hard-float, matching Go's default when
# cross-compiling.
_LINUX_ARCH_MAP: dict[str, tuple[str, str]] = {
    "amd64": ("2_17", "x86_64"),
    "arm64": ("2_17", "aarch64"),
    "386": ("2_17", "i686"),
    "arm": ("2_17", "armv7l"),
    "s390x": ("2_17", "s390x"),
    "ppc64le": ("2_17", "ppc64le"),
    "ppc64": ("2_17", "ppc64"),
    "riscv64": ("2_27", "riscv64"),
    "loong64": ("2_36", "loongarch64"),
}


def is_goos(value: str | None) -> TypeGuard[GOOS]:
    return value in _GOOS_VALUES


def is_goarch(value: str | None) -> TypeGuard[GOARCH]:
    return value in _GOARCH_VALUES


def get_platform_tag(goos: GOOS, goarch: GOARCH) -> str:
    """Convert GOOS and GOARCH to a valid Python platform tag.

    This function provides a mapping between Go's platform identifiers
    (operating system and architecture) and Python platform tags used
    in packaging and distribution.

    Supported platforms are derived from the Go toolchain's supported
    platforms, which can be listed via `$ go tool dist list`. See also,
    https://go.dev/doc/install/source#environment. Python's platform tags
    are described in https://packaging.python.org/en/latest/specifications/platform-compatibility-tags/#platform-tag.

    Linux tags assume a statically linked binary (CGO_ENABLED=0), which has
    no glibc dependency; the manylinux version is only the lowest one valid
    for the architecture. Binaries built with cgo depend on the glibc of the
    build host and may need a newer tag.

    Args:
        goos (GOOS): The operating system identifier
        goarch (GOARCH): The architecture identifier

    Returns:
        str: A Python platform tag suitable for wheel or other packaging

    Raises:
        ValueError: If no platform tag can be generated for the given combination

    """
    if (goos, goarch) in _PLATFORM_MAP:
        return _PLATFORM_MAP[(goos, goarch)]

    if goos == "linux" and goarch in _LINUX_ARCH_MAP:
        glibc, machine = _LINUX_ARCH_MAP[goarch]
        return f"manylinux_{glibc}_{machine}"

    msg = f"No platform tag for {goos}/{goarch}"
    raise ValueError(msg)
