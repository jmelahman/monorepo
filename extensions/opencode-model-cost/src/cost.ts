export interface ModelCost {
  readonly tier?: { readonly type: "context"; readonly size: number }
  readonly input: number
  readonly output: number
}

export interface CostFormatOptions {
  /** Inserted between the model name and its price. Defaults to `" · "`. */
  readonly separator?: string
  /**
   * Pad the name so the price ends in this many terminal cells, right-aligning
   * it inside the model picker's title column. Undefined leaves the name
   * unpadded.
   */
  readonly padTo?: number
}

const DEFAULT_SEPARATOR = " · "
const MAX_PAD_TO = 10_000

/**
 * Whether every input price is zero. Mirrors the model picker's own "Free"
 * rule so the plugin never contradicts the built-in label.
 */
export function isFree(cost: readonly ModelCost[]): boolean {
  return cost.length > 0 && cost.every((tier) => tier.input === 0)
}

/**
 * A compact `$input/$output` price per million tokens. Undefined when there is
 * no price to show or the model is free.
 */
export function formatModelCost(cost: readonly ModelCost[]): string | undefined {
  if (cost.length === 0) return undefined
  if (isFree(cost)) return undefined
  const base = cost.find((tier) => tier.tier === undefined) ?? cost[0]
  if (!base) return undefined
  return `${usd(base.input)}/${usd(base.output)}`
}

/** Appends the price to a model's display name, idempotently. */
export function withModelCost(name: string, cost: readonly ModelCost[], options: CostFormatOptions = {}): string {
  const price = formatModelCost(cost)
  if (!price) return name
  const suffix = `${options.separator ?? DEFAULT_SEPARATOR}${price}`
  if (name.endsWith(suffix)) return name
  if (!isValidPadTo(options.padTo)) return `${name}${suffix}`
  return padName(name, suffix, options.padTo)
}

/**
 * Pads the name so `name + suffix` is exactly `width` terminal cells. The
 * picker left-aligns titles, so this is what pushes the price to the right
 * edge. Long names are truncated so the price always stays visible.
 */
function padName(name: string, suffix: string, width: number): string {
  const room = width - Bun.stringWidth(suffix)
  if (room <= 0) return `${name}${suffix}`
  const label = truncateToWidth(name, room)
  return `${label}${" ".repeat(room - Bun.stringWidth(label))}${suffix}`
}

function truncateToWidth(value: string, width: number): string {
  if (Bun.stringWidth(value) <= width) return value
  if (width <= Bun.stringWidth("…")) return "…"

  let result = ""
  const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" })
  for (const { segment } of segmenter.segment(value)) {
    if (Bun.stringWidth(`${result}${segment}…`) > width) break
    result += segment
  }
  return `${result}…`
}

function isValidPadTo(value: number | undefined): value is number {
  return value !== undefined && Number.isSafeInteger(value) && value >= 0 && value <= MAX_PAD_TO
}

function usd(value: number): string {
  if (!Number.isFinite(value) || value < 0) return "$?"
  if (value === 0) return "$0"
  if (value < 0.001) return "<$0.001"
  return `$${Number(value.toFixed(4))}`
}
