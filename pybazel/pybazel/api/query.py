from __future__ import annotations

import subprocess

from pybazel.models import label

from .protocol import ApiProtocol


class QueryApiMixin(ApiProtocol):
    def query(
        self: ApiProtocol,
        query_string: str,
        query_options: list[str] | None = None,
    ) -> list[label.Label]:
        labels: list[label.Label] = []
        query_command = [self.which_bazel, *self.bazel_options, "query"]
        query_command += query_options or []
        query_command += [query_string]
        output = (
            subprocess.check_output(
                query_command, cwd=self.workspace, stderr=subprocess.DEVNULL
            )
            .decode()
            .rstrip()
        )
        for line in output.rsplit("\n"):
            labels.append(label.Label(line))
        return labels
