load("@pip_deps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_binary")

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
        requirement("types-werkzeug"),
        requirement("requests"),
    ],
)
