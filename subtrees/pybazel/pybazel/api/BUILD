load("@rules_python//python:defs.bzl", "py_library")

package(default_visibility = ["//pybazel:__subpackages__"])

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
        ":protocol",
        "//pybazel:errors",
        "//pybazel/models:info",
        "//pybazel/models:label",
    ],
)

py_library(
    name = "protocol",
    srcs = ["protocol.py"],
)
