from __future__ import annotations

from concurrent import futures
import io
import itertools
import subprocess
import tempfile
from typing import Iterable, NamedTuple, TYPE_CHECKING

import yaml

from buildprint import _logging
import pybazel

if TYPE_CHECKING:
    from pybazel.models.label import Label

logger = _logging.getLogger(__name__)

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
    targets: list[Label]
    options: str
    config: str


class PipelineBuilder:
    def __init__(self, dry_run: bool, platform: str) -> None:
        self._bazel = pybazel.BazelClient()
        self._platform = platform
        self._dry_run = dry_run

    @property
    def platform(self) -> str:
        return self._platform

    @property
    def dry_run(self) -> bool:
        return self._dry_run

    def _parse_tag_filters(
        self,
        tags: list[str],
        positive_filters: list[str] | None = None,
        negative_filters: list[str] | None = None,
    ) -> tuple[list[str], list[str]]:
        positive_filters = positive_filters or []
        negative_filters = negative_filters or []
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
        bazel_matrix: dict,
        subcommand: str,
    ) -> list[BazelTask]:
        tasks = []
        adjustments = bazel_matrix.get("adjustments", [])
        options_str = " ".join(bazel_matrix.get("options", []))
        bazel_matrix_key = "commands" if subcommand == _RUN else "universes"
        positive_filters, negative_filters = self._parse_tag_filters(
            bazel_matrix.get("tag_filters", [])
        )
        for universe, config in itertools.product(
            bazel_matrix[bazel_matrix_key], bazel_matrix.get("configs", [""])
        ):
            positive_adjusted_filters = positive_filters.copy()
            negative_adjusted_filters = negative_filters.copy()
            for adjustment in adjustments:
                if not adjustment["config"] == config:
                    continue
                (
                    positive_adjusted_filters,
                    negative_adjusted_filters,
                ) = self._parse_tag_filters(
                    adjustment["tag_filters"],
                    positive_adjusted_filters,
                    negative_adjusted_filters,
                )
            query_str = (
                bazel_matrix["filter_query"].format(UNIVERSE=universe)
                if bazel_matrix.get("filter_query")
                else universe
            )

            if positive_adjusted_filters:
                query_str += f" intersect attr(tags, '\\b({'|'.join(positive_adjusted_filters)})\\b', {universe})"
            if negative_adjusted_filters:
                query_str += f" except attr(tags, '\\b({'|'.join(negative_adjusted_filters)})\\b', {universe})"
            targets = self._bazel.query(query_str)
            tasks.append(
                BazelTask(
                    command=f"bazel {subcommand}",
                    targets=targets,
                    options=options_str,
                    config=f"--config={config}" if config else "",
                )
            )
        return tasks

    def generate_matrix(self, task: dict) -> list[BazelTask]:
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

    def upload_targets_artifact(self, targets: Iterable[Label]) -> str:
        with tempfile.NamedTemporaryFile(suffix=".txt", delete=False) as tmp:
            tmp.write("\n".join([target.name for target in targets]).encode())
            tmp.seek(0)
            if self.dry_run:
                logger.info(f"Would have uploaded file: {tmp.name}")
                logger.debug("Target file contained:\n{!r}".format(tmp.read()))
            else:
                subprocess.check_call(
                    ["buildkite-agent", "artifact", "upload", tmp.name]
                )
            return tmp.name.lstrip("/")

    def translate_to_buildkite_step(self, parsed_task: BazelTask) -> dict:
        if parsed_task.command == _RUN:
            return {
                "commands": [
                    f"{parsed_task.command} {parsed_task.options} {parsed_task.config} {parsed_task.targets}",
                ],
            }
        else:
            artifact_name = self.upload_targets_artifact(parsed_task.targets)
            return {
                "commands": [
                    f"buildkite-agent artifact download {artifact_name}",
                    f"{parsed_task.command} {parsed_task.options} {parsed_task.config} --target_pattern_file {artifact_name}",
                ],
            }

    def upload_buildkite_step(self, step: BazelTask) -> None:
        if not step.targets:
            return
        steps = {
            "steps": [
                self.translate_to_buildkite_step(step),
            ]
        }
        if self.dry_run:
            logger.info("Would have upload:")
            print(yaml.dump(steps))
        else:
            subprocess.run(
                ["buildkite-agent", "pipeline", "upload"],
                check=True,
                stdout=subprocess.PIPE,
                input=yaml.dump(steps),
                encoding="ascii",
            )

    def upload_steps(self, generic_steps: list[BazelTask]) -> None:
        for step in generic_steps:
            if self.platform == _BUILDKITE:
                self.upload_buildkite_step(step)


def run(blueprint: io.BufferedReader, dry_run: bool, platform: str) -> None:
    loaded_blueprint = yaml.safe_load(blueprint)
    builder = PipelineBuilder(dry_run, platform)

    with futures.ThreadPoolExecutor(max_workers=10) as executor:
        results = [
            executor.submit(builder.generate_matrix, task)
            for task in loaded_blueprint["tasks"]
        ]
    # Upload the results in order.
    for future in results:
        builder.upload_steps(future.result())
