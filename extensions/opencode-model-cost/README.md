# opencode-model-cost

An OpenCode plugin that shows each model's input/output price in the model
selector (`/models` or the leader-key picker).

The plugin is a **server part**. It uses the `model.transform` hook to append a
compact price to each model's display name:

```
Claude Sonnet 4.5 · $3/$15
```

The built-in picker already renders `ModelInfo.name`, so no custom dialog or
TUI part is needed. The price is present on every row, on the home screen and
inside sessions, and model selection keeps working exactly as before.

Prices are USD per million tokens, using the base (untiered) entry from the
model catalog. A model with only context-tier prices shows its lowest tier with
the threshold, for example `$6/$22.5 (200K+)`. Free models are left alone because the picker already labels
them `Free`.

## Enable

This repository loads the plugin through `.opencode/opencode.json`, with the
price right-aligned and no separator (see [`padTo`](#padto)):

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "plugins": [
    {
      "package": "../extensions/opencode-model-cost",
      "options": { "separator": "", "padTo": 52 }
    }
  ]
}
```

That renders rows like `Claude Sonnet 4.5                             $3/$15`.
The bare string form uses the defaults (`Claude Sonnet 4.5 · $3/$15`). To use
it from any project, add an entry to the global
`~/.config/opencode/opencode.jsonc` with an absolute path:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "plugins": ["/home/jamison/code/monorepo/extensions/opencode-model-cost"]
}
```

Configuration changes reload without restarting OpenCode.

## Options

Pass options through the object form of the plugin entry:

```jsonc
{
  "plugins": [
    {
      "package": "../extensions/opencode-model-cost",
      "options": {
        "separator": " · ",
        "padTo": 52
      }
    }
  ]
}
```

| Option      | Default     | Description                                             |
| ----------- | ----------- | ------------------------------------------------------- |
| `separator` | `" · "`     | Text between the model name and the price.              |
| `padTo`     | `undefined` | Pad the name so the price ends at this terminal column. |

### `padTo`

The picker left-aligns titles, so `padTo` is the only way a plugin can push the
price toward the right edge: it pads the name with spaces until `name + price`
is exactly `padTo` terminal cells. The model dialog is 60 cells wide, which leaves
52 cells for a title with no trailing label, so `52` lines prices up with the
built-in `Free` text. Long names are truncated to keep the price visible.

It is a name-level workaround, not real alignment. Measured against v2.2.5:

- In the default list, prices end at the same column as `Free`.
- Search results put the provider name in the row's trailing slot, which takes
  that space back and clips the price off the row.
- The prompt footer shows the name too, so the padding becomes a visible gap
  there.

Use the [upstream patch](#upstream-picker-patch) if you want the real
flush-right, muted price.

## Notes

- The price is part of the model's display name, so it also appears anywhere
  else OpenCode shows that name (for example the prompt metadata). That is the
  trade-off for not replacing the built-in picker.
- OpenCode's own redesigned picker can show prices for the highlighted model
  with [`"experimental": { "new-designs": true }`](https://opencode.ai/v2/docs/cli/config#debug-and-experimental)
  in `cli.json`. This plugin is for showing prices on every row without
  changing the rest of the UI.

## Upstream picker patch

Appending the price to the name is the only thing a plugin can do, so the price
inherits the row title's styling instead of sitting flush right in the row's
gray trailing slot like the built-in `Free` label. Getting that treatment means
changing the picker itself, which a plugin cannot reach (see
[opencode#60856](https://github.com/anomalyco/opencode/issues/60856)).

`patches/0001-tui-show-model-prices-in-picker-footer.patch` is that change
against the OpenCode `dev` branch. It extracts `formatModelPrice` in
`packages/tui/src/component/model-options.tsx` and has
`dialog-model.tsx`'s `trailing()` return it, so paid models get the same
right-aligned secondary text as `Free`, and adds tests for the formatter.

Apply it to an OpenCode checkout and build from source:

```sh
git clone https://github.com/anomalyco/opencode.git
cd opencode
git checkout dev
git apply /path/to/extensions/opencode-model-cost/patches/0001-tui-show-model-prices-in-picker-footer.patch
bun install
cd packages/tui && bun test test/cli/cmd/tui/model-options.test.ts
```

The patch does not apply to a released binary, so it needs a source build or an
upstream PR. Once it is in, remove this plugin from `plugins` so the price is
not shown twice.

## Development

```sh
cd extensions/opencode-model-cost
bun install
bun test
bun run typecheck
```

The plugin source is `src/index.ts`; the pure formatting logic and its tests
live in `src/cost.ts` and `test/cost.test.ts`.
