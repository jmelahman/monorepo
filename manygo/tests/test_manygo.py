from __future__ import annotations

from typing import get_args

import pytest

import manygo


def test_is_goarch() -> None:
    assert manygo.is_goarch("amd64")
    assert not manygo.is_goarch("foo")


def test_is_goarch_all_values() -> None:
    for goarch in get_args(manygo.GOARCH):
        assert manygo.is_goarch(goarch)


def test_is_goos() -> None:
    assert manygo.is_goos("linux")
    assert not manygo.is_goos("bar")


def test_get_platform_tag_darwin() -> None:
    assert manygo.get_platform_tag("darwin", "amd64") == "macosx_11_0_x86_64"
    assert manygo.get_platform_tag("darwin", "arm64") == "macosx_11_0_arm64"
    with pytest.raises(ValueError, match="No platform tag for darwin/386"):
        manygo.get_platform_tag("darwin", "386")


def test_get_platform_tag_linux() -> None:
    assert manygo.get_platform_tag("linux", "amd64") == "manylinux_2_17_x86_64"
    assert manygo.get_platform_tag("linux", "arm64") == "manylinux_2_17_aarch64"
    assert manygo.get_platform_tag("linux", "386") == "manylinux_2_17_i686"
    assert manygo.get_platform_tag("linux", "arm") == "manylinux_2_17_armv7l"


def test_get_platform_tag_windows() -> None:
    assert manygo.get_platform_tag("windows", "amd64") == "win_amd64"
    assert manygo.get_platform_tag("windows", "arm64") == "win_arm64"
    assert manygo.get_platform_tag("windows", "386") == "win32"


def test_get_platform_tag_other_architectures() -> None:
    assert manygo.get_platform_tag("linux", "s390x") == "manylinux_2_17_s390x"
    assert manygo.get_platform_tag("linux", "ppc64le") == "manylinux_2_17_ppc64le"
    assert manygo.get_platform_tag("linux", "ppc64") == "manylinux_2_17_ppc64"
    assert manygo.get_platform_tag("linux", "riscv64") == "manylinux_2_27_riscv64"
    assert manygo.get_platform_tag("linux", "loong64") == "manylinux_2_36_loongarch64"

    # Unsupported OS
    with pytest.raises(ValueError, match="No platform tag for freebsd/amd64"):
        manygo.get_platform_tag("freebsd", "amd64")  # ty: ignore[invalid-argument-type]

    # Unsupported architecture for a supported OS
    with pytest.raises(ValueError, match="No platform tag for darwin/ppc64"):
        manygo.get_platform_tag("darwin", "ppc64")

    # Unsupported architecture for Linux
    with pytest.raises(ValueError, match="No platform tag for linux/mips"):
        manygo.get_platform_tag("linux", "mips")  # ty: ignore[invalid-argument-type]

    # Unsupported combination
    with pytest.raises(ValueError, match="No platform tag for windows/s390x"):
        manygo.get_platform_tag("windows", "s390x")
