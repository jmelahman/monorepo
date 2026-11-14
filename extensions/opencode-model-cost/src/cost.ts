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
const ELLIPSIS = "…"
export const MAX_PAD_TO = 10_000

const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" })

/**
 * Whether every input price is zero. Mirrors the model picker's own "Free"
 * rule so the plugin never contradicts the built-in label.
 */
export function isFree(cost: readonly ModelCost[]): boolean {
  return cost.length > 0 && cost.every((tier) => tier.input === 0)
}

/**
 * A compact `$input/$output` price per million tokens. Undefined when there is
 * no price to show or the model is free. When the catalog only has context
 * tiers, the smallest tier is shown with its threshold so it is not mistaken
 * for a base price.
 */
export function formatModelCost(cost: readonly ModelCost[]): string | undefined {
  if (cost.length === 0 || isFree(cost)) return undefined
  const base = cost.find((tier) => tier.tier === undefined)
  if (base) return price(base)
  const lowest = cost.reduce((a, b) => ((b.tier?.size ?? 0) < (a.tier?.size ?? 0) ? b : a))
  return `${price(lowest)} (${tokens(lowest.tier?.size ?? 0)}+)`
}

/**
 * Appends the price to a model's display name. A name that already ends with
 * the exact suffix is returned unchanged, so callers should pass the original
 * catalog name when they need to re-decorate (for example after `padTo`
 * changes).
 */
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
  const room = width - stringWidth(suffix)
  if (room <= 0) return `${name}${suffix}`
  const label = truncateToWidth(name, room)
  return `${label}${" ".repeat(room - stringWidth(label))}${suffix}`
}

function truncateToWidth(value: string, width: number): string {
  if (stringWidth(value) <= width) return value
  const budget = width - stringWidth(ELLIPSIS)
  if (budget <= 0) return ELLIPSIS

  let result = ""
  let used = 0
  for (const { segment } of segmenter.segment(value)) {
    const cells = stringWidth(segment)
    if (used + cells > budget) break
    result += segment
    used += cells
  }
  return `${result}${ELLIPSIS}`
}

/**
 * Terminal cell width. Uses `Bun.stringWidth` when running under Bun and a
 * grapheme-based approximation otherwise (the plugin SDK also loads under
 * Node). Ambiguous-width characters count as narrow, matching how the TUI
 * itself measures titles.
 */
export function stringWidth(value: string): number {
  if (typeof Bun !== "undefined") return Bun.stringWidth(value)
  return approximateWidth(value)
}

/** The non-Bun fallback for `stringWidth`. Exported for tests. */
export function approximateWidth(value: string): number {
  let width = 0
  for (const { segment } of segmenter.segment(value)) width += graphemeWidth(segment)
  return width
}

const ZERO_WIDTH = /^[\p{Mn}\p{Me}\p{Cf}\p{Cc}]+$/u
const WIDE =
  /\p{Extended_Pictographic}|[\u{1100}-\u{115F}\u{2E80}-\u{303E}\u{3041}-\u{33FF}\u{3400}-\u{4DBF}\u{4E00}-\u{9FFF}\u{A000}-\u{A4CF}\u{AC00}-\u{D7A3}\u{F900}-\u{FAFF}\u{FE30}-\u{FE4F}\u{FF00}-\u{FF60}\u{FFE0}-\u{FFE6}\u{20000}-\u{3FFFD}]/u

function graphemeWidth(segment: string): number {
  if (ZERO_WIDTH.test(segment)) return 0
  return WIDE.test(segment) ? 2 : 1
}

export function isValidPadTo(value: unknown): value is number {
  return typeof value === "number" && Number.isSafeInteger(value) && value >= 0 && value <= MAX_PAD_TO
}

function price(cost: ModelCost): string {
  return `${usd(cost.input)}/${usd(cost.output)}`
}

function tokens(value: number): string {
  if (value >= 1_000_000 && value % 1_000_000 === 0) return `${value / 1_000_000}M`
  if (value >= 1_000) return `${Number((value / 1_000).toFixed(1))}K`
  return String(value)
}

function usd(value: number): string {
  if (!Number.isFinite(value) || value < 0) return "$?"
  if (value === 0) return "$0"
  if (value < 0.001) return "<$0.001"
  return `$${Number(value.toFixed(4))}`
}
