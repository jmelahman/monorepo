import os
import re
import shutil
import urllib.request
import tarfile
import tempfile

from hatchling.builders.hooks.plugin.interface import BuildHookInterface
import manygo


class GoBinaryBuildHook(BuildHookInterface):
    def initialize(self, version, build_data):
        build_data["pure_python"] = False
        goos = os.getenv("GOOS")
        goarch = os.getenv("GOARCH")
        if goos and goarch:
            build_data["tag"] = "py3-none-" + manygo.get_platform_tag(goos=goos, goarch=goarch)
        tag = os.environ["GITHUB_REF_NAME"]
        match = re.search(r'v(\d+\.\d+\.\d+)\.\d+', tag)
        assert match is not None
        version = match.group(1)
        archive = "go{}.{}-{}.tar.gz".format(version, goos, goarch)

        if not os.path.exists(archive):
            urllib.request.urlretrieve("https://storage.googleapis.com/golang/" + archive, archive)

        if not os.path.exists("go"):
            with tempfile.TemporaryDirectory() as temp_dir:
                with tarfile.open(archive, "r:gz") as tar:
                    tar.extractall(path=temp_dir)
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
