import { describe, expect, it } from "vitest"
import type { ModId, RunState, WordSource } from "../../src/engine"
import { MODIFIER_BY_ID, startRun } from "../../src/engine"
import { displacedAt } from "../../src/ui/views"

const words: WordSource = { answers: ["braid"], allowed: new Set(["braid", "crane"]) }

const fresh = (): RunState => startRun(1, words).state

const withLetter = (
  letter: string,
  patch: { mod?: ModId | null; destroyed?: boolean },
): RunState => {
  const state = fresh()
  const entry = state.letters[letter]
  if (!entry) throw new Error(`no such letter: ${letter}`)
  return { ...state, letters: { ...state.letters, [letter]: { ...entry, ...patch } } }
}

const mod = (id: ModId) => {
  const found = MODIFIER_BY_ID.get(id)
  if (!found) throw new Error(`no such modifier: ${id}`)
  return found
}

/**
 * The picker asks before it destroys, and this is the question it asks. A wrong
 * answer here has two shapes and they land on opposite players.
 *
 * Answer "something" about a bare letter and the tap that should have placed the
 * modifier arms instead, drawing a screen identical to the one already showing,
 * because the sheet, told to warn about nothing, correctly warns about nothing.
 * That is the bug this file was written for: `mod` on an empty letter is `null`,
 * the caller tested it against `undefined`, and every letter in the alphabet
 * read as occupied. It reached a player as a dead first tap.
 *
 * Answer "nothing" about an occupied one and the confirmation never appears at
 * all, which is the failure the confirmation exists to prevent and is silent
 * until someone loses a $12 Anchor to a thumb.
 */
describe("what a placement would destroy", () => {
  it("says nothing about a letter with no modifier on it", () => {
    // `null`, not `undefined`: the whole alphabet on the first visit, and the
    // exact value the old test read as occupied.
    expect(fresh().letters.e?.mod).toBe(null)
    expect(displacedAt(fresh(), mod("glass"), "e")).toBeUndefined()
  })

  it("names the modifier being displaced when there is one", () => {
    const state = withLetter("e", { mod: "steel" })
    expect(displacedAt(state, mod("glass"), "e")?.id).toBe("steel")
  })

  it("says nothing about the letters the engine is going to refuse anyway", () => {
    // A refusal is not a trade to confirm. Arming one would put a question about
    // losing Steel in front of a placement that was never going to happen, and
    // the second tap would answer it into the same refusal as the first.

    // Echo goes on `aelost` only, so the O is a trade and the R is not, however
    // much the R is carrying.
    const echoes = withLetter("r", { mod: "steel" })
    expect(displacedAt(echoes, mod("echo"), "r")).toBeUndefined()
    expect(displacedAt(withLetter("o", { mod: "steel" }), mod("echo"), "o")?.id).toBe("steel")

    // A broken letter takes nothing, whatever it was holding when it broke.
    const broken = withLetter("e", { mod: "steel", destroyed: true })
    expect(displacedAt(broken, mod("glass"), "e")).toBeUndefined()

    // The modifier already on the letter. `placeableLetters` keeps this off the
    // shelf, so the card should not exist, but asked, the honest answer is that
    // nothing is lost, not "replaces Steel" about the Steel that is staying.
    const steeled = withLetter("e", { mod: "steel" })
    expect(displacedAt(steeled, mod("steel"), "e")).toBeUndefined()
  })
})
