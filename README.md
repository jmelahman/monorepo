# Golang Binary Packaging

## Overview

This package provides a Hatch build hook for seamlessly integrating Golang binaries into Python projects. It automatically downloads and includes the appropriate Go runtime for different platforms during the build process.

## Features

- Automatic Go runtime download for multiple platforms
- Cross-platform support (Windows, Linux, macOS)
- Configurable through environment variables
- Integrates with Hatch build system

## Installation

```bash
pip install your-package-name
```

## Usage

The package uses environment variables to configure the Go binary build:

- `GOOS`: Target operating system (e.g., `linux`, `windows`, `darwin`)
- `GOARCH`: Target architecture (e.g., `amd64`, `arm64`)

### Example pyproject.toml Configuration

```toml
[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.hatch.build.hooks.golang]
platforms = ["linux-x86_64", "windows-x86_64", "macos-x86_64"]
```

## Development

To build the package locally:

```bash
hatch build
```

## License

See the LICENSE file for details.

## Contributing

Contributions are welcome! Please submit pull requests or open issues on the project's repository.
