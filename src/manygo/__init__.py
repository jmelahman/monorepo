from __future__ import annotations

from enum import Enum
from typing import Literal

# https://go.dev/doc/install/source#environment
# See also, `$ go tools dist list`
class GOOS(Enum):
    """Enumeration of valid Go operating systems."""
    AIX = "aix"
    ANDROID = "android"
    DARWIN = "darwin"
    FREEBSD = "freebsd"
    IOS = "ios"
    JS = "js"
    LINUX = "linux"
    NETBSD = "netbsd"
    OPENBSD = "openbsd"
    PLAN9 = "plan9"
    SOLARIS = "solaris"
    WINDOWS = "windows"

class GOARCH(Enum):
    """Enumeration of valid Go architectures."""
    AMD64 = "amd64"
    ARM = "arm"
    ARM64 = "arm64"
    MIPS = "mips"
    MIPS64 = "mips64"
    PPC64 = "ppc64"
    RISCV64 = "riscv64"
    S390X = "s390x"
    WASM = "wasm"
    X86 = "386"

DistList = frozenset([
    "aix/ppc64",
    "android/386",
    "android/amd64",
    "android/arm",
    "android/arm64",
    "darwin/amd64",
    "darwin/arm64",
    "dragonfly/amd64",
    "freebsd/386",
    "freebsd/amd64",
    "freebsd/arm",
    "freebsd/arm64",
    "freebsd/riscv64",
    "illumos/amd64",
    "ios/amd64",
    "ios/arm64",
    "js/wasm",
    "linux/386",
    "linux/amd64",
    "linux/arm",
    "linux/arm64",
    "linux/loong64",
    "linux/mips",
    "linux/mips64",
    "linux/mips64le",
    "linux/mipsle",
    "linux/ppc64",
    "linux/ppc64le",
    "linux/riscv64",
    "linux/s390x",
    "netbsd/386",
    "netbsd/amd64",
    "netbsd/arm",
    "netbsd/arm64",
    "openbsd/386",
    "openbsd/amd64",
    "openbsd/arm",
    "openbsd/arm64",
    "openbsd/ppc64",
    "openbsd/riscv64",
    "plan9/386",
    "plan9/amd64",
    "plan9/arm",
    "solaris/amd64",
    "wasip1/wasm",
    "windows/386",
    "windows/amd64",
    "windows/arm",
    "windows/arm64",
])

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
