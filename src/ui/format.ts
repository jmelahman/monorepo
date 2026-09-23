/**
 * How a score is written down.
 *
 * The targets are geometric, since stage 1 asks for 300 and each one after it
 * asks for a little over twice the last, so by stage 8 the ask is in the millions and
 * an endless run, which has no last stage, has no last target either. Ten digits
 * do not fit on a phone beside the number chasing them, and past a point they
 * stop being read anyway: nobody compares 148,392,110 to 151,000,000 digit by
 * digit, they compare "148M" to "151M".
 *
 * So the cut is at ten thousand, not at a thousand. An early target of 1450 is
 * exactly the kind of number a player checks against digit by digit, and "1.45K"
 * is worse at that job than "1450". Above the cut, three significant figures
 * keeps the error under half a percent, which is far inside what a progress read
 * needs, and never runs past six characters.
 */

/** Long enough that the ladder outlasts any run; past it, exponent notation. */
const DEFAULT_UNITS: readonly string[] = ["K", "M", "B", "T", "Qa", "Qi", "Sx", "Sp"]

/**
 * The ladder in force, which is a per-language thing: French writes a milliard
 * where English writes a billion, and abbreviates it `Md`.
 *
 * A module-level value that `lang` pushes into, rather than a catalog this file
 * reads, because the edge between the two modules already runs the other way —
 * every catalog imports `formatNumber` and `pluralizer` from here to build its
 * own sentences, so a read back would be a cycle. Same shape as the catalog in
 * force itself; see `current` in `./lang`.
 *
 * It starts English so that anything importing this module without a language
 * having been chosen — a test, a tool — gets numbers rather than `undefined`.
 */
let UNITS = DEFAULT_UNITS

export const setUnits = (units: readonly string[]): void => {
  UNITS = units
}

const PLAIN_BELOW = 10_000

/** Three significant figures, with trailing zeros dropped: 1.20 reads as 1.2. */
const significant = (scaled: number): number =>
  Number(scaled.toFixed(scaled >= 100 ? 0 : scaled >= 10 ? 1 : 2))

export function formatNumber(value: number): string {
  // Not reachable from the engine, which deals in finite integers, but the UI
  // divides by targets, and a screen reading "NaN" is worse than one reading 0.
  if (!Number.isFinite(value)) return value > 0 ? "∞" : "0"

  const sign = value < 0 ? "-" : ""
  const size = Math.abs(value)
  if (size < PLAIN_BELOW) return `${sign}${Math.round(size)}`

  let step = Math.floor(Math.log10(size) / 3)
  // Rounding carries: 999,950 scales to 999.95K, which prints as "1000K" unless
  // the carry is allowed to move it up a unit first.
  if (significant(size / 1000 ** step) >= 1000) step += 1

  const unit = UNITS[step - 1]
  // Trailing zeros go here too, so the fallback reads like everything above it.
  if (!unit)
    return `${sign}${size
      .toExponential(2)
      .replace(/\.?0+e/, "e")
      .replace("e+", "e")}`
  return `${sign}${significant(size / 1000 ** step)}${unit}`
}

/** Gold. Small all run, but an endless run is long and interest compounds. */
export const money = (amount: number): string => `$${formatNumber(amount)}`

/**
 * The forms a sentence takes for a count, keyed by the categories CLDR names.
 *
 * Whole phrases rather than suffixes. English can be pluralized by hanging an
 * `s` on the end and German cannot: one Wort is two Wörter, and a translator
 * handed a suffix has nowhere to put the stem change. It costs a repeated
 * `${count}` in the common case and buys every case that is not English.
 *
 * `other` is the only form required, and everything else falls back to it, so a
 * catalog never has to enumerate a category its sentences do not distinguish.
 * Both Spanish and French quietly select `many` for exact millions, which none
 * of these sentences spell differently, and which would otherwise be six dead
 * keys per string in two files.
 */
export type PluralForms = Partial<Record<Intl.LDMLPluralRule, string>> & { other: string }

export type Plural = (count: number, forms: PluralForms) => string

/**
 * Agreement with a number, which is the language's question rather than the
 * code's, so it is asked of `Intl` rather than of a `=== 1`.
 *
 * Bound to a locale instead of taking one, because the caller that knows the
 * language is the catalog file: `en.ts` names its locale once at the top and
 * every sentence in it then reads `plural(count, …)`. Reading the language in
 * force out of `./lang` instead would have this module importing the module that
 * imports it, and would also be wrong — a catalog's forms belong to the language
 * it is written in, not to the one on screen.
 *
 * English lands where the hand-rolled ternaries this replaced did, `one` at 1
 * and `other` everywhere else including 0. French is why it is worth asking: it
 * puts 0 in the singular, which no amount of care with a ternary would have
 * caught, because nobody writing English has any reason to look for it.
 */
export const pluralizer = (locale: string): Plural => {
  const rules = new Intl.PluralRules(locale)
  return (count, forms) => forms[rules.select(count)] ?? forms.other
}
