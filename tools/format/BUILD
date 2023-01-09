load("@pip_deps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_binary")

py_binary(
    name = "format",
    srcs = ["format.py"],
    visibility = ["//tools/format/tests:__subpackages__"],
    deps = [
        requirement("black"),
    ],
)
