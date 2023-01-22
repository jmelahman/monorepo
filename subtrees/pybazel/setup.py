#!/usr/bin/env python3.10

import glob
import os
import pathlib
import sys

from mypyc.build import mypycify
from setuptools import find_packages
from setuptools import setup

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))

from pybazel import __title__
from pybazel import __version__

README = (pathlib.Path(__file__).parent / "README.md").read_text()

# Adopted from https://github.com/python/mypy/blob/master/setup.py
def find_package_data(base, globs, root=__title__):
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
    name=__title__,
    version=__version__,
    description="A python client for Bazel",
    author="Jamison Lahman",
    author_email="jamison@lahman.dev",
    long_description=README,
    long_description_content_type="text/markdown",
    url=f"https://github.com/jmelahman/{__title__}",
    py_modules=[],
    ext_modules=mypycify(
        [os.path.join(__title__, x) for x in find_package_data(__title__, ["*.py"])]
    ),
    keywords=["bazel", "bazelbuild", "buildtools", "tools"],
    package_dir={__title__: __title__},
    packages=find_packages(),
    download_url=f"https://github.com/jmelahman/{__title__}/archive/refs/tags/v{__version__}.tar.gz",
    license="MIT",
    python_requires=">3.10",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "License :: OSI Approved :: MIT License",
        "Operating System :: Unix",
        "Topic :: System :: Software Distribution",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.10",
    ],
    install_requires=[
        "colorama==0.4.5",
    ],
)
