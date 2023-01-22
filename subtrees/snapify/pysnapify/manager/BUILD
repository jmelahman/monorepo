load("@pydeps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_library")

package(default_visibility = ["//snapify/tests:__subpackages__"])

py_library(
    name = "base",
    srcs = ["base.py"],
    visibility = [
        "//snapify/pysnapify:__subpackages__",
        "//snapify/tests:__subpackages__",
    ],
    deps = [
        ":utils",
    ],
)

py_library(
    name = "pacman",
    srcs = ["pacman.py"],
    visibility = [
        "//snapify/pysnapify:__subpackages__",
        "//snapify/tests:__subpackages__",
    ],
    deps = [
        ":base",
    ],
)

py_library(
    name = "snap",
    srcs = ["snap.py"],
    visibility = [
        "//snapify/pysnapify:__subpackages__",
        "//snapify/tests:__subpackages__",
    ],
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
