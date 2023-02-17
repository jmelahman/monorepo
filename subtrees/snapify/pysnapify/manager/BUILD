load("@pip_deps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_library")

package(default_visibility = [
    "//pysnapify:__subpackages__",
    "//subtrees/snapify/tests:__subpackages__",
])

py_library(
    name = "base",
    srcs = ["base.py"],
    deps = [
        ":utils",
    ],
)

py_library(
    name = "pacman",
    srcs = ["pacman.py"],
    deps = [
        ":base",
    ],
)

py_library(
    name = "snap",
    srcs = ["snap.py"],
    deps = [
        ":base",
        requirement("requests"),
        requirement("urllib3"),
    ],
)

py_library(
    name = "utils",
    srcs = ["utils.py"],
)
