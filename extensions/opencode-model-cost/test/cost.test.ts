import { describe, expect, test } from "bun:test"
import { formatModelCost, isFree, withModelCost } from "../src/cost"

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

  test("returns nothing for a model with no cost data", () => {
    expect(formatModelCost([])).toBeUndefined()
  })

  test("leaves free models to the picker's built-in Free label", () => {
    expect(formatModelCost([{ input: 0, output: 0 }])).toBeUndefined()
  })

  test("can label free models when asked", () => {
    expect(
      formatModelCost([{ input: 0, output: 0 }], { includeFree: true }),
    ).toBe("Free")
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
})
