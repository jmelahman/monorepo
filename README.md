# manygo

A Python library for generating platform-specific tags for Golang packages and binaries.

## Overview

`manygo` provides utilities to help with packaging and distributing Golang applications, with a focus on generating accurate Python platform tags for cross-platform compatibility.

## Features

- Convert Golang platform identifiers (GOOS and GOARCH) to Python platform tags
- Support for multiple architectures and operating systems
- Helpful for creating wheel distributions and managing platform-specific builds

## Supported Platforms

### Operating Systems
- Darwin (macOS)
- Linux
- Windows

### Architectures
- x86_64 (amd64)
- ARM64 (aarch64)
- 32-bit x86 (386)
- ARM (armv7l)
- s390x
- PowerPC 64-bit Little Endian (ppc64le)
- PowerPC 64-bit (ppc64)

## Installation

```bash
pip install manygo
```

## Usage

```python
from manygo import get_platform_tag

# Get platform tag for Linux on x86_64
tag = get_platform_tag('linux', 'amd64')
print(tag)  # Outputs: 'manylinux_2_17_x86_64'

# Get platform tag for macOS on ARM64
tag = get_platform_tag('darwin', 'arm64')
print(tag)  # Outputs: 'macosx_11_0_arm64'
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License.
