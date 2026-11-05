# viewer

A PDF viewer that runs entirely in the browser. No build step and no server-side
processing: documents stay on your device unless you explicitly fetch one from
a URL.

## Features

- Open a PDF from disk, a URL, drag-and-drop, or the clipboard
- Continuous vertical scrolling with lazy page rendering
- Previous/next page and direct page navigation
- Fit-page, fit-width, preset, and custom zoom levels
- Distraction-free full-screen mode
- Invert colors for a dark-mode reading view while preserving hues
- High-DPI rendering and password-protected PDF support
- Remembers the inversion preference

## Shortcuts

| Key                       | Action                  |
| ------------------------- | ----------------------- |
| `Ctrl+O`                  | Open a PDF              |
| `I`                       | Toggle inverted colors  |
| `F` / `Esc`               | Enter / exit full screen |
| `Page Up` / `Page Down`   | Previous / next page    |
| `Home` / `End`            | First / last page       |
| `-` / `+`                 | Zoom out / in           |
| `0`                       | Fit page                |
| `Ctrl+Scroll` / pinch     | Zoom around the pointer |

## Development

Serve the directory (ES modules and the PDF.js worker require HTTP):

```sh
python -m http.server
```

Then open <http://localhost:8000>.

PDF.js is vendored with [vinder](../vinder). After changing `package.json` or
`vinder.toml`, regenerate `vendor/` with:

```sh
go run ../vinder sync
```
