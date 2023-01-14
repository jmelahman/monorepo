load("@pip_deps//:requirements.bzl", "requirement")
load("@pydeps//:requirements.bzl", requirement_pydep = "requirement")
load("@rules_python//python:defs.bzl", "py_binary", "py_library")

package(
    default_visibility = ["//buildprint:__subpackages__"],
)

py_binary(
    name = "main",
    srcs = [
        "__main__.py",
    ],
    imports = [".."],
    main = "__main__.py",
    deps = [
        "//buildprint/buildprint:run",
        "//buildprint/buildprint:version",
        requirement("click"),
    ],
)

py_library(
    name = "run",
    srcs = ["_run.py"],
    deps = [
        requirement_pydep("pybazel"),
        requirement("pyyaml"),
        requirement("types-PyYAML"),
    ],
)

py_library(
    name = "version",
    srcs = ["_version.py"],
)
