# text-editor

A WYSIWYG Markdown editor that runs entirely in the browser. No build step,
and nothing leaves your machine.

![The Markdown editor in rich mode](screenshot.png)

## Features

- Rich editing of the formatted document, or the Markdown source beside a live preview
- Headings, emphasis, strikethrough, code, links, images, quotes, lists, task lists, and tables
- Formatting from a bar that appears over the selected text; tables, code blocks, and dividers from the toolbar
- Markdown typed in rich mode is converted as you go: `## `, `* `, `> `, `` ``` ``
- Open a file from disk or by drag-and-drop; dropped or pasted images are embedded
- Save back to the opened file where the browser supports it, or download a copy
- Open and save files kept on the server, in any browser, where it offers a folder of them
- Reopen recent files from the File menu: from disk where the browser can save back to a file, and elsewhere as they were last opened or saved here
- Export a standalone HTML page, or copy the document as rich text
- The current document is kept as a draft in the browser and restored on the next visit
- Revisions are kept in the browser every few minutes while a document changes, and when it is saved or replaced; any of them can be restored from the File menu
- Light and dark themes, following the system until one is chosen

## Shortcuts

| Key                       | Action                                 |
| ------------------------- | -------------------------------------- |
| `Ctrl+B` / `Ctrl+I`       | Bold / italic                          |
| `Ctrl+K`                  | Link                                   |
| `Tab` / `Shift+Tab`       | Indent / outdent a list item           |
| `Ctrl+Z` / `Ctrl+Shift+Z` | Undo / redo                            |
| `Ctrl+O`                  | Open                                   |
| `Ctrl+S` / `Ctrl+Shift+S` | Save / save as                         |

Right-click a table cell to add, remove, or align rows and columns.

## Caveats

Rich mode keeps the document as a tree, not as text, and writes Markdown back
out in its own style. A file opened and saved there can change where nothing
was edited: `-` bullets become `*`, `_emphasis_` becomes `*emphasis*`, tables
are realigned, the lines of a paragraph are joined into one, a line break
forced within a paragraph starts a new paragraph, and reference-style link
definitions are not preserved.
Markdown mode leaves the source as typed, for as long as the document stays
in it.

Raw HTML is the exception. Front matter, comments, common blocks (`<div>`,
`<details>`, `<svg>`, and the like), and inline tags such as `<kbd>` and
`<sub>` are shown in rich mode and written back exactly as they were. A block
can be moved or deleted there, but only edited in Markdown mode. A document
with any other HTML opens in Markdown mode, and switching it to rich mode
asks first, as that would leave only the text.

SVG is drawn without styles, scripts, gradients, or embedded HTML.

Images with a remote address are fetched by the browser when shown. Relative
image paths do not resolve, as the page cannot see the folder a file came
from.

## Files on the server

Only some browsers let a page write back to a file on disk. Served from a
machine that has the files, the editor can do it through the server instead.
For that the server has to offer, under `/files/`,

- a listing of each folder in JSON, as nginx's `autoindex_format json` gives
- the Markdown files themselves, by `GET` and `PUT`

Open and Save as then choose among those files, and Save writes back to the
one that is open. Reloading the page shows the file as it is on disk; changes
that had not been saved are kept as a revision. Without `/files/` the editor
works with the browser's own files, as before.

## Dependencies

Editing is done by [TOAST UI Editor](https://github.com/nhn/tui.editor) 3.2.2
(MIT), vendored unmodified in `vendor/toastui-editor`. Its usage statistics
are switched off, and its HTML sanitizer is replaced by the allowlist in
`app.js`.

## Development

Open `index.html` in a browser, or serve the directory:

```sh
python -m http.server
```

The tests run in a browser, against the page itself, as there is no other
place the editor exists. With Node and Firefox or Chromium installed:

```sh
node test/run.mjs
```

Or open `/test/` on the server above to run them in a browser of your own.
