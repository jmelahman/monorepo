load("@rules_python//python:defs.bzl", "py_binary", "py_library")

package(default_visibility = [":__subpackages__"])

py_binary(
    name = "client",
    srcs = ["client.py"],
    deps = [
        "//pybazel/api:client",
        "//pybazel/utils:logging",
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
