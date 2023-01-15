from __future__ import annotations

import io
import itertools
from typing import NamedTuple

import yaml

import pybazel

# TODO: Convert to enum.
_BUILDKITE = "buildkite"
_SUPPORTED_PLATFORMS = frozenset(
    [
        _BUILDKITE,
    ]
)
_BUILD = "build"
_RUN = "run"
_TEST = "test"
_MANUAL_TAG = "manual"


class BazelTask(NamedTuple):
    command: str
    targets: list[pybazel.models.label.Label]
    options: str
    config: str


class PipelineBuilder:
    def __init__(self, platform: str) -> None:
        print(dir(pybazel))
        print(pybazel.__path__)
        import sys

        print(sys.path)
        self._bazel = pybazel.BazelClient()
        self._platform = platform

    @property
    def platform(self) -> str:
        return self._platform

    def _parse_tag_filters(self, tags: list[str]) -> tuple[list[str], list[str]]:
        positive_filters = []
        negative_filters = []
        positive_manual_filter = False
        for tag in tags:
            if tag.startswith("-"):
                negative_filters.append(tag[1:])
            else:
                if tag == _MANUAL_TAG:
                    positive_manual_filter = True
                positive_filters.append(tag)
        if not positive_manual_filter:
            negative_filters.append(_MANUAL_TAG)
        return positive_filters, negative_filters

    def generate_bazel_matrix(
        self,
        bazel_matrix: dict | None,
        subcommand: str,
    ) -> list[BazelTask]:
        tasks = []
        options_str = " ".join(bazel_matrix.get("options", []))
        positive_filters, negative_filters = self._parse_tag_filters(
            bazel_matrix.get("tag_filters", [])
        )
        for universe, config in itertools.product(
            bazel_matrix["universe"], bazel_matrix.get("config_matrix", [""])
        ):
            query_str = (
                bazel_matrix["filter_query"].format(UNIVERSE=universe)
                if bazel_matrix.get("filter_query")
                else universe
            )
            if positive_filters:
                query_str += f" intersect attr(tags, \\b({'|'.join(positive_filters)})\\b, {universe})"
            if negative_filters:
                query_str += f" except attr(tags, \\b({'|'.join(negative_filters)})\\b, {universe})"
            targets = self._bazel.query(query_str)
            tasks.append(
                BazelTask(
                    command=f"bazel {subcommand}",
                    targets=" ".join([str(label) for label in targets]),
                    options=options_str,
                    config=f"--config={config}" if config else "",
                )
            )
        return tasks

    def parse_task(self, task: dict) -> list[BazelTask]:
        bazel_test_matrix = task.get("bazel_test_matrix")
        if bazel_test_matrix:
            return self.generate_bazel_matrix(bazel_test_matrix, _TEST)
        bazel_build_matrix = task.get("bazel_build_matrix")
        if bazel_build_matrix:
            return self.generate_bazel_matrix(bazel_build_matrix, _BUILD)
        bazel_run_matrix = task.get("bazel_run_matrix")
        if bazel_run_matrix:
            return self.generate_bazel_matrix(bazel_run_matrix, _RUN)
        raise ValueError(f"Unknown task: {task}")

    def translate_task_to_buildkite_step(self, parsed_task: BazelTask) -> dict:
        step = {
            "command": f"{parsed_task.command} {parsed_task.options} {parsed_task.config} {parsed_task.targets}",
        }
        return step

    def translate_task(self, parsed_task: BazelTask) -> dict:
        if self._platform == _BUILDKITE:
            return self.translate_task_to_buildkite_step(parsed_task)


def run(blueprint: io.BufferedReader, platform: str) -> None:
    loaded_blueprint = yaml.safe_load(blueprint)
    builder = PipelineBuilder(platform)

    steps = []
    for task in loaded_blueprint["tasks"]:
        parsed_tasks = builder.parse_task(task)
        for parsed_task in parsed_tasks:
            steps.append(builder.translate_task(parsed_task))
    print(steps)
