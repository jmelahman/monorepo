load("@rules_python//python:defs.bzl", "py_library")

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
        "//pybazel/pybazel:errors",
        "//pybazel/pybazel/models:info",
        "//pybazel/pybazel/models:label",
    ],
)

py_library(
    name = "protocol",
    srcs = ["protocol.py"],
)
