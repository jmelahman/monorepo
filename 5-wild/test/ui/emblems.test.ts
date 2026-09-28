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
  CATEGORY_EMBLEMS,
  CONSUMABLE_EMBLEMS,
  ETCHING_EMBLEMS,
  MOD_EMBLEMS,
  PACK_EMBLEMS,
  RANGE_EMBLEMS,
  RELIC_EMBLEMS,
} from "../../src/ui/emblems"

/**
 * That every card the shelf sells has a drawing, and that each drawing
 * is a drawing.
 *
 * The ids come off the engine's own tables, as `catalogs.test.ts` takes them, so
 * a relic added to the game fails here rather than reaching the shelf as the
 * kind's gem, which is what `cardHead` falls back to and which looks like
 * nothing is wrong at all. Nothing here renders: the tests have no DOM, so the
 * drawings are checked as the data they are, and looked at in a browser.
 */
const TABLES = {
  relic: [RELICS.map((entry) => entry.id), RELIC_EMBLEMS],
  consumable: [CONSUMABLES.map((entry) => entry.id), CONSUMABLE_EMBLEMS],
  pack: [PACKS.map((entry) => entry.id), PACK_EMBLEMS],
  etching: [ETCHINGS.map((entry) => entry.id), ETCHING_EMBLEMS],
  modifier: [MODIFIERS.map((entry) => entry.id), MOD_EMBLEMS],
  category: [CATEGORIES.map((entry) => entry.id), CATEGORY_EMBLEMS],
  range: [RANGES.map((entry) => entry.id), RANGE_EMBLEMS],
} as const

describe.each(Object.entries(TABLES))("the %s emblems", (_, [ids, emblems]) => {
  it("draw every card the engine has, and no card it does not", () => {
    expect(Object.keys(emblems).sort()).toEqual([...ids].sort())
  })

  it("are path data, and stay inside the 24-unit box", () => {
    for (const [id, paths] of Object.entries(emblems) as [string, readonly string[]][]) {
      expect(paths.length, id).toBeGreaterThan(0)
      for (const d of paths) {
        expect(d, id).toMatch(/^[MmLlHhVvCcSsQqTtAaZz0-9.,\s-]+$/)
        // Only the absolute moves and lines are checked, since those are the
        // coordinates a slip of the pen lands off the edge with; relative and
        // curve commands would need a path parser to place.
        for (const [, x, y] of d.matchAll(/[ML]\s*(-?[\d.]+)[\s,]+(-?[\d.]+)/g)) {
          expect(Number(x), `${id}: ${d}`).toBeGreaterThanOrEqual(0)
          expect(Number(x), `${id}: ${d}`).toBeLessThanOrEqual(24)
          expect(Number(y), `${id}: ${d}`).toBeGreaterThanOrEqual(0)
          expect(Number(y), `${id}: ${d}`).toBeLessThanOrEqual(24)
        }
      }
    }
  })
})
