import os
import subprocess

from hatchling.builders.hooks.plugin.interface import BuildHookInterface


class GoBinaryBuildHook(BuildHookInterface):
    def initialize(self, version, build_data):
        binary_name = self.config["binary_name"]

        if not os.path.exists(binary_name):
            print(f"Building Go binary '{binary_name}'...")
            subprocess.check_call(
                ["go", "build",  f"-ldflags=-X main.version={version} -s -w", "-o", binary_name],
                env={"GOOS": "linux", "GOARCH": "amd64", **os.environ},
            )

        build_data["artifacts"].append(binary_name)
