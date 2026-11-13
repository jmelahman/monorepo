import { Plugin } from "@opencode/plugin"
import { isValidPadTo, withModelCost } from "./cost.js"

export interface Options {
  /** Inserted between the model name and its price. Defaults to `" · "`. */
  readonly separator?: string
  /**
   * Pad the name so the price ends in this many terminal cells, right-aligning
   * it in the picker's title column. Undefined appends the price directly.
   */
  readonly padTo?: number
}

/**
 * The catalog name and the name this plugin produced, per model. Lets a later
 * transform re-decorate from the original name instead of trusting
 * `endsWith`, so changed options re-pad and look-alike names are not skipped.
 * Module scope keeps it across `setup` calls on config reload.
 */
const decorated = new Map<string, { readonly original: string; readonly name: string }>()

export default Plugin.define({
  id: "model-cost",
  async setup(context) {
    const options = readOptions(context.options)

    // The TUI renders `ModelInfo.name` in the model picker, so appending the
    // price here puts it in the selector without replacing the built-in dialog.
    await context.model.transform((models) => {
      for (const model of models.list()) {
        const key = `${model.providerID}/${model.id}`
        const previous = decorated.get(key)
        const original = previous?.name === model.name ? previous.original : model.name
        const name = withModelCost(original, model.cost, options)
        decorated.set(key, { original, name })
        if (name === model.name) continue
        models.update(model.providerID, model.id, (draft) => {
          draft.name = name
        })
      }
    })
  },
})

function readOptions(options: Readonly<Record<string, any>> | undefined): Options {
  const separator = options?.separator
  const padTo = options?.padTo
  return {
    separator: typeof separator === "string" ? separator : undefined,
    padTo: isValidPadTo(padTo) ? padTo : undefined,
  }
}
