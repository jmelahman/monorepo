from __future__ import annotations

from typing import Literal

# Supported Go operating systems with known platform tags
GOOS = Literal['darwin', 'linux', 'windows']

# Supported Go architectures with known platform tags
GOARCH = Literal['amd64', 'arm64', '386', 'arm']

def get_platform_tag(
    goos: GOOS,
    goarch: GOARCH
) -> str:
    """
    Convert GOOS and GOARCH to a valid Python platform tag.

    This function provides a mapping between Go's platform identifiers
    (operating system and architecture) and Python platform tags used
    in packaging and distribution.

    Supported platforms are derived from the Go toolchain's supported
    platforms, which can be listed via `$ go tools dist list`. See also,
    https://go.dev/doc/install/source#environment. Python's platform tags
    are described in https://packaging.python.org/en/latest/specifications/platform-compatibility-tags/#platform-tag.

    Args:
        goos (GOOS): The operating system identifier
        goarch (GOARCH): The architecture identifier

    Returns:
        str: A Python platform tag suitable for wheel or other packaging

    Raises:
        ValueError: If no platform tag can be generated for the given combination
    """
    # Mapping of special cases and conversions
    platform_map = {
        ('darwin', 'amd64'): 'macosx_10_12_x86_64',
        ('darwin', 'arm64'): 'macosx_11_0_arm64',
        ('windows', '386'): 'win32',
        ('windows', 'arm64'): 'win_arm64',
    }

    # Check for direct mapping first
    if (goos, goarch) in platform_map:
        return platform_map[(goos, goarch)]

    # Generic fallback conversion
    os_map = {
        'linux': 'manylinux_2_17',
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

    raise ValueError(f'No platform tag for {goos}/{goarch}')
