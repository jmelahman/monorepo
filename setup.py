#!/usr/bin/env python3.10

import pathlib
import os
from setuptools import setup

README = os.path.join(os.path.dirname(__file__), "README.md")

setup(
    name="python-snapify",
    version="0.1.0",
    description="Reversible package converter -- convert to and from snap packages",
    author="Jamison Lahman",
    author_email="jamison@lahman.dev",
    long_description=README,
    long_description_content_type="text/markdown",
    url="https://github.com/jmelahman/python-snapify",
    py_modules=["snapify"],
    keywords=["arch linux", "pacman", "snap", "snapd", "snapify"],
    download_url="https://github.com/jmelahman/python-snapify/archive/refs/tags/v0.1.0.tar.gz",
    license="MIT",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "License :: OSI Approved :: MIT License",
        "Operating System :: Unix",
        "Topic :: System :: Software Distribution",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.10",
    ],
    install_requires=["requests", "urllib3"],
)
