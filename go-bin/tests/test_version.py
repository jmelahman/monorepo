from __future__ import annotations

import importlib.util
import os

import pytest

# Loaded by path so the test runs the same from go-bin/ and the monorepo root,
# where the `go` package is not installed.
VERSION_FILE = os.path.join(os.path.dirname(__file__), os.pardir, "src", "go", "_version.py")


def load_version() -> str:
    spec = importlib.util.spec_from_file_location("_version", VERSION_FILE)
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.__version__


@pytest.mark.parametrize(
    ("tag", "expected"),
    [
        ("v1.25.1", "1.25.1"),
        ("v1.25.1.2", "1.25.1"),
        ("1.25.1", "1.25.1"),
        ("master", "0.0.0"),
    ],
)
def test_version_from_tag(monkeypatch: pytest.MonkeyPatch, tag: str, expected: str) -> None:
    monkeypatch.setenv("GITHUB_REF_NAME", tag)
    assert load_version() == expected


def test_version_without_tag(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("GITHUB_REF_NAME", raising=False)
    assert load_version() == "0.0.0"
