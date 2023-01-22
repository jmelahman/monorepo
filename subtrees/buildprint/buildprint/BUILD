load("@pip_deps//:requirements.bzl", "requirement")
load("@pip_deps_internal//:requirements.bzl", requirement_internal = "requirement")
load("@rules_python//python:defs.bzl", "py_binary", "py_library")

py_binary(
    name = "main",
    srcs = [
        "__main__.py",
    ],
    imports = [".."],
    main = "__main__.py",
    deps = [
        ":run",
        ":version",
        requirement("click"),
    ],
)

py_library(
    name = "run",
    srcs = ["_run.py"],
    deps = [
        ":logging",
        requirement_internal("pybazel"),
        requirement("pyyaml"),
        requirement("types-PyYAML"),
    ],
)

py_library(
    name = "version",
    srcs = ["_version.py"],
)


py_library(
    name = "logging",
    srcs = ["_logging.py"],
    deps = [
        requirement("colorama"),
    ],
)
