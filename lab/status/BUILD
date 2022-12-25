load("@rules_python//python:defs.bzl", "py_binary")
load("@pydeps//:requirements.bzl", "requirement")

py_binary(
    name = "app",
    srcs = ["app.py"],
    data = [
        "robots.txt",
        "static/assets/favicon.ico",
        "static/style/stylesheet.min.css",
        "templates/index.html",
    ],
    deps = [
        requirement("apscheduler"),
        requirement("flask"),
        requirement("matplotlib"),
        requirement("paramiko"),
    ],
)
