#!/usr/bin/env python3.10

import pathlib
from setuptools import setup

README = (pathlib.Path(__file__).parent / "README.md").read_text()
VERSION = "0.0.3"

setup(
    name="buildprint",
    version=VERSION,
    description="Print from a build blueprint",
    author="Jamison Lahman",
    author_email="jamison@lahman.dev",
    long_description=README,
    long_description_content_type="text/markdown",
    url="https://github.com/jmelahman/buildprint",
    py_modules=["buildprint"],
    keywords=["bazel", "bazelbuild", "buildtools", "tools"],
    download_url=f"https://github.com/jmelahman/buildprint/archive/refs/tags/v{VERSION}.tar.gz",
    license="MIT",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "License :: OSI Approved :: MIT License",
        "Operating System :: Unix",
        "Topic :: System :: Software Distribution",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.10",
    ],
)
