load("@pydeps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_library")

package(
    default_visibility = ["//pybazel:visibility"],
)

py_library(
    name = "logging",
    srcs = ["logging.py"],
    deps = [
        requirement("colorama"),
    ],
)
