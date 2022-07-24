load("@rules_python//python:defs.bzl", "py_library")
load("@pydeps//:requirements.bzl", "requirement")

package(
    default_visibility = ["//pybazel:visibility"],
)

py_binary(
    name = "logging",
    srcs = ["logging.py"],
    deps = [
        requirement("colorama"),
    ],
)
