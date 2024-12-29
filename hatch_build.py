import os
import shutil
import urllib.request
import tarfile
import tempfile

from hatchling.builders.hooks.plugin.interface import BuildHookInterface


class GoBinaryBuildHook(BuildHookInterface):
    def initialize(self, version, build_data):
        build_data["pure_python"] = False
        version = os.environ["GITHUB_REF_NAME"]
        archive = "go{}.linux-amd64.tar.gz".format(version.lstrip("v"))

        if not os.path.exists(archive):
            url = "https://storage.googleapis.com/golang/" + archive
            urllib.request.urlretrieve(url, archive)

        try:
            os.remove("go")
        except:
            pass

        with tempfile.TemporaryDirectory() as temp_dir:
            with tarfile.open(archive, "r:gz") as tar:
                tar.extractall(path=temp_dir)
            shutil.move(os.path.join(temp_dir, "go", "bin", "go"), self.root)

        build_data["force_include"] = {
            "go": "go/bin/go",
            "src/go/__init__.py": "go/__init__.py",
        }
