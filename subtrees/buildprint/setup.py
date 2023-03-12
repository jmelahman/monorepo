#!/usr/bin/env python3.10

import glob
import os
import pathlib

from mypyc.build import mypycify  # type: ignore[import]
from setuptools import find_packages  # type: ignore[import]
from setuptools import setup

from buildprint import _version

README = (pathlib.Path(__file__).parent / "README.md").read_text()
NAME = "buildprint"

# Adopted from https://github.com/python/mypy/blob/master/setup.py
def find_package_data(base, globs, root=NAME):
    """Find all interesting data files, for setup(package_data=)
    Arguments:
      root:  The directory to search in.
      globs: A list of glob patterns to accept files.
    """

    rv_dirs = [root for root, _, _ in os.walk(base)]
    rv = []
    for rv_dir in rv_dirs:
        files = []
        for pat in globs:
            files += glob.glob(os.path.join(rv_dir, pat))
        if not files:
            continue
        rv.extend([os.path.relpath(f, root) for f in files])
    return rv


setup(
    name=NAME,
    version=_version.__version__,
    description="Print from a build blueprint",
    author="Jamison Lahman",
    author_email="jamison@lahman.dev",
    long_description=README,
    long_description_content_type="text/markdown",
    url=f"https://github.com/jmelahman/{NAME}",
    py_modules=[],
    ext_modules=mypycify(
        [os.path.join(NAME, x) for x in find_package_data(NAME, ["*.py"])]
    ),
    keywords=["bazel", "bazelbuild", "buildtools", "tools"],
    package_dir={NAME: NAME},
    packages=find_packages(),
    download_url=f"https://github.com/jmelahman/{NAME}/archive/refs/tags/v{_version.__version__}.tar.gz",
    license="MIT",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "License :: OSI Approved :: MIT License",
        "Operating System :: Unix",
        "Topic :: System :: Software Distribution",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.10",
    ],
    install_requires=[
        "click==8.1.3",
        "colorama==0.4.5",
        "gitpython==3.1.31",
        "pyyaml==6.0",
        "pybazel==0.4.0",
    ],
    setup_requires=[
        "setuptools >= 40.6.2",
        "wheel >= 0.30.0",
        "mypy==1.0.0",
        "types-click==7.1.8",
        "types-colorama==0.4.15.4",
        "types-PyYAML==6.0.12.2",
    ],
    entry_points={"console_scripts": [f"{NAME}={NAME}.__main__:main"]},
)
