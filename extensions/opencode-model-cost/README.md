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
model catalog. Free models are left alone because the picker already labels
them `Free`.

## Enable

This repository loads the plugin through `.opencode/opencode.json`:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "plugins": ["../extensions/opencode-model-cost"]
}
```

To use it from any project, add the same entry to the global
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
        "includeFree": false
      }
    }
  ]
}
```

| Option        | Default | Description                                              |
| ------------- | ------- | -------------------------------------------------------- |
| `separator`   | `" · "` | Text between the model name and the price.               |
| `includeFree` | `false` | Append `Free` to models whose input price is zero.        |

## Notes

- The price is part of the model's display name, so it also appears anywhere
  else OpenCode shows that name (for example the prompt metadata). That is the
  trade-off for not replacing the built-in picker.
- OpenCode's own redesigned picker can show prices for the highlighted model
  with [`"experimental": { "new-designs": true }`](https://opencode.ai/v2/docs/cli/config#debug-and-experimental)
  in `cli.json`. This plugin is for showing prices on every row without
  changing the rest of the UI.

## Development

```sh
cd extensions/opencode-model-cost
bun install
bun test
bun run typecheck
```

The plugin source is `src/index.ts`; the pure formatting logic and its tests
live in `src/cost.ts` and `test/cost.test.ts`.
