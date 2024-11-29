#!/usr/bin/env python3
import os
import shutil
import subprocess

from setuptools import setup
from setuptools.command.build import build
from setuptools.command.install import install

class BuildGoBinary(build):
    def run(self):
        if not os.path.exists("work"):
            print("Building Go binary...")
            subprocess.check_call(
                ["go", "build",  "-ldflags=-s -w", "-o", "work", "main.go"],
                env={"GOOS": "linux", "GOARCH": "amd64", **os.environ},
            )
        build.run(self)

class PostInstallCommand(install):
    def run(self):
        binary_source = os.path.join(os.path.dirname(__file__), "work")
        binary_dest = os.path.join(self.install_scripts, "work")

        os.makedirs(self.install_scripts, exist_ok=True)
        shutil.move(binary_source, binary_dest)

        install.run(self)

setup(
    name="go-work",
    version="0.1.0",
    packages=[],
    include_package_data=True,
    cmdclass={
        "build": BuildGoBinary,
        "install": PostInstallCommand,
    },
    description="A stupid simple time tracker",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    classifiers=[
        "Programming Language :: Python",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
    ],
    python_requires=">=3.6",
)
