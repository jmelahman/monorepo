load("@pydeps//:requirements.bzl", "requirement")

py_binary(
    name = "snapify",
    srcs = ["snapify.py"],
    deps = [
        requirement("requests"),
        requirement("urllib3"),
    ],
    visibility = ["//snapify/tests:__subpackages__"],
)
