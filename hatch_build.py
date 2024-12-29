import os
import shutil
import urllib.request
import tarfile
import tempfile

from hatchling.builders.hooks.plugin.interface import BuildHookInterface


class GoBinaryBuildHook(BuildHookInterface):
    def initialize(self, version, build_data):
        build_data["pure_python"] = False
        version = os.getenv("GITHUB_REF_NAME", "1.23.3")
        archive = "go{}.linux-amd64.tar.gz".format(version)

        url = "https://storage.googleapis.com/golang/" + archive
        urllib.request.urlretrieve(url, archive)

        if os.path.exists("go"):
            os.remove("go")

        with tempfile.TemporaryDirectory() as temp_dir:
            with tarfile.open(archive, "r:gz") as tar:
                tar.extractall(path=temp_dir)
            shutil.move(os.path.join(temp_dir, "go", "bin","go"), self.root)

        build_data["shared_scripts"] = {"go": "go"}
