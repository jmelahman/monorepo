from __future__ import annotations

import pytest

from manygo import get_platform_tag


def test_get_platform_tag_darwin() -> None:
    assert get_platform_tag("darwin", "amd64") == "macosx_10_12_x86_64"
    assert get_platform_tag("darwin", "arm64") == "macosx_11_0_arm64"
    with pytest.raises(ValueError):  # noqa: PT011
        get_platform_tag("darwin", "386")


def test_get_platform_tag_linux() -> None:
    assert get_platform_tag("linux", "amd64") == "manylinux_2_17_x86_64"
    assert get_platform_tag("linux", "arm64") == "manylinux_2_17_aarch64"
    assert get_platform_tag("linux", "386") == "manylinux_2_17_i686"
    assert get_platform_tag("linux", "arm") == "manylinux_2_17_armv7l"


def test_get_platform_tag_windows() -> None:
    assert get_platform_tag("windows", "amd64") == "win_amd64"
    assert get_platform_tag("windows", "arm64") == "win_arm64"
    assert get_platform_tag("windows", "386") == "win32"


def test_get_platform_tag_other_architectures() -> None:
    assert get_platform_tag("linux", "s390x") == "manylinux_2_17_s390x"
    assert get_platform_tag("linux", "ppc64le") == "manylinux_2_17_ppc64le"
    assert get_platform_tag("linux", "ppc64") == "manylinux_2_17_ppc64"

    # Unsupported OS
    with pytest.raises(ValueError):  # noqa: PT011
        get_platform_tag("freebsd", "amd64")

    # Unsupported architecture for a supported OS
    with pytest.raises(ValueError):  # noqa: PT011
        get_platform_tag("darwin", "ppc64")

    # Unsupported architecture for Linux
    with pytest.raises(ValueError):  # noqa: PT011
        get_platform_tag("linux", "mips")

    # Unsupported combination
    with pytest.raises(ValueError):  # noqa: PT011
        get_platform_tag("windows", "s390x")
