load("@pip_deps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_binary", "py_library")

py_binary(
    name = "main",
    srcs = [
        "__main__.py",
    ],
    imports = [".."],
    main = "__main__.py",
    deps = [
        "//buildprint:run",
        "//buildprint:version",
        requirement("click"),
    ],
)

py_library(
    name = "run",
    srcs = ["_run.py"],
    deps = [
        requirement("pybazel"),
        requirement("pyyaml"),
        requirement("types-PyYAML"),
        # "//buildprint/platform:buildkite",
    ],
)

py_library(
    name = "version",
    srcs = ["_version.py"],
)
