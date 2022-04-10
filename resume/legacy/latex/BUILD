load("@bazel_latex//:latex.bzl", "latex_document")

latex_document(
    name = "cover_letter",
    srcs = [
        ":assets/signature.png",
        ":cover_letter_data.txt",
        "@bazel_latex//packages:array",
        "@bazel_latex//packages:geometry",
        "@bazel_latex//packages:graphicx",
        "@bazel_latex//packages:hyperref",
        "@bazel_latex//packages:tabularx",
        "@bazel_latex//packages:verbatim",
        "@bazel_latex//packages:xcolor",
    ],
    main = "cover_letter.tex",
)

latex_document(
    name = "cover_letter_resume",
    srcs = [
        ":output/cover_letter.pdf",
        ":output/resume.pdf",
        "@bazel_latex//packages:calc",
        "@texlive_texmf__texmf-dist__tex__generic__ifxetex",
        "@texlive_texmf__texmf-dist__tex__generic__oberdiek",
        "@texlive_texmf__texmf-dist__tex__latex__eso-pic",
        "@texlive_texmf__texmf-dist__tex__latex__graphics",
        "@texlive_texmf__texmf-dist__tex__latex__graphics-cfg",
        "@texlive_texmf__texmf-dist__tex__latex__graphics-def",
        "@texlive_texmf__texmf-dist__tex__latex__oberdiek",
        "@texlive_texmf__texmf-dist__tex__latex__pdfpages",
    ],
    main = "cover_letter_resume.tex",
    # TODO: Currently doesn't build.
    tags = ["manual"],
)

latex_document(
    name = "resume",
    srcs = [
        "@bazel_latex//packages:array",
        "@bazel_latex//packages:geometry",
        "@bazel_latex//packages:hyperref",
        "@bazel_latex//packages:tabularx",
        "@bazel_latex//packages:verbatim",
        "@bazel_latex//packages:xcolor",
        "@texlive_texmf__texmf-dist__tex__latex__dejavu",
    ],
    main = "resume.tex",
)
