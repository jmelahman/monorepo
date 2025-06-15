# Resume

Source for compiling the resume published to [https://jamison.lahman.dev/resume/](https://jmelahman.github.io/resume/).

The resume is compiled to an HTML file with a [Markdown](https://www.markdownguide.org/) file as the input.
See [Compiling files](#compiling-files).
This version comes in light and dark variations.

#### Preview

<p align="left">
  <img src="preview.png" width="450"/>
</p>

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
