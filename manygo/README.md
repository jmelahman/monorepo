# manygo

[![Deploy Status](https://github.com/jmelahman/manygo/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/manygo/actions)
[![PyPI](https://img.shields.io/pypi/v/manygo.svg)](https://pypi.org/project/manygo/)

A Python library for generating platform-specific tags for Golang packages and binaries.

## Features

- Convert Golang platform identifiers (GOOS and GOARCH) to Python platform tags

## Installation

```bash
pip install manygo
```

## Usage

```python
>>> import manygo
>>> manygo.get_platform_tag('linux', 'amd64')
'manylinux_2_17_x86_64'

>>> manygo.get_platform_tag('darwin', 'arm64')
'macosx_11_0_arm64'
```
