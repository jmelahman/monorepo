load("@rules_python//python:defs.bzl", "py_binary")
load("@pydeps//:requirements.bzl", "requirement")

py_binary(
    name = "app",
    srcs = ["app.py"],
    data = [
        "templates/index.html",
    ],
    deps = [
        requirement("flask"),
        requirement("paramiko"),
    ],
)
