import { describe, expect, test } from "bun:test"
import { MAX_PAD_TO, approximateWidth, formatModelCost, isFree, withModelCost } from "../src/cost"

const paid = [{ input: 3, output: 15 }]

describe("formatModelCost", () => {
  test("formats the base input and output price per million tokens", () => {
    expect(formatModelCost(paid)).toBe("$3/$15")
  })

  test("keeps fractional prices readable", () => {
    expect(formatModelCost([{ input: 0.15, output: 0.6 }])).toBe("$0.15/$0.6")
  })

  test("shows sub-cent prices without rounding them to zero", () => {
    expect(formatModelCost([{ input: 0.0001, output: 0.0002 }])).toBe("<$0.001/<$0.001")
  })

  test("uses the untiered entry when context tiers are present", () => {
    const tiered = [
      { tier: { type: "context" as const, size: 200_000 }, input: 6, output: 22.5 },
      { input: 3, output: 15 },
    ]
    expect(formatModelCost(tiered)).toBe("$3/$15")
  })

  test("marks the threshold when only context tiers are present", () => {
    const tiered = [
      { tier: { type: "context" as const, size: 1_000_000 }, input: 8, output: 30 },
      { tier: { type: "context" as const, size: 200_000 }, input: 6, output: 22.5 },
    ]
    expect(formatModelCost(tiered)).toBe("$6/$22.5 (200K+)")
  })

  test("returns nothing for a model with no cost data", () => {
    expect(formatModelCost([])).toBeUndefined()
  })

  test("leaves free models to the picker's built-in Free label", () => {
    expect(formatModelCost([{ input: 0, output: 0 }])).toBeUndefined()
  })
})

describe("isFree", () => {
  test("treats a model as free only when every input price is zero", () => {
    expect(isFree([{ input: 0, output: 4 }])).toBe(true)
    expect(isFree(paid)).toBe(false)
  })
})

describe("withModelCost", () => {
  test("appends the price to the display name", () => {
    expect(withModelCost("Claude Sonnet", paid)).toBe("Claude Sonnet · $3/$15")
  })

  test("does not append the price twice", () => {
    expect(withModelCost("Claude Sonnet · $3/$15", paid)).toBe("Claude Sonnet · $3/$15")
  })

  test("leaves the name alone when there is no price to show", () => {
    expect(withModelCost("Local model", [])).toBe("Local model")
  })

  test("honours a custom separator", () => {
    expect(withModelCost("Claude Sonnet", paid, { separator: "  " })).toBe("Claude Sonnet  $3/$15")
  })

  test("pads the name so the price ends at the given width", () => {
    const aligned = withModelCost("Claude Sonnet", paid, { padTo: 24 })
    expect(aligned).toBe("Claude Sonnet   · $3/$15")
    expect(Bun.stringWidth(aligned)).toBe(24)
  })

  test("truncates a long name instead of pushing the price out", () => {
    const aligned = withModelCost("A very long model name indeed", paid, { padTo: 24 })
    expect(aligned).toBe("A very long mo… · $3/$15")
    expect(Bun.stringWidth(aligned)).toBe(24)
  })

  test("aligns wide and emoji names by terminal cells", () => {
    const aligned = withModelCost("模型😀", paid, { padTo: 24 })
    expect(Bun.stringWidth(aligned)).toBe(24)
    expect(aligned).toContain("模型😀")
  })

  test("truncates wide names without splitting a grapheme", () => {
    const aligned = withModelCost("😀😀😀😀😀😀😀😀", paid, { padTo: 24 })
    expect(Bun.stringWidth(aligned)).toBe(24)
    expect(aligned).toContain("…")
    expect(aligned).not.toContain("�")
  })

  test("pads with an empty separator, as this repo configures it", () => {
    const aligned = withModelCost("Claude Sonnet", paid, { separator: "", padTo: 24 })
    expect(aligned).toBe("Claude Sonnet     $3/$15")
    expect(Bun.stringWidth(aligned)).toBe(24)
  })

  test("accepts the maximum padding width", () => {
    expect(Bun.stringWidth(withModelCost("Claude Sonnet", paid, { padTo: MAX_PAD_TO }))).toBe(MAX_PAD_TO)
  })

  test("does not pad when the width is already used by the price", () => {
    expect(withModelCost("Model", paid, { padTo: 5 })).toBe("Model · $3/$15")
  })

  test("does not pad a name that already carries the price", () => {
    expect(withModelCost("Claude Sonnet   · $3/$15", paid, { padTo: 24 })).toBe("Claude Sonnet   · $3/$15")
  })

  test("ignores invalid padding widths", () => {
    for (const padTo of [Number.NaN, Number.POSITIVE_INFINITY, -1, 24.5, MAX_PAD_TO + 1]) {
      expect(withModelCost("Claude Sonnet", paid, { padTo })).toBe("Claude Sonnet · $3/$15")
    }
  })
})

describe("approximateWidth", () => {
  test("matches Bun.stringWidth outside Bun", () => {
    for (const value of ["Claude Sonnet · $3/$15", "模型😀", "😀😀", "e\u0301", "…", "👨‍👩‍👧", "ｆｕｌｌ"]) {
      expect(approximateWidth(value)).toBe(Bun.stringWidth(value))
    }
  })
})
