load("@pydeps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_library")

package(default_visibility = ["//snapify/tests:__subpackages__"])

py_library(
    name = "base",
    srcs = ["base.py"],
    visibility = ["//snapify/pysnapify:__subpackages__"],
    deps = [
        ":utils",
    ],
)

py_library(
    name = "utils",
    srcs = ["utils.py"],
)
