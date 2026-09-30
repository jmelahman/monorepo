import { h } from "./dom"

/**
 * Card text with its amounts marked in the colour of what they add to, so
 * "×2 mult" and "+2 mult" stop being one glyph apart, and "+4 points" and
 * "+4 mult" stop being one word apart.
 *
 * The first was the reason. At the shop's 13px, in all three skins' faces
 * (system-ui, IBM Plex Sans, Jost), `×` and `+` are the same width at the same
 * height, and the digit beside them carries all the weight, so the two cards
 * differed by a mark the eye reads last. They are the most different cards on
 * the shelf: on a stage-one guess a ×2 doubles a mult of about eighteen and a +2
 * adds a tenth to it. Points and gold came after, for the same reason one step
 * down: the scoring readout already speaks in blue points, red mult and gold
 * money, and a sentence that says the same three things in grey makes the
 * player find the unit word to learn which one a card feeds.
 *
 * Marked here rather than in the catalogs. Every relic, modifier and consumable
 * sentence already says its amount the same way in all four languages, a sign
 * and a number before its unit word, and money as `$` and a number. Putting
 * markup in the strings instead would make every translator re-type it and give
 * the contract in `lang/types.ts` a second kind of string.
 *
 * An amount with a `$` in it is gold, wherever it stands: "+$1 per yellow
 * tile", "Earn $2", and the "$5" in The Mint's "+3 mult per $5 you hold", which
 * is a threshold rather than a payout but is still money on the table. Any other
 * amount takes the unit its clause names, and inherits the one before it when
 * its clause names nothing, which is how "+24 mult on a gray, +14 on a yellow",
 * "+0 to +20 mult" and "×1 mult, plus ×0.1 for every shape level" read. "+1 to
 * your solve multiplier" names nothing, since `mult` is matched as a word and
 * not as a prefix, and is left alone.
 *
 * The same test for `×` as for `+`, although every `×` on a shelf is mult. It
 * was "a `×` is always mult" until the codex took this too, and the codex holds
 * the two that are not: The Auditor's "capped at ×2", which is the solve
 * multiplier, and an ascension's "(×1.5 in all)", which is the target. Both
 * name nothing after the amount, so both inherit the nothing before them.
 *
 * Only a ×mult gets the chip. It is the one pair that differs by a glyph, and
 * nothing a card says multiplies points or gold.
 */

const AMOUNT = /(?:[×+]\$?|\$)\d+(?:[.,]\d+)?/g
/** The clause an amount governs: up to the next comma or full stop. */
const CLAUSE = /^[^,.;]*/
/** Singular as well as plural: "Chaque consonne vaut +1 point". */
const UNIT = /\b(mult|points?|puntos?|punkte?)\b/i

type Kind = "mult" | "chips" | "gold"

export function withAmounts(text: string): Array<Node | string> {
  const out: Array<Node | string> = []
  let at = 0
  let previous: Kind | null = null
  for (const token of text.matchAll(AMOUNT)) {
    const start = token.index ?? 0
    const amount = token[0]
    const clause = CLAUSE.exec(text.slice(start + amount.length))?.[0] ?? ""
    const unit = UNIT.exec(clause)?.[1]?.toLowerCase()
    const kind: Kind | null = amount.includes("$")
      ? "gold"
      : unit
        ? unit === "mult"
          ? "mult"
          : "chips"
        : previous
    previous = kind
    if (!kind) continue
    if (start > at) out.push(text.slice(at, start))
    const style = kind === "mult" && amount.startsWith("×") ? "amt-times" : `amt-${kind}`
    out.push(h("span", { class: `amt ${style}` }, amount))
    at = start + amount.length
  }
  if (at < text.length) out.push(text.slice(at))
  return out
}
