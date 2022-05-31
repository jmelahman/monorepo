load("@pydeps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_library")

package(default_visibility = ["//snapify/tests:__subpackages__"])

py_library(
    name = "main",
    srcs = ["main.py"],
    deps = [
        ":constants",
        requirement("requests"),
        requirement("urllib3"),
    ],
)

py_library(
    name = "constants",
    srcs = ["constants.py"],
)
