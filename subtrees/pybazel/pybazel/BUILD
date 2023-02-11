load("@rules_python//python:defs.bzl", "py_library")

package(default_visibility = [":__subpackages__"])

py_library(
    name = "pkg",
    srcs = [
        "__init__.py",
    ],
    deps = [
        ":client",
        ":errors",
        ":version",
    ],
)

py_library(
    name = "client",
    srcs = [
        "client.py",
    ],
    deps = [
        "//pybazel/utils:logger",
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
