import { Plugin } from "@opencode/plugin"
import { withModelCost } from "./cost.js"

export interface Options {
  /** Inserted between the model name and its price. Defaults to `" · "`. */
  readonly separator?: string
  /**
   * Pad the name so the price ends in this many terminal cells, right-aligning
   * it in the picker's title column. Undefined appends the price directly.
   */
  readonly padTo?: number
}

export default Plugin.define({
  id: "model-cost",
  async setup(context) {
    const options = readOptions(context.options)

    // The TUI renders `ModelInfo.name` in the model picker, so appending the
    // price here puts it in the selector without replacing the built-in dialog.
    await context.model.transform((models) => {
      for (const model of models.list()) {
        const name = withModelCost(model.name, model.cost, options)
        if (name === model.name) continue
        models.update(model.providerID, model.id, (draft) => {
          draft.name = name
        })
      }
    })
  },
})

function readOptions(options: Readonly<Record<string, any>>): Options {
  return {
    separator: typeof options.separator === "string" ? options.separator : undefined,
    padTo:
      typeof options.padTo === "number" &&
      Number.isSafeInteger(options.padTo) &&
      options.padTo >= 0 &&
      options.padTo <= 10_000
        ? options.padTo
        : undefined,
  }
}
