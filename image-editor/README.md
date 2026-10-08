# image-editor

A basic image editor that runs entirely in the browser. No build step, no
dependencies, nothing leaves your machine.

## Features

- Brush, eraser, line, arrow, rectangle, ellipse, text, fill, and color picker tools
- Primary and secondary colors that swap with one key
- Layers: add, delete, reorder, hide, and merge down
- Text stays editable on its own layer: drag to move it, click to edit it
- Open an image from disk, by drag-and-drop, or by pasting from the clipboard
- Save as PNG or copy to the clipboard
- Undo / redo, zoom, mouse / touch / pen input

## Shortcuts

| Key                                 | Action                                                                 |
| ----------------------------------- | ---------------------------------------------------------------------- |
| `B` `E` `L` `A` `R` `O` `T` `G` `I` | Brush, eraser, line, arrow, rectangle, ellipse, text, fill, pick color |
| `X`                                 | Swap primary and secondary colors                                      |
| `[` / `]`                           | Decrease / increase stroke size                                        |
| `Shift` (while dragging)            | Snap lines and arrows to 45°, constrain to square / circle             |
| `Ctrl+Enter` / `Esc`                | Apply / cancel text                                                    |
| `Ctrl+Z` / `Ctrl+Shift+Z`           | Undo / redo                                                            |
| `Ctrl+O` / `Ctrl+S`                 | Open / save                                                            |
| `Ctrl+Scroll` / pinch               | Zoom the canvas around the cursor                                      |
| `Shift+Scroll`                      | Scroll the canvas horizontally                                         |

## Development

Open `index.html` in a browser, or serve the directory:

```sh
python -m http.server
```
