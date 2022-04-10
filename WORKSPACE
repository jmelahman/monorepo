# Consider https://thundergolfer.com/bazel/python/2021/06/25/a-basic-python-bazel-toolchain/
workspace(name = "monorepo")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

##############################################################################
# Python
##############################################################################
PYTHON_INTERPRETER = "python3.10"

rules_python_version = "0.8.0"

http_archive(
    name = "rules_python",
    sha256 = "9fcf91dbcc31fde6d1edb15f117246d912c33c36f44cf681976bd886538deba6",
    strip_prefix = "rules_python-{version}".format(version = rules_python_version),
    url = "https://github.com/bazelbuild/rules_python/archive/{version}.tar.gz".format(
        version = rules_python_version,
    ),
)

load("@rules_python//python:pip.bzl", "pip_install")

pip_install(
    name = "pydeps",
    python_interpreter = PYTHON_INTERPRETER,
    requirements = "//:third_pary/requirements.txt",
)

##############################################################################
# Mypy
##############################################################################
mypy_integration_version = "c1193a230e3151b89d2e9ed05b986da34075c280"  # HEAD

http_archive(
    name = "mypy_integration",
    sha256 = "2014c4758da248f316b15c95f5e3be2978faacf137042de6586e0a8152b91946",
    strip_prefix = "bazel-mypy-integration-{version}".format(
        version = mypy_integration_version,
    ),
    url = "https://github.com/thundergolfer/bazel-mypy-integration/archive/{version}.tar.gz".format(
        version = mypy_integration_version,
    ),
)

load(
    "@mypy_integration//repositories:repositories.bzl",
    mypy_integration_repositories = "repositories",
)

mypy_integration_repositories()

load("@mypy_integration//:config.bzl", "mypy_configuration")

mypy_configuration("//tools/typing:mypy.ini")

load("@mypy_integration//repositories:deps.bzl", mypy_integration_deps = "deps")

mypy_integration_deps(
    mypy_requirements_file = "//tools/typing:mypy-requirements.txt",
    python_interpreter = PYTHON_INTERPRETER,
)

##############################################################################
# Buildtools
##############################################################################
buildtools_version = "5.0.1"

# Buildtools transitively depends on io_bazel_rules_go.
# https://github.com/bazelbuild/buildtools/blob/a9f46b2bb3de812fce9f5fe59b29e75d95750aed/WORKSPACE#L5-L18
http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2b1641428dff9018f9e85c0384f03ec6c10660d935b750e3fa1492a281a53b0f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "1.17.2")

# Buildtools transitively depends on com_google_protobuf.
# https://github.com/bazelbuild/buildtools/blob/a9f46b2bb3de812fce9f5fe59b29e75d95750aed/WORKSPACE#L40-L51
http_archive(
    name = "com_google_protobuf",
    sha256 = "9b4ee22c250fe31b16f1a24d61467e40780a3fbb9b91c3b65be2a376ed913a1a",
    strip_prefix = "protobuf-3.13.0",
    urls = [
        "https://github.com/protocolbuffers/protobuf/archive/v3.13.0.tar.gz",
    ],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = "7f43df3cca7bb4ea443b4159edd7a204c8d771890a69a50a190dc9543760ca21",
    strip_prefix = "buildtools-{version}".format(
        version = buildtools_version,
    ),
    url = "https://github.com/bazelbuild/buildtools/archive/{version}.tar.gz".format(
        version = buildtools_version,
    ),
)
