load("@rules_python//python/pip_install:requirements.bzl", "compile_pip_requirements")

compile_pip_requirements(
    name = "requirements",
    src = "requirements.in",
    requirements_txt = "requirements.txt",
    extra_args = [
        "--allow-unsafe",
    ],
)
