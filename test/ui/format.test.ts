import { describe, expect, it } from "vitest"
import { formatNumber, money, pluralizer } from "../../src/ui/format"
import { en } from "../../src/ui/lang/en"

describe("score formatting", () => {
  it("writes small numbers out in full", () => {
    // Everything a normal run ever shows is on this side of the cut, and on this
    // side the digits are the point: a target of 1450 is checked against a score
    // of 1402 digit by digit, and no abbreviation helps with that.
    expect(formatNumber(0)).toBe("0")
    expect(formatNumber(300)).toBe("300")
    expect(formatNumber(1450)).toBe("1450")
    expect(formatNumber(9999)).toBe("9999")
  })

  it("abbreviates to three significant figures above it", () => {
    expect(formatNumber(10_000)).toBe("10K")
    expect(formatNumber(12_400)).toBe("12.4K")
    expect(formatNumber(148_392)).toBe("148K")
    expect(formatNumber(1_352_000)).toBe("1.35M")
    // Stage 20, roughly, which is the number that made this worth writing.
    expect(formatNumber(2_200_000_000)).toBe("2.2B")
    expect(formatNumber(4.5e12)).toBe("4.5T")
  })

  it("drops trailing zeros rather than padding to three", () => {
    expect(formatNumber(2_000_000)).toBe("2M")
    expect(formatNumber(2_500_000)).toBe("2.5M")
    expect(formatNumber(2_050_000)).toBe("2.05M")
  })

  it("carries into the next unit instead of printing 1000 of the last", () => {
    // 999,950 scales to 999.95K, and three significant figures rounds that to a
    // flat thousand. Printed as "1000K" it is both wrong-looking and one
    // character longer than the abbreviation was meant to allow.
    expect(formatNumber(999_950)).toBe("1M")
    expect(formatNumber(999_999_999)).toBe("1B")
    expect(formatNumber(999_000)).toBe("999K")
  })

  it("falls back to exponent notation past the ladder", () => {
    // Unreachable by any run, but endless has no ceiling to argue from, and a
    // number with no unit left would otherwise print as "undefined".
    expect(formatNumber(1e27)).toBe("1e27")
    expect(formatNumber(Number.POSITIVE_INFINITY)).toBe("∞")
  })

  it("keeps the sign", () => {
    expect(formatNumber(-12_400)).toBe("-12.4K")
    expect(formatNumber(-5)).toBe("-5")
  })

  it("rounds fractions, which the meter and the solve floor both produce", () => {
    expect(formatNumber(1449.6)).toBe("1450")
    expect(formatNumber(0.4)).toBe("0")
  })

  it("prints gold through the same rule", () => {
    expect(money(4)).toBe("$4")
    expect(money(12_400)).toBe("$12.4K")
  })
})

describe("agreement with a count", () => {
  const en_ = pluralizer("en")

  it("lands exactly where the hand-rolled ternaries did in English", () => {
    // The whole of the English rule, and the reason the ternaries looked right
    // for as long as they did: one at 1, plural at everything else, zero
    // included.
    expect(en_(1, { one: "1 run", other: "1 runs" })).toBe("1 run")
    expect(en_(0, { one: "0 run", other: "0 runs" })).toBe("0 runs")
    expect(en_(2, { one: "2 run", other: "2 runs" })).toBe("2 runs")
  })

  it("puts zero in the singular in French, which is why this is not a ternary", () => {
    // Nobody writing English has a reason to look for this, and no amount of
    // care with a `=== 1` would have found it. 0 partie, 1 partie, 2 parties.
    const fr = pluralizer("fr")
    expect(fr(0, { one: "0 partie", other: "0 parties" })).toBe("0 partie")
    expect(fr(1, { one: "1 partie", other: "1 parties" })).toBe("1 partie")
    expect(fr(2, { one: "2 partie", other: "2 parties" })).toBe("2 parties")
  })

  it("falls back to other for any category a catalog did not spell", () => {
    // Spanish and French both select `many` at an exact million. No sentence in
    // the game says anything different there, and a catalog that had to write
    // the key anyway would be six dead entries per string in two files.
    const es = pluralizer("es")
    expect(es(1_000_000, { one: "una", other: "otras" })).toBe("otras")
  })

  it("is what the English catalog's own counted sentences are built on", () => {
    expect(en.ui.title.runs(1)).toBe("1 run")
    expect(en.ui.title.runs(0)).toBe("0 runs")
    expect(en.ui.stats.wins(1)).toBe("win")
    expect(en.ui.stats.wins(0)).toBe("wins")
    expect(en.ui.tip.tileChips("a", 1)).toBe("A · +1 chip")
    expect(en.ui.tip.tileChips("a", 3)).toBe("A · +3 chips")
    // Zero is its own sentence rather than a plural form, in both tips.
    expect(en.ui.tip.tileChips("a", 0)).toBe("A · no chips")
    expect(en.ui.tip.keyChips("a", 0)).toBe("A · no chips")
  })
})
