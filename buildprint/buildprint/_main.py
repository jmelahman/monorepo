from __future__ import annotations

import io

import yaml


def run(blueprint: io.BufferedReader) -> None:
    pipeline = yaml.safe_load(blueprint)
    print(pipeline)
