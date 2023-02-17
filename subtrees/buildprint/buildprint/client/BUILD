load("@pip_deps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_library")

package(default_visibility = ["//buildprint:__subpackages__"])

py_library(
    name = "git",
    srcs = ["_git.py"],
    deps = [
        requirement("GitPython"),
    ],
)
