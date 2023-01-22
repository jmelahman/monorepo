load("@rules_python//python:defs.bzl", "py_library")

package(default_visibility = [":__subpackages__"])

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
        "//pysnapify/manager:base",
        "//pysnapify/manager:pacman",
        "//pysnapify/manager:snap",
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
