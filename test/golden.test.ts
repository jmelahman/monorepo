/**
 * Golden vectors: recorded runs, replayed, asserted to the last chip.
 *
 * Their day job is catching a balance edit that quietly changes a score
 * somewhere nobody was looking. Their second job is the portability contract:
 * if these rules are ever reimplemented, that implementation is correct exactly
 * when it reproduces this file.
 *
 * To accept a deliberate balance change: bump CONTENT_VERSION, run
 * `npm run golden`, and read the diff. The diff is the change, priced in
 * points.
 */

import { readFileSync, writeFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import { describe, expect, it } from "vitest"
import { CONTENT_VERSION } from "../src/engine"
import { SCENARIOS } from "./golden/scenarios"
import type { Vector } from "./golden/vectors"
import { record, replay } from "./golden/vectors"
import { realWords } from "./helpers/words"

const FILE = join(dirname(fileURLToPath(import.meta.url)), "golden", "vectors.json")

if (process.env.UPDATE_GOLDEN) {
  const vectors = SCENARIOS.map((scenario) => record(scenario, realWords, CONTENT_VERSION))
  writeFileSync(FILE, `${JSON.stringify(vectors, null, 2)}\n`)
}

const vectors: Vector[] = JSON.parse(readFileSync(FILE, "utf8"))

describe("golden vectors", () => {
  it("has one recorded", () => {
    expect(vectors.length).toBeGreaterThan(0)
  })

  /*
   * A vector recorded against different numbers is not a failure to fix, it is
   * a baseline to re-record, and the two need telling apart before anyone
   * reads a hundred changed scores as a hundred bugs.
   */
  it("was recorded at the current content version", () => {
    for (const vector of vectors) {
      expect(vector.contentVersion, `${vector.name}: re-record with \`npm run golden\``).toBe(
        CONTENT_VERSION,
      )
    }
  })

  for (const vector of vectors) {
    const rerun = () => replay(vector.seed, vector.actions, realWords, vector.ascension)
    describe(`${vector.name}: ${vector.covers}`, () => {
      it("replays to the same scores", () => {
        expect(rerun().expected.guesses).toEqual(vector.expected.guesses)
      })

      it("replays to the same gold timeline", () => {
        expect(rerun().expected.gold).toEqual(vector.expected.gold)
      })

      it("replays to the same outcome and holdings", () => {
        const { guesses: _g, gold: _gold, ...rest } = rerun().expected
        const { guesses: _eg, gold: _egold, ...expected } = vector.expected
        expect(rest).toEqual(expected)
      })

      /*
       * Every action here was legal when it was recorded, so this asserts a
       * property rather than a number, and that is the point of asserting it
       * separately instead of adding an always-empty list to `expected`. A
       * re-record would write the new empty list as happily as the old one; a
       * rule that quietly started refusing something would then reach the diff
       * as nothing but a shorter run and a pile of moved scores.
       *
       * `reduce` returns the state untouched on a refusal, so without this the
       * replay just plays on with one fewer action and the failure surfaces
       * several rounds later wearing someone else's clothes.
       */
      it("replays without a refused action", () => {
        const refused = rerun().refused
        const named = refused.map((r) => `#${r.index} ${r.action.type}: ${r.refusal.code}`)
        expect(named).toEqual([])
      })
    })
  }

  /*
   * Recording is only trustworthy if it is repeatable. This catches the class
   * of bug the vectors themselves cannot: a scenario, or the engine, reading
   * something outside the seed.
   */
  it("records the same run twice", () => {
    for (const scenario of SCENARIOS) {
      const once = record(scenario, realWords, CONTENT_VERSION)
      const twice = record(scenario, realWords, CONTENT_VERSION)
      expect(once).toEqual(twice)
    }
  })

  /*
   * A vector that never scored a guess asserts nothing, and would go on
   * asserting nothing after a change that broke every score in the game.
   */
  it("covers something in every vector", () => {
    for (const vector of vectors) {
      expect(vector.expected.guesses.length, vector.name).toBeGreaterThan(0)
    }
  })
})
