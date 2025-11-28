from __future__ import annotations

import os
import re
import shutil
import tarfile
import tempfile
import urllib.request
import zipfile

from hatchling.builders.hooks.plugin.interface import BuildHookInterface

import manygo


class GoBinaryBuildHook(BuildHookInterface):
    def initialize(self, version, build_data) -> None:  # noqa: ANN001
        build_data["pure_python"] = False
        goos = os.getenv("GOOS")
        goarch = os.getenv("GOARCH")
        if goos and goarch:
            build_data["tag"] = "py3-none-" + manygo.get_platform_tag(goos=goos, goarch=goarch)  # type: ignore[invalid-argument-type]
        tag = os.environ["GITHUB_REF_NAME"]
        match = re.search(r"v(\d+\.\d+\.\d+)(?:\.\d+)?", tag)
        assert match is not None
        version = match.group(1)
        if goos == "windows":
            archive = f"go{version}.{goos}-{goarch}.zip"
        else:
            archive = f"go{version}.{goos}-{goarch}.tar.gz"

        if not os.path.exists(archive):
            urllib.request.urlretrieve("https://storage.googleapis.com/golang/" + archive, archive)

        if not os.path.exists("go"):
            with tempfile.TemporaryDirectory() as temp_dir:
                if goos == "windows":
                    with zipfile.ZipFile(archive) as zip:  # noqa: A001
                        zip.extractall(path=temp_dir)  # noqa: S202
                else:
                    with tarfile.open(archive, "r:gz") as tar:
                        tar.extractall(path=temp_dir)  # noqa: S202
                shutil.move(os.path.join(temp_dir, "go"), self.root)

        build_data["force_include"] = {
            "go/go.env": "go/go.env",
            "go/VERSION": "go/VERSION",
            "go/bin": "go/bin",
            "go/lib": "go/lib",
            "go/misc": "go/misc",
            "go/pkg": "go/pkg",
            "go/src": "go/src",
            "src/go/__init__.py": "go/__init__.py",
        }
