load("@pydeps//:requirements.bzl", "requirement")

py_binary(
    name = "snapify",
    srcs = ["snapify.py"],
    visibility = ["//snapify/tests:__subpackages__"],
    deps = [
        requirement("requests"),
        requirement("urllib3"),
    ],
)
