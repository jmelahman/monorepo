load("@pydeps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_binary")

py_binary(
    name = "format",
    srcs = ["format.py"],
    deps = [
        requirement("black"),
    ],
)
