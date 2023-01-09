#!/usr/bin/env python3.10

import pathlib

from setuptools import setup

from pybazel.version import version

README = (pathlib.Path(__file__).parent / "README.md").read_text()

setup(
    name="pybazel",
    version=version,
    description="A python client for Bazel",
    author="Jamison Lahman",
    author_email="jamison@lahman.dev",
    long_description=README,
    long_description_content_type="text/markdown",
    url="https://github.com/jmelahman/pybazel",
    py_modules=["pybazel"],
    keywords=["bazel", "bazelbuild", "buildtools", "tools"],
    download_url=f"https://github.com/jmelahman/pybazel/archive/refs/tags/v{version}.tar.gz",
    license="MIT",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "License :: OSI Approved :: MIT License",
        "Operating System :: Unix",
        "Topic :: System :: Software Distribution",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.10",
    ],
    install_requires=["colorama==0.4.5"],
)
