import { describe, expect, it } from "vitest"
import type { ModId, RunState, WordSource } from "../../src/engine"
import { reduce, startRun } from "../../src/engine"
import { tileTip } from "../../src/ui/views"

/**
 * What a played tile says when it is asked how it was scored.
 *
 * Pure text, so it is testable here rather than in a browser, which is the
 * reason `tileTip` is a function returning a string instead of markup built
 * inline in `grid`. The rows are played through the real engine, because the
 * whole point of the tip is that it reads back numbers nobody wrote by hand.
 */

const words: WordSource = {
  answers: ["braid"],
  allowed: new Set(["braid", "crane", "quazy", "dairy", "arose"]),
}

const start = (): RunState => startRun(1, words).state

const play = (state: RunState, word: string): RunState => {
  let current = state
  for (const letter of word) current = reduce(current, { type: "type_letter", letter }, words).state
  return reduce(current, { type: "submit" }, words).state
}

const withMod = (state: RunState, letter: string, mod: ModId): RunState => {
  const entry = state.letters[letter]
  if (!entry) throw new Error(`no such letter: ${letter}`)
  return { ...state, letters: { ...state.letters, [letter]: { ...entry, mod } } }
}

const underBoss = (state: RunState, bossId: string): RunState => ({
  ...state,
  round: { ...state.round, bossId },
})

/** The tip on one tile of one row, split back into the lines it is built from. */
function lines(state: RunState, row: number, column: number): string[] {
  const guess = state.round.guesses[row]
  if (!guess) throw new Error(`no row ${row}`)
  const tip = tileTip(state, guess, column)
  if (tip === undefined) throw new Error(`no tip on row ${row}, column ${column}`)
  return tip.split("\n")
}

describe("what a played tile says about how it scored", () => {
  /*
   * The last line is the letter's share of the row rather than the row's totals,
   * which read identically on all five tiles and so said nothing a per-tile panel
   * is for. Both totals are still in it, as the right-hand halves.
   */
  it("prices the letter, its color, and its share of the guess", () => {
    // QUAZY against BRAID: 26 chips × 4 mult, and a lone green on the A.
    const state = play(start(), "quazy")
    expect(lines(state, 0, 0)).toEqual([
      "Q · +10 chips",
      "gray · no mult",
      "10 of 26 chips · no mult",
    ])
    // The green: a single chip of the 26 and three of the four mult, which is the
    // reading a share of the score would have lost: 1 × 4 of 104 makes the tile
    // that quadrupled the row look like the cheapest thing in it.
    expect(lines(state, 0, 2)).toEqual([
      "A · +1 chip",
      "green · +3 mult",
      "1 of 26 chips · 3 of 4 mult",
    ])
  })

  it("breaks the chips down once a letter has been bought up", () => {
    const etched = start()
    const q = etched.letters.q
    if (!q) throw new Error("no Q")
    const state = play({ ...etched, letters: { ...etched.letters, q: { ...q, etch: 2 } } }, "quazy")
    expect(lines(state, 0, 0)).toEqual([
      "Q · +12 chips",
      "10 base +2 etched",
      "gray · no mult",
      "12 of 28 chips · no mult",
    ])
  })

  /*
   * The etching is still on the letter and the round still refused to pay for
   * it, so the tip has to say both. A breakdown that quietly dropped to "10
   * base" would read as the purchase having been lost rather than taxed.
   */
  it("names the boss that moved the number, and only then", () => {
    const base = start()
    const q = base.letters.q
    if (!q) throw new Error("no Q")
    const rusted = underBoss(
      { ...base, letters: { ...base.letters, q: { ...q, etch: 2 } } },
      "rust",
    )
    const state = play(rusted, "quazy")
    expect(lines(state, 0, 0)).toEqual([
      "Q · +10 chips",
      "10 base +2 etched",
      "The Rust: Letter upgrades score nothing. Letters are worth only what they started as.",
      "gray · no mult",
      "10 of 26 chips · no mult",
    ])
    // U is worth what it always was, so The Rust is not named on it: a boss line
    // under every letter would make the whole row look cursed.
    expect(lines(state, 0, 1).some((line) => line.startsWith("The Rust"))).toBe(false)
  })

  /*
   * The one line here that gives something away, and it does so on purpose.
   *
   * Under The Fog a misplaced letter reads gray and pays its mult anyway, and
   * the tip says "yellow": the color that scored, not the color on the tile.
   * The first pass had it the other way, quoting `shown` so as not to undo the
   * boss for the price of a hover. What changed the call is that the round was
   * already telling: the readout tracks the true running mult as each tile
   * lands, so a gray that moves the multiplier has announced itself to anyone
   * watching that corner of the screen. The panel does not create the leak, it
   * takes it away from the attentive and gives it to whoever asks, and The Fog,
   * which attacks deduction itself, is a hard enough round played straight.
   *
   * Bloodhound is here because it is the sharpest form of the same thing: it
   * pays for yellows, so naming it on a tile drawn gray is the leak in one line.
   */
  it("tells a fogged gray what color it really scored as", () => {
    const state = play(underBoss({ ...start(), relics: [{ id: "bloodhound" }] }, "fog"), "dairy")
    expect(state.round.guesses[0]?.tiles[0]).toMatchObject({ color: "yellow", shown: "gray" })
    expect(lines(state, 0, 0)).toEqual([
      "D · +2 chips",
      "yellow · +1 mult",
      "Bloodhound · +6",
      "8 of 33 chips · 1 of 5 mult",
    ])
  })

  it("quotes the modifier in the words it used at the time", () => {
    const state = play(withMod(start(), "c", "steel"), "crane")
    expect(lines(state, 0, 0)).toContain("Steel · ×2 mult")
  })

  /*
   * Three silences that look identical on the board and mean different things:
   * the letter carries nothing, the letter carries something that wanted a
   * color it did not get, and the boss switched the layer off. Only the first
   * is worth saying nothing about.
   */
  it("tells a card that declined apart from one that was silenced", () => {
    const declined = play(withMod(start(), "c", "anchor"), "crane")
    expect(lines(declined, 0, 0)).toContain(
      "Anchor · scores +125 chips when it lands green · nothing this time",
    )

    const silenced = play(underBoss(withMod(start(), "c", "steel"), "vandal"), "crane")
    expect(lines(silenced, 0, 0)).toContain("Steel · scores ×2 mult · silenced this round")

    expect(lines(play(start(), "crane"), 0, 0)).toEqual([
      "C · +3 chips",
      "gray · no mult",
      "3 of 7 chips · no mult",
    ])
  })

  /*
   * The question the first draft of this tip could not answer: the tray is half
   * the scoring and a tile that came out worth 9 with a base of 1 needs to say
   * which card did it. One line per firing, in slot order, in the words the card
   * used as it lit up.
   */
  it("names the relics that paid on the tile", () => {
    const state = play({ ...start(), relics: [{ id: "green_thumb" }] }, "quazy")
    expect(lines(state, 0, 2)).toEqual([
      "A · +1 chip",
      "green · +3 mult",
      "Green Thumb · +8",
      // The relic's 8 counts toward the column that earned it, so the share adds
      // up the lines above it rather than repeating the letter's own chips.
      "9 of 34 chips · 3 of 4 mult",
    ])
    // And nothing on the four tiles it did not want.
    expect(lines(state, 0, 0).some((line) => line.startsWith("Green Thumb"))).toBe(false)
  })

  /*
   * Silence for a relic that declined, where the modifier above gets a sentence.
   * Five tray slots asked about every tile means five dead lines under each of
   * them, and a panel that is mostly "nothing this time" is a panel nobody reads
   * to the end.
   */
  it("says nothing about a relic that was asked and did nothing", () => {
    const state = play({ ...start(), relics: [{ id: "bloodhound" }] }, "quazy")
    expect(lines(state, 0, 0)).toEqual([
      "Q · +10 chips",
      "gray · no mult",
      "10 of 26 chips · no mult",
    ])
  })

  /*
   * A row played before guesses kept their arithmetic: a save carried across
   * the change, and only until the round ends. No tip at all beats a plausible
   * reconstruction, because the reconstruction is wrong exactly under the two
   * bosses somebody would most want to ask about.
   */
  it("says nothing about a row that did not write its arithmetic down", () => {
    const state = play(start(), "quazy")
    const guess = state.round.guesses[0]
    if (!guess) throw new Error("no guess")
    // The key missing rather than set to undefined, which is what an old save
    // actually reads back as, and what `exactOptionalPropertyTypes` is for.
    const { paid, ...older } = guess
    expect(paid).toBeDefined()
    expect(tileTip(state, older, 0)).toBeUndefined()
  })
})
