# Consider https://thundergolfer.com/bazel/python/2021/06/25/a-basic-python-bazel-toolchain/
workspace(
    name = "monorepo",
)

##############################################################################
# Bazel
##############################################################################
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "bazel_skylib",
    sha256 = "97e70364e9249702246c0e9444bccdc4b847bed1eb03c5a3ece4f83dfe6abc44",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.0.2/bazel-skylib-1.0.2.tar.gz",
        "https://github.com/bazelbuild/bazel-skylib/releases/download/1.0.2/bazel-skylib-1.0.2.tar.gz",
    ],
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
    requirements = "//:third_party/requirements.txt",
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
    patch_args = ["-p1"],
    patches = [
        "@//:third_party/mypy_integration-stubs.patch",
        "@//:third_party/mypy_integration-site_packages.patch",
    ],
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
# gtest
##############################################################################
gtest_version = "1.11.0"

http_archive(
    name = "com_google_googletest",
    sha256 = "b4870bf121ff7795ba20d20bcdd8627b8e088f2d1dab299a031c1034eddc93d5",
    strip_prefix = "googletest-release-{version}".format(version = gtest_version),
    url = "https://github.com/google/googletest/archive/release-{version}.tar.gz".format(
        version = gtest_version,
    ),
)

##############################################################################
# npm
##############################################################################
#rules_nodejs_version = "5.4.0"
#
#http_archive(
#    name = "build_bazel_rules_nodejs",
#    sha256 = "ac7eb554af28232dc43deaf7e7247d12b128a97ecb676c2e5d028c5d521b0433",
#    strip_prefix = "rules_nodejs-{version}".format(version = rules_nodejs_version),
#    url = "https://github.com/bazelbuild/rules_nodejs/archive/{version}.tar.gz".format(
#        version = rules_nodejs_version,
#    ),
#)
#
#load("@build_bazel_rules_nodejs//:repositories.bzl", "build_bazel_rules_nodejs_dependencies")
#
#build_bazel_rules_nodejs_dependencies()
#
## Fetch transitive Bazel dependencies of karma_web_test
#http_archive(
#    name = "io_bazel_rules_webtesting",
#    sha256 = "e9abb7658b6a129740c0b3ef6f5a2370864e102a5ba5ffca2cea565829ed825a",
#    urls = ["https://github.com/bazelbuild/rules_webtesting/releases/download/0.3.5/rules_webtesting.tar.gz"],
#)
#
#load("@build_bazel_rules_nodejs//:index.bzl", "yarn_install")
#
#yarn_install(
#    name = "npm",
#    package_json = "//game:package.json",
#    yarn_lock = "//game:yarn.lock",
#)
#
##############################################################################
# LaTeX
##############################################################################
BAZEL_LATEX_VERSION = "1.0"

http_archive(
    name = "bazel_latex",
    sha256 = "f81604ec9318364c05a702798c5507c6e5257e851d58237d5f171eeca4d6e2db",
    strip_prefix = "bazel-latex-{}".format(BAZEL_LATEX_VERSION),
    url = "https://github.com/ProdriveTechnologies/bazel-latex/archive/v{}.tar.gz".format(
        BAZEL_LATEX_VERSION,
    ),
)

load("@bazel_latex//:repositories.bzl", "latex_repositories")

latex_repositories()

##############################################################################
# Pandoc
##############################################################################
BAZEL_PANDOC_VERSION = "51605c25d3ae69a5b325d9986ac7ce8c9741ffa9"

http_archive(
    name = "bazel_pandoc",
    sha256 = "0fcfa6a461098c8b8b9ba2f2d236d7f7aed988953f303c22c8c9cf96eb0c651f",
    strip_prefix = "bazel-pandoc-%s" % BAZEL_PANDOC_VERSION,
    url = "https://github.com/ProdriveTechnologies/bazel-pandoc/archive/{}.tar.gz".format(
        BAZEL_PANDOC_VERSION,
    ),
)

load("@bazel_pandoc//:repositories.bzl", "pandoc_repositories")

pandoc_repositories()

##############################################################################
# Bats
##############################################################################
BAZEL_BATS_VERSION = "05902c66e7aba5bca0816109e9f34e2dbebe19f6"

http_archive(
    name = "bazel_bats",
    sha256 = "0be1795d8052c54e1068b3b0a648d67de0b9bf43cd15fd7bef73b6460b73b78f",
    strip_prefix = "bazel-bats-{version}".format(version = BAZEL_BATS_VERSION),
    url = "https://github.com/filmil/bazel-bats/archive/{version}.tar.gz".format(
        version = BAZEL_BATS_VERSION,
    ),
)

load("@bazel_bats//:deps.bzl", "bazel_bats_dependencies")

bazel_bats_dependencies()

##############################################################################
# Shellmock
##############################################################################
BAZEL_SHELLMOCK_VERSION = "6612562e9683366490c48c83a97df0ea490772b7"

http_archive(
    name = "bazel_shellmock",
    sha256 = "f935f7c901e8a17c95d7367e4ed4645aad682e61ddd1c4b2cd82c2b74ec206a9",
    strip_prefix = "bazel-shellmock-{version}".format(version = BAZEL_SHELLMOCK_VERSION),
    url = "https://github.com/jmelahman/bazel-shellmock/archive/{version}.tar.gz".format(
        version = BAZEL_SHELLMOCK_VERSION,
    ),
)

load("@bazel_shellmock//:deps.bzl", "bazel_shellmock_dependencies")

bazel_shellmock_dependencies()

#local_repository(
#    name = "bazel_shellmock_git",
#    path = "/home/jamison/code/bazel-shellmock",
#)
#
#load("@bazel_shellmock_git//:deps.bzl", "bazel_shellmock_dependencies")
#
#bazel_shellmock_dependencies()
