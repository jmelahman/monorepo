# text-editor

A basic Markdown editor that runs entirely in the browser. No build step, no
dependencies, nothing leaves your machine.

## Features

- Live preview beside the source, with edit-only and preview-only views and synced scrolling
- Headings, emphasis, strikethrough, code, links, images, quotes, lists, task lists, and tables
- Toolbar and shortcuts for formatting; lists and quotes continue as you type
- Open a file from disk or by drag-and-drop
- Save back to the opened file where the browser supports it, or download a copy
- Export a standalone HTML page, or copy the rendered document as rich text
- The current document is kept as a draft in the browser and restored on the next visit

## Shortcuts

| Key                         | Action                                           |
| --------------------------- | ------------------------------------------------ |
| `Ctrl+B` / `Ctrl+I`         | Bold / italic                                    |
| `Ctrl+K`                    | Link; pasting a URL over a selection also links  |
| `Enter`                     | Continue the list or quote, or end it when empty |
| `Tab` / `Shift+Tab`         | Indent / outdent                                 |
| `Esc`, then `Tab`           | Move focus out of the editor                     |
| `Ctrl+Z` / `Ctrl+Shift+Z`   | Undo / redo                                      |
| `Ctrl+O`                    | Open                                             |
| `Ctrl+S` / `Ctrl+Shift+S`   | Save / save as                                   |

## Markdown support

The parser in `app.js` covers the common parts of CommonMark and GitHub
Flavored Markdown rather than every corner of either spec. Raw HTML is
allowed, but everything shown in the preview passes through an allowlist
first: scripts, styles, event handlers, and `javascript:` links are removed.

Images with a remote address are fetched by the browser when previewed.
Relative image paths do not resolve, as the page cannot see the folder a file
came from.

## Development

Open `index.html` in a browser, or serve the directory:

```sh
python -m http.server
```
