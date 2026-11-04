export interface ModelCost {
  readonly tier?: { readonly type: "context"; readonly size: number }
  readonly input: number
  readonly output: number
}

export interface CostFormatOptions {
  /** Inserted between the model name and its price. Defaults to `" · "`. */
  readonly separator?: string
  /** Show a price for models that cost nothing. Defaults to `false`. */
  readonly includeFree?: boolean
  /**
   * Pad the name so the price ends in this many characters, right-aligning it
   * inside the model picker's title column. Undefined leaves the name unpadded.
   */
  readonly padTo?: number
}

const DEFAULT_SEPARATOR = " · "

/**
 * Whether every input price is zero. Mirrors the model picker's own "Free"
 * rule so the plugin never contradicts the built-in label.
 */
export function isFree(cost: readonly ModelCost[]): boolean {
  return cost.length > 0 && cost.every((tier) => tier.input === 0)
}

/**
 * A compact `$input/$output` price per million tokens, or `"Free"` when asked
 * to include free models. Undefined when there is no price to show.
 */
export function formatModelCost(cost: readonly ModelCost[], options: CostFormatOptions = {}): string | undefined {
  if (cost.length === 0) return undefined
  if (isFree(cost)) return options.includeFree ? "Free" : undefined
  const base = cost.find((tier) => tier.tier === undefined) ?? cost[0]
  if (!base) return undefined
  return `${usd(base.input)}/${usd(base.output)}`
}

/** Appends the price to a model's display name, idempotently. */
export function withModelCost(name: string, cost: readonly ModelCost[], options: CostFormatOptions = {}): string {
  const price = formatModelCost(cost, options)
  if (!price) return name
  const suffix = `${options.separator ?? DEFAULT_SEPARATOR}${price}`
  if (name.endsWith(suffix)) return name
  if (options.padTo === undefined) return `${name}${suffix}`
  return padName(name, suffix, options.padTo)
}

/**
 * Pads the name so `name + suffix` is exactly `width` characters. The picker
 * left-aligns titles, so this is what pushes the price to the right edge. Long
 * names are truncated so the price always stays visible.
 */
function padName(name: string, suffix: string, width: number): string {
  const room = width - suffix.length
  if (room <= 0) return `${name}${suffix}`
  const label = name.length > room ? `${name.slice(0, room - 1)}…` : name
  return `${label.padEnd(room)}${suffix}`
}

function usd(value: number): string {
  if (!Number.isFinite(value) || value < 0) return "$?"
  if (value === 0) return "$0"
  if (value < 0.001) return "<$0.001"
  return `$${Number(value.toFixed(4))}`
}
