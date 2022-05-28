load("@pydeps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_binary")

py_binary(
    name = "snapify",
    srcs = ["snapify.py"],
    visibility = ["//snapify/tests:__subpackages__"],
    deps = [
        requirement("requests"),
        requirement("urllib3"),
    ],
)
