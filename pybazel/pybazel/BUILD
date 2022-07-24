load("@rules_python//python:defs.bzl", "py_binary", "py_library")

package(
    default_visibility = ["//pybazel:visibility"],
)

py_binary(
    name = "client",
    srcs = ["client.py"],
    deps = [
        "//pybazel/pybazel/api:client",
        "//pybazel/pybazel:logging",
    ],
)

py_library(
    name = "errors",
    srcs = ["errors.py"],
)

py_library(
    name = "version",
    srcs = ["version.py"],
)
