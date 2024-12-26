#!/usr/bin/env python3
import os
import shutil
import subprocess

from setuptools import setup
from setuptools.command.build import build
from setuptools.command.install import install

class BuildGoBinary(build):
    def run(self):
        if not os.path.exists("nature-sounds"):
            print("Building Go binary...")
            tag = os.getenv("GITHUB_REF_NAME", "dev")
            subprocess.check_call(
                ["go", "build",  f"-ldflags=-X main.version={tag} -s -w", "-o", "nature-sounds", "main.go"],
                env={"GOOS": "linux", "GOARCH": "amd64", **os.environ},
            )
        build.run(self)

class PostInstallCommand(install):
    def run(self):
        binary_source = os.path.join(os.path.dirname(__file__), "nature-sounds")
        binary_dest = os.path.join(self.install_scripts, "nature-sounds")

        os.makedirs(self.install_scripts, exist_ok=True)
        shutil.move(binary_source, binary_dest)

        install.run(self)

setup(
    name="nature-sounds",
    packages=[],
    include_package_data=True,
    cmdclass={
        "build": BuildGoBinary,
        "install": PostInstallCommand,
    },
    description="A nature sounds player for the command-line",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    classifiers=[
        "Programming Language :: Go",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
    ],
    python_requires=">=3.6",
)
