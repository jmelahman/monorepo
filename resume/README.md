# Resume

Source for compiling the resume that gets published to [https://jmelahman.github.io/resume/](https://jmelahman.github.io/resume/).

## Personal resume in markdown, LaTeX and Open Document formats.

This resume can be generated from three different formats: Open Document, LaTeX, and Markdown.

### Open Document Format

The first iteration was created using an Open Document format now located in `/odts/`.
This format is no longer supported, but provides a WYSIWYG solution similar to a Word document.

### LaTeX

The second format is a [LaTeX](https://www.latex-project.org/) template which can be compiled with most LaTeX compilers.
The intended way to compile is via [Bazel](https://docs.bazel.build/versions/4.2.1/bazel-overview.html).
See [Compiling files](#compiling-files).
Necessary, user-specific changes in `/src/resume.tex` are prefaced with comments of the format `### <Information to add>`.
The cover letter templates the text from `/src/txts/cover_letter_generic.txt`.

#### Preview

<p align="left">
  <img src="preview.png" width="450"/>
</p>

### Markdown

Lastly, the resume can be compiled to an HTML file with a [Markdown](https://www.markdownguide.org/) file as the input.
The intended way to compile is via [Bazel](https://docs.bazel.build/versions/4.2.1/bazel-overview.html).
See [Compiling files](#compiling-files).
This version comes in light and dark variations.

#### Preview

https://jmelahman.github.io/resume/

## Compiling files

Build all,

```shell
bazel build //resume/...
```

Build single targets,

```shell
bazel build //resume/src/latex:resume
bazel build //resume/src/latex:cover_letter
bazel build //resume/src/markdown:resume
bazel build //resume/src/markdown:resume_light
```

After building a target, open the respective output with,

```shell
xdg-open bazel-bin/src/latex/resume.pdf
xdg-open bazel-bin/src/latex/cover_letter.pdf
xdg-open bazel-bin/src/markdown/resume.html
xdg-open bazel-bin/src/markdown/resume-light.html
```
