load("@bazel_latex//:latex.bzl", "latex_document")

latex_document(
    name = "cover_letter",
    main = "cover_letter.tex",
    srcs = [
        ":cover_letter_data.txt",
        ":assets/signature.png",
        "@bazel_latex//packages:geometry",
        "@bazel_latex//packages:array",
        "@bazel_latex//packages:graphicx",
        "@bazel_latex//packages:hyperref",
        "@bazel_latex//packages:xcolor",
        "@bazel_latex//packages:tabularx",
        "@bazel_latex//packages:verbatim",
    ],
)

latex_document(
    name = "cover_letter_resume",
    main = "cover_letter_resume.tex",
    srcs = [
        ":output/cover_letter.pdf",
        ":output/resume.pdf",
        "@texlive_texmf__texmf-dist__tex__latex__pdfpages",
        "@texlive_texmf__texmf-dist__tex__latex__eso-pic",
        "@texlive_texmf__texmf-dist__tex__latex__graphics",
        "@texlive_texmf__texmf-dist__tex__latex__graphics-cfg",
        "@texlive_texmf__texmf-dist__tex__latex__graphics-def",
        "@texlive_texmf__texmf-dist__tex__latex__oberdiek",
        "@texlive_texmf__texmf-dist__tex__generic__ifxetex",
        "@texlive_texmf__texmf-dist__tex__generic__oberdiek",
        "@bazel_latex//packages:calc",
    ],
    # TODO: Currently doesn't build.
    tags = ["manual"],
)

latex_document(
    name = "resume",
    main = "resume.tex",
    srcs = [
        "@bazel_latex//packages:geometry",
        "@bazel_latex//packages:array",
        "@bazel_latex//packages:hyperref",
        "@bazel_latex//packages:xcolor",
        "@bazel_latex//packages:tabularx",
        "@bazel_latex//packages:verbatim",
        "@texlive_texmf__texmf-dist__tex__latex__dejavu",
    ],
)
