from __future__ import annotations

import pathlib
import subprocess
from typing import Iterable

import git


def _get_inferred_toplevel() -> pathlib.Path:
    toplevel = (
        subprocess.check_output(["git", "rev-parse", "--show-toplevel"])
        .decode()
        .rstrip()
    )
    return pathlib.Path(toplevel)


class GitClient:
    def __init__(self, base_commit: str, toplevel: pathlib.Path | None = None) -> None:
        toplevel = toplevel or _get_inferred_toplevel()
        self._repo = git.Repo(toplevel)
        self._base_commit = self._repo.commit(base_commit)

    @property
    def base_commit(self) -> git.Commit:  # type: ignore[name-defined]
        return self._base_commit

    def diff_base(self) -> Iterable[str]:
        changed_files = set()
        # Do we want this (i.e. to include tracked files), or should this only
        # be between the HEAD commit and the base commit? The former seems safer.
        diffs = self.base_commit.diff()
        for diff in diffs:
            # Do we need/want both A and B paths?
            changed_files.add(diff.a_path)
            changed_files.add(diff.b_path)
        return changed_files
