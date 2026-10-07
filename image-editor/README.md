# image-editor

A basic image editor that runs entirely in the browser. No build step, no
dependencies, nothing leaves your machine.

## Features

- Brush, eraser, line, rectangle, ellipse, text, fill, and color picker tools
- Open an image from disk, by drag-and-drop, or by pasting from the clipboard
- Save as PNG or copy to the clipboard
- Undo / redo, zoom, mouse / touch / pen input

## Shortcuts

| Key                             | Action                                                          |
| ------------------------------- | --------------------------------------------------------------- |
| `B` `E` `L` `R` `O` `T` `G` `I` | Brush, eraser, line, rectangle, ellipse, text, fill, pick color |
| `[` / `]`                       | Decrease / increase stroke size                                 |
| `Shift` (while dragging)        | Snap lines to 45°, constrain to square / circle                 |
| `Ctrl+Enter` / `Esc`            | Apply / cancel text                                             |
| `Ctrl+Z` / `Ctrl+Shift+Z`       | Undo / redo                                                     |
| `Ctrl+O` / `Ctrl+S`             | Open / save                                                     |

## Development

Open `index.html` in a browser, or serve the directory:

```sh
python -m http.server
```
