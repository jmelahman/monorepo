#!/usr/bin/env python3
import os
import shutil
import subprocess

from setuptools import setup
from setuptools.command.build import build
from setuptools.command.install import install

class BuildGoBinary(build):
    def run(self):
        if not os.path.exists("tag"):
            print("Building Go binary...")
            tag = os.getenv("GITHUB_REF_NAME", "dev")
            subprocess.check_call(
                ["go", "build",  f"-ldflags=-X main.version={tag} -s -w", "-o", "tag", "main.go"],
                env={"GOOS": "linux", "GOARCH": "amd64", **os.environ},
            )
        build.run(self)

class PostInstallCommand(install):
    def run(self):
        binary_source = os.path.join(os.path.dirname(__file__), "tag")
        binary_dest = os.path.join(self.install_scripts, "tag")

        os.makedirs(self.install_scripts, exist_ok=True)
        shutil.move(binary_source, binary_dest)

        install.run(self)

setup(
    name="release-tag",
    packages=[],
    include_package_data=True,
    cmdclass={
        "build": BuildGoBinary,
        "install": PostInstallCommand,
    },
    description="Automatically create [semantic version](https://semver.org/) git tags",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    classifiers=[
        "Programming Language :: Go",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
    ],
    use_scm_version=True,
    setup_requires=["setuptools>=42", "setuptools_scm"],
    python_requires=">=3.6",
)
