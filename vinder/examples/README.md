# Examples

Each directory is a complete project with its `vendor/` committed, so you can
see what vinder generates. Serve one over HTTP, since browsers won't load an ES
module from `file://`, and open it:

```sh
python3 -m http.server -d examples/quickstart
```

- [quickstart](quickstart): what `vinder add canvas-confetti` produces, with
  nothing written by hand except the page.
- [custom-entry](custom-entry): a `vinder.toml` written by hand, for a plain
  `<script>` global from a tree-shaken entry of the project's own, a stylesheet
  from a path inside a package, and license files.

After changing a project's `package.json` or `vinder.toml`, run `vinder sync`
in its directory, and `vinder check` will confirm `vendor/` matches.
