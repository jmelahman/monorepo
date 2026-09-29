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
  C,
  CATEGORY_SPRITES,
  CONSUMABLE_SPRITES,
  ETCHING_SPRITES,
  MOD_SPRITES,
  PACK_SPRITES,
  RANGE_SPRITES,
  RELIC_SPRITES,
  type Sprite,
  spritePaths,
} from "../../src/ui/sprites"

/**
 * That every card the shelf sells has a sprite, and that each sprite is a
 * well-formed 12x12 grid drawn from the base palette.
 *
 * Ids come off the engine's own tables, as `emblems.test.ts` takes them. Nothing
 * renders: the grids are checked as the data they are, and looked at in a
 * browser.
 */
const TABLES = {
  relic: [RELICS.map((entry) => entry.id), RELIC_SPRITES],
  consumable: [CONSUMABLES.map((entry) => entry.id), CONSUMABLE_SPRITES],
  pack: [PACKS.map((entry) => entry.id), PACK_SPRITES],
  etching: [ETCHINGS.map((entry) => entry.id), ETCHING_SPRITES],
  modifier: [MODIFIERS.map((entry) => entry.id), MOD_SPRITES],
  category: [CATEGORIES.map((entry) => entry.id), CATEGORY_SPRITES],
  range: [RANGES.map((entry) => entry.id), RANGE_SPRITES],
} as const

const BASE = new Set<string>(Object.values(C))

describe.each(Object.entries(TABLES))("the %s sprites", (_, [ids, sprites]) => {
  const entries = Object.entries(sprites) as [string, Sprite][]

  it("draw every card the engine has, and no card it does not", () => {
    expect(Object.keys(sprites).sort()).toEqual([...ids].sort())
  })

  it("are twelve rows of twelve", () => {
    for (const [id, { rows }] of entries) {
      expect(rows.length, id).toBe(12)
      for (const row of rows) expect(row.length, `${id}: ${row}`).toBe(12)
    }
  })

  it("use only characters their palette names", () => {
    for (const [id, { rows, palette }] of entries) {
      for (const ch of rows.join("")) {
        if (ch === "." || ch === "o") continue
        expect(Object.keys(palette), `${id}: ${ch}`).toContain(ch)
      }
    }
  })

  it("name no colour they do not use, and no more than five", () => {
    for (const [id, { rows, palette }] of entries) {
      const drawn = rows.join("")
      for (const key of Object.keys(palette)) expect(drawn, `${id}: ${key}`).toContain(key)
      expect(Object.keys(palette).length, id).toBeLessThanOrEqual(5)
    }
  })

  it("take their colours from the base palette", () => {
    for (const [id, { palette }] of entries)
      for (const hex of Object.values(palette)) {
        expect(hex, id).toMatch(/^#[0-9a-f]{6}$/)
        expect(BASE.has(hex), `${id}: ${hex}`).toBe(true)
      }
  })

  it("leave no pixel alone", () => {
    // Diagonal counts as touching: a spark off a flame's tip is drawn that way on
    // purpose, and it is a stray only when nothing at all is beside it.
    for (const [id, { rows }] of entries) {
      const at = (x: number, y: number) => rows[y]?.[x] ?? "."
      for (let y = 0; y < 12; y++)
        for (let x = 0; x < 12; x++) {
          if (at(x, y) === "." || at(x, y) === "o") continue
          let kin = 0
          for (let dy = -1; dy <= 1; dy++)
            for (let dx = -1; dx <= 1; dx++) if ((dx || dy) && at(x + dx, y + dy) !== ".") kin++
          expect(kin, `${id}: ${x},${y}`).toBeGreaterThan(0)
        }
    }
  })
})

describe("spritePaths", () => {
  it("draws the outline first and merges a horizontal run into one h", () => {
    const rows = Array.from({ length: 12 }, () => ".".repeat(12))
    rows[0] = "ooAAA.......".replace("A", "A")
    rows[1] = ".....B......"
    const sprite: Sprite = { palette: { A: "#58c9b3", B: "#ff5a64" }, rows }
    expect(spritePaths(sprite)).toEqual([
      { fill: "#06070a", d: "M0 0h2v1h-2z" },
      { fill: "#58c9b3", d: "M2 0h3v1h-3z" },
      { fill: "#ff5a64", d: "M5 1h1v1h-1z" },
    ])
  })

  it("leaves out a colour that is not drawn", () => {
    const rows = Array.from({ length: 12 }, () => ".".repeat(12))
    rows[3] = "..Z........."
    expect(spritePaths({ palette: { Z: "#58c9b3", Q: "#ff5a64" }, rows })).toEqual([
      { fill: "#58c9b3", d: "M2 3h1v1h-1z" },
    ])
  })
})
