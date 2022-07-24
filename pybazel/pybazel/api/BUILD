load("@rules_python//python:defs.bzl", "py_binary", "py_library")

package(
    default_visibility = ["//pybazel:visibility"],
)

py_library(
    name = "build",
    srcs = ["build.py"],
    deps = [
        ":protocol",
    ],
)

py_library(
    name = "client",
    srcs = ["client.py"],
    deps = [
        ":build",
        ":info",
        "//pybazel/pybazel:errors",
        "//pybazel/pybazel/models:info",
    ],
)

py_library(
    name = "info",
    srcs = ["info.py"],
    deps = [
        ":protocol",
        "//pybazel/pybazel/models:info",
    ],
)

py_library(
    name = "protocol",
    srcs = ["protocol.py"],
    deps = [
        "//pybazel/pybazel/models:info",
    ],
)

