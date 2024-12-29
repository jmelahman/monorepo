from __future__ import annotations

from typing import Literal

# https://go.dev/doc/install/source#environment
# See also, `$ go tools dist list`
def get_platform_tag(
    goos: Literal['aix', 'android', 'darwin', 'freebsd', 'ios', 'js', 'linux', 'netbsd', 'openbsd', 'plan9', 'solaris', 'windows'],
    goarch: Literal['amd64', 'arm', 'arm64', 'mips', 'mips64', 'ppc64', 'riscv64', 's390x', 'wasm', '386']
) -> str:
    """
    Convert GOOS and GOARCH to a valid Python platform tag.

    Args:
        goos (str): The operating system
        goarch (str): The architecture

    Returns:
        str: A Python platform tag

    Raises:
        ValueError: If the combination is not supported
    """
    # Mapping of special cases and conversions
    platform_map = {
        ('darwin', 'amd64'): 'macosx_10_9_x86_64',
        ('darwin', 'arm64'): 'macosx_11_0_arm64',
        ('linux', 'amd64'): 'manylinux_2_17_x86_64',
        ('linux', 'arm64'): 'manylinux_2_17_aarch64',
        ('windows', 'amd64'): 'win_amd64',
        ('windows', '386'): 'win32',
        ('linux', '386'): 'linux_i686',
        ('linux', 'arm'): 'linux_armv7l',
    }

    # Check for direct mapping first
    if (goos, goarch) in platform_map:
        return platform_map[(goos, goarch)]

    # Generic fallback conversion
    os_map = {
        'linux': 'linux',
        'windows': 'win',
        'darwin': 'macosx',
    }

    arch_map = {
        'amd64': 'x86_64',
        'arm64': 'aarch64',
        '386': 'i686',
        'arm': 'armv7l',
    }

    # Try to construct a generic tag
    if goos in os_map and goarch in arch_map:
        return f'{os_map[goos]}_{arch_map[goarch]}'

    # If no mapping found, raise an error
    raise ValueError(f'No platform tag for {goos}/{goarch}')
