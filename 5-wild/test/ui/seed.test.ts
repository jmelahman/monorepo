import { describe, expect, it } from "vitest"
import { MAX_ASCENSION, startRun } from "../../src/engine"
import { parseSeed, readPastedLink, readSeedParam, seedCode, seedLink } from "../../src/ui/seed"
import { realWords } from "../helpers/words"

describe("seed codes", () => {
  it("spell every seed as seven characters and read back to it", () => {
    const seeds = [0, 1, 31, 32, 2 ** 31 - 1, 123456789]
    // A fixed stride across the range rather than `Math.random`, so a failure
    // names the same seed on the next run.
    for (let i = 0; i < 200; i++) seeds.push((i * 10737419 + 7) % 2 ** 31)
    for (const seed of seeds) {
      const code = seedCode(seed)
      expect(code).toMatch(/^[0-9A-HJKMNP-TV-Z]{7}$/)
      expect(parseSeed(code)).toBe(seed)
    }
  })

  it("forgive the ways a person copies a code", () => {
    const code = seedCode(123456789)
    expect(parseSeed(code.toLowerCase())).toBe(123456789)
    expect(parseSeed(` ${code.slice(0, 3)}-${code.slice(3)} `)).toBe(123456789)
    // Crockford's aliases: the letters a hand confuses with digits are digits.
    expect(parseSeed("OOOOOOI")).toBe(1)
    expect(parseSeed("0000001")).toBe(1)
    expect(parseSeed("000000l")).toBe(1)
  })

  it("refuse anything that is not one", () => {
    for (const text of ["", "ABC", "12345678", "00000U0", "ABC!EFG"]) {
      expect(parseSeed(text)).toBeNull()
    }
    // Seven characters hold 35 bits, and a seed is 31.
    expect(parseSeed("2000000")).toBeNull()
    expect(parseSeed("1ZZZZZZ")).toBe(2 ** 31 - 1)
  })

  it("deal the run they were copied from", () => {
    const seed = 987654321
    const again = parseSeed(seedCode(seed))
    expect(again).not.toBeNull()
    expect(startRun(again as number, realWords, 3).state).toEqual(
      startRun(seed, realWords, 3).state,
    )
  })
})

describe("seed links", () => {
  it("leave the defaults off, and carry the rest", () => {
    const code = seedCode(42)
    expect(seedLink(42, 0, "en")).toBe(`https://5-wild.com/?seed=${code}`)
    expect(seedLink(42, 3, "fr")).toBe(`https://5-wild.com/?seed=${code}&a=3&w=fr`)
  })

  it("read back to the run they name", () => {
    const link = new URL(seedLink(42, 3, "fr"))
    expect(readSeedParam(link.search)).toEqual({ seed: 42, ascension: 3, words: "fr" })
    expect(readSeedParam(new URL(seedLink(42, 0, "en")).search)).toEqual({
      seed: 42,
      ascension: 0,
      words: "en",
    })
  })

  it("clamp a bad level and ignore an unknown list, but not a bad code", () => {
    const code = seedCode(42)
    expect(readSeedParam(`?seed=${code}&a=9999&w=xx`)).toEqual({
      seed: 42,
      ascension: MAX_ASCENSION,
      words: "en",
    })
    expect(readSeedParam(`?seed=${code}&a=-3`)?.ascension).toBe(0)
    expect(readSeedParam("?seed=nope")).toBeNull()
    expect(readSeedParam("?watch=localhost")).toBeNull()
  })
})

describe("a link pasted into the field", () => {
  // What the copy button puts on the clipboard, so what the field will most
  // often be handed; read as a code alone it would lose its level and list.
  it("keeps the terms it carries", () => {
    expect(readPastedLink(` ${seedLink(42, 3, "fr")}\n`)).toEqual({
      seed: 42,
      ascension: 3,
      words: "fr",
    })
  })

  it("is not mistaken for a typed code", () => {
    expect(readPastedLink(seedCode(42))).toBeNull()
    expect(readPastedLink("https://5-wild.com/?watch=1")).toBeNull()
  })
})
