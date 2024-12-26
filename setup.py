#!/usr/bin/env python3
import os
import shutil
import subprocess

from setuptools import setup
from setuptools.command.build import build
from setuptools.command.install import install

PACKAGE_NAME = "work"

class BuildGoBinary(build):
    def run(self):
        if not os.path.exists(PACKAGE_NAME):
            print("Building Go binary...")
            tag = os.getenv("GITHUB_REF_NAME", "dev")
            commit = os.getenv("GITHUB_SHA", "none")
            subprocess.check_call(
                ["go", "build",  f"-ldflags=-X main.version={tag} -X main.commit={commit} -s -w", "-o", PACKAGE_NAME, "main.go"],
                env={"GOOS": "linux", "GOARCH": "amd64", **os.environ},
            )
        build.run(self)

class PostInstallCommand(install):
    def run(self):
        binary_source = os.path.join(os.path.dirname(__file__), PACKAGE_NAME)
        binary_dest = os.path.join(self.install_scripts, PACKAGE_NAME)

        os.makedirs(self.install_scripts, exist_ok=True)
        shutil.move(binary_source, binary_dest)

        install.run(self)

setup(
    cmdclass={
        "build": BuildGoBinary,
        "install": PostInstallCommand,
    },
)
