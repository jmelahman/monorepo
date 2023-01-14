from __future__ import annotations

import io

import yaml

import pybazel


def run(blueprint: io.BufferedReader) -> None:
    loaded_blueprint = yaml.safe_load(blueprint)

    bazel = pybazel.BazelClient()
    print(dir(bazel))
    print(bazel.api.info())
    for task in loaded_blueprint["tasks"]:
        if not task.get("bazel_test_matrix"):
            continue
