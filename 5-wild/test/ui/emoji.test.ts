import { readdirSync, readFileSync } from "node:fs"
import { describe, expect, it } from "vitest"
import {
  CATEGORIES,
  CONSUMABLES,
  ETCHINGS,
  MODIFIERS,
  PACKS,
  RANGES,
  RELICS,
} from "../../src/engine"
import {
  CATEGORY_EMOJI,
  CONSUMABLE_EMOJI,
  EMOJI,
  EMOJI_ART,
  ETCHING_EMOJI,
  MOD_EMOJI,
  PACK_EMOJI,
  RANGE_EMOJI,
  RELIC_EMOJI,
} from "../../src/ui/emoji"

/**
 * That every card the shelf sells has an emoji, that the emoji is a file, and
 * that the file is one a picture can be made from.
 *
 * Ids come off the engine's tables, as `emblems.test.ts` takes them, so a relic
 * added to the game fails here rather than reaching Classic as its line emblem
 * on a shelf of emoji. Nothing renders: the files are checked as the text they
 * are, and looked at in a browser.
 */
const DIR = "src/ui/emoji"
const TABLES: [string, Record<string, string>, readonly { id: string }[]][] = [
  ["relic", RELIC_EMOJI, RELICS],
  ["consumable", CONSUMABLE_EMOJI, CONSUMABLES],
  ["pack", PACK_EMOJI, PACKS],
  ["etching", ETCHING_EMOJI, ETCHINGS],
  ["modifier", MOD_EMOJI, MODIFIERS],
  ["category", CATEGORY_EMOJI, CATEGORIES],
  ["range", RANGE_EMOJI, RANGES],
]
const used = TABLES.flatMap(([, table]) => Object.values(table))
const files = readdirSync(DIR)
  .filter((name) => name.endsWith(".svg"))
  .map((name) => name.replace(/\.svg$/, ""))

describe("the emoji", () => {
  it.each(TABLES)("picture every %s the engine has, and none it does not", (_, table, source) => {
    expect(Object.keys(table).sort()).toEqual(source.map((entry) => entry.id).sort())
  })

  it("are every table the views can reach", () => {
    expect(Object.values(EMOJI)).toEqual(TABLES.map(([, table]) => table))
  })

  it("are each a file, and no file is left over", () => {
    // A leftover is 1 KB shipped to every player for a picture nobody sees.
    expect([...new Set(used)].sort()).toEqual([...files].sort())
    expect(Object.keys(EMOJI_ART).sort()).toEqual([...files].sort())
  })

  // Two cards with one picture are one card, on a shelf read by its pictures.
  it("are not shared between two cards", () => {
    expect(new Set(used).size).toBe(used.length)
  })

  it("ship with the licence they came under", () => {
    expect(readFileSync(`${DIR}/LICENSE`, "utf8")).toMatch(
      /MIT License[\s\S]*Microsoft Corporation/,
    )
  })
})

describe("the emoji files", () => {
  it.each(files)("%s is one flat drawing in the set's box", (name) => {
    const file = readFileSync(`${DIR}/${name}.svg`, "utf8")
    expect(file).toMatch(/^<svg xmlns="http:\/\/www\.w3\.org\/2000\/svg" viewBox="0 0 32 32">/)
    // The outer tag is stripped and the rest set inside the emblem, so the
    // rest must be something to set.
    expect(EMOJI_ART[name]).toMatch(/^<(g|path)\b[\s\S]+>$/)
    expect(EMOJI_ART[name]).not.toMatch(/<svg|<script|<style|<image|<use|href=/)
    // Flat: an `id` is a gradient or a clip, and one id on two cards of the
    // same emoji is a duplicate in the document.
    expect(file).not.toMatch(/\bid=|url\(/)
  })
})
