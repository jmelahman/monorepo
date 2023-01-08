load("@pydeps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_binary")

py_binary(
    name = "app",
    srcs = ["app.py"],
    data = [
        "robots.txt",
        "static/assets/favicon.ico",
        "static/scripts/script.js",
        "static/style/dot-flashing.css",
        "templates/index.html",
    ],
    deps = [
        requirement("flask"),
        requirement("six"),
        requirement("transformers"),
    ],
)
