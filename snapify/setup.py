#!/usr/bin/env python3.10

import pathlib
from setuptools import setup

README = (pathlib.Path(__file__).parent / "README.md").read_text()
VERSION = "0.2.2"

setup(
    name="python-snapify",
    version=VERSION,
    description="Reversible package converter -- convert to and from snap packages",
    author="Jamison Lahman",
    author_email="jamison@lahman.dev",
    long_description=README,
    long_description_content_type="text/markdown",
    url="https://github.com/jmelahman/python-snapify",
    py_modules=["snapify"],
    keywords=["arch linux", "pacman", "snap", "snapd", "snapify"],
    download_url=f"https://github.com/jmelahman/python-snapify/archive/refs/tags/v{VERSION}.tar.gz",
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
