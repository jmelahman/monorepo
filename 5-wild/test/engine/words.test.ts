import { describe, expect, it } from "vitest"
import type { Color } from "../../src/engine"
import { computeFeedback } from "../../src/engine"

const GLYPH: Record<Color, string> = { green: "G", yellow: "Y", gray: "." }

const feedback = (guess: string, answer: string) =>
  computeFeedback(guess, answer)
    .map((color) => GLYPH[color])
    .join("")

describe("computeFeedback", () => {
  it("marks exact positions green", () => {
    expect(feedback("braid", "braid")).toBe("GGGGG")
  })

  it("marks present-but-misplaced letters yellow", () => {
    // DAIRY shares four letters with BRAID and misplaces every one of them.
    expect(feedback("dairy", "braid")).toBe("YYYY.")
    expect(feedback("crane", "braid")).toBe(".GG..")
  })

  it("marks absent letters gray", () => {
    expect(feedback("skunk", "brief")).toBe(".....")
  })

  // The rule everyone reimplements wrong: greens claim their copies first, and
  // yellows only draw from whatever is left over.
  it("withholds a yellow when greens have claimed every copy", () => {
    expect(feedback("lolly", "silly")).toBe("..GGG")
    // The answer's only A is the green one, so the leading A gets nothing.
    expect(feedback("aback", "shard")).toBe("..G..")
  })

  it("pays out yellows only up to the number of copies in the answer", () => {
    // One E in MEDAL, two in GREEN: the first claims it, the second goes gray.
    expect(feedback("green", "medal")).toBe("..Y..")
  })
})
