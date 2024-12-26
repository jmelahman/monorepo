import os
import subprocess

from hatchling.builders.hooks.plugin.interface import BuildHookInterface


class GoBinaryBuildHook(BuildHookInterface):
    def initialize(self, version, build_data):
        build_data["pure_python"] = False
        binary_name = self.config["binary_name"]
        tag = os.getenv("GITHUB_REF_NAME", "dev")
        commit = os.getenv("GITHUB_SHA", "none")

        if not os.path.exists(binary_name):
            print(f"Building Go binary '{binary_name}'...")
            subprocess.check_call(
                ["go", "build",  f"-ldflags=-X main.version={tag} -X main.commit={commit} -s -w", "-o", binary_name],
                env={"GOOS": "linux", "GOARCH": "amd64", **os.environ},
            )

        build_data["shared_scripts"] = {binary_name: binary_name}

