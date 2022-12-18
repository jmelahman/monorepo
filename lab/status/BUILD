load("@rules_python//python:defs.bzl", "py_binary")
load("@pydeps//:requirements.bzl", "requirement")

py_binary(
    name = "app",
    srcs = ["app.py"],
    data = [
        "robots.txt",
        "templates/index.html",
        "static/assets/favicon.ico",
        "static/style/stylesheet.min.css",
    ],
    deps = [
        requirement("apscheduler"),
        requirement("flask"),
        requirement("matplotlib"),
        requirement("paramiko"),
    ],
)
