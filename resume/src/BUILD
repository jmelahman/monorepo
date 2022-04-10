load("@bazel_pandoc//:pandoc.bzl", "pandoc")

load("@//resume/src:stylesheet.bzl", "themed_stylesheet")

themed_stylesheet(
    name = "dark_stylesheet",
    src = "dark.css",
)

themed_stylesheet(
    name = "light_stylesheet",
    src = "light.css",
)

pandoc(
    name = "resume",
    src = "resume.md",
    output = "resume.html",
    to_format = "html",
    css = ":dark_stylesheet.css",
)

pandoc(
    name = "resume_light",
    src = "resume.md",
    output = "resume-light.html",
    to_format = "html",
    css = ":light_stylesheet.css",
)
