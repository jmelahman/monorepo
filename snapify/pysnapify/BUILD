load("@rules_python//python:defs.bzl", "py_library")

package(default_visibility = ["//snapify/tests:__subpackages__"])

py_library(
    name = "main",
    srcs = ["main.py"],
    deps = [
        ":constants",
        ":snapifier",
    ],
)

py_library(
    name = "snapifier",
    srcs = ["snapifier.py"],
    deps = [
        ":constants",
        ":errors",
        "//snapify/pysnapify/manager:base",  # Build-only
        "//snapify/pysnapify/manager:pacman",
        "//snapify/pysnapify/manager:snap",
    ],
)

py_library(
    name = "constants",
    srcs = ["constants.py"],
)

py_library(
    name = "errors",
    srcs = ["errors.py"],
)
