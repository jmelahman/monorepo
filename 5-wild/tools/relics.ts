/**
 * What each relic is worth, measured the same way every time.
 *
 *   bun run relics [--seeds 250] [--policy solver|farmer] [--only keystone,twins]
 *
 * Every figure quoted in a comment in `relics.ts` came from a harness that was
 * written, run and deleted, and no two of them measured the same thing: "×2.04
 * on an empty tray", "×1.74 on a late kit", "×1.95 over 250 seeds" are three
 * different questions, so the comments could not be compared with each other
 * and none of them could be checked again. Re-measured with this, Keystone's
 * "×2.04" read ×1.81, and the old harness's "×1.40" for its ×2 read ×1.54. This fixes the method so
 * the next comment quotes the same one.
 *
 * The method is a counterfactual on the blind bot's own games. The bot plays
 * each seed as it would, with whatever tray its shopping list bought, and at
 * every guess the same row is scored twice through the engine's own
 * `scoreGuess`: once with the tray it held, less the card being measured if it
 * held it, and once with the card added in the last slot. Each round is summed
 * under both trays, and a solved one is multiplied by each tray's own solve
 * bonus, the way `reduce` does it. The figure is the ratio of those two round
 * totals, so it is "how much more does this player bank a round with the
 * card", split by stage because a flat +mult and a ×mult tell opposite stories
 * across a run.
 *
 * Rounds rather than guesses because the solve bonus is a round's: it
 * multiplies the pile, not the guess that solved it, so a per-guess figure
 * could not see Long Game at all and undercounted every card that pays on the
 * guesses before the last.
 *
 * Three things it is not, which are worth saying before a number from it is
 * quoted:
 *
 *   - A win rate. It prices guesses the bot already made; it does not replay
 *     the run with the card, so a card that would have bought a round is still
 *     just a bigger score here.
 *   - A player. The bot never steers for a card, so a card whose condition only
 *     fires when chased (Keystone's middle green, Twins' repeat) reads as its
 *     floor, and one whose condition fires by accident (Anagrammer's five
 *     distinct letters) reads as close to its ceiling. `fires` says which.
 *   - A grown card. Every card is added fresh, so the growers (marked `*`) read
 *     as bought that guess, which is their worst case.
 *
 * The last slot, although slot 0 is where a flat card does the most, ahead of
 * every ×mult in the tray. It was slot 0 first, and every card in the tray
 * moved one slot right, and a relic's roll is keyed by its slot
 * (`scoring.ts`), so Loaded Dice re-rolled under every measurement: cards that
 * cannot touch a guess read ×0.99 and "fired" on a fifth of them. At the end,
 * nothing held moves. The cost is that a flat card reads a little low against
 * a tray already holding a ×mult. Guesses taken under The Plateau are in the
 * sums like any other, since a player buying a ×mult card buys that too.
 *
 * A card that pays only in gold still reads a hair under ×1.00, and that is
 * not noise: it takes a slot, and Blank Page and Collector in the tray count
 * slots. With both removed from the bot's tray, Scavenger reads ×1.00 flat.
 *
 * Only cards that touch a guess are listed: an `onTile`, `onGuess` or
 * `solveBonus`. Stipend, Second Look and the rest are worth gold or rerolls,
 * which this cannot see, and would read ×1.00 as if they did nothing.
 */

import type { Relic, RelicInstance, RunState } from "../src/engine"
import { CONTENT_VERSION, RELICS, reduce, startRun } from "../src/engine"
import { scoreGuess } from "../src/engine/scoring"
import type { Policy } from "../test/helpers/blind"
import { blindPlayer } from "../test/helpers/blind"
import { realWords as words } from "../test/helpers/words"

const args = process.argv.slice(2)
const flag = (name: string): string | undefined => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}

const SEEDS = Number(flag("--seeds")) || 250
const policy: Policy = flag("--policy") === "farmer" ? "farmer" : "solver"
const only = flag("--only")?.split(",")

/** The same cap `sim.test.ts` uses: a run cannot need more unless it loops. */
const CAP = 6000

/**
 * Three bands rather than eight stages. Most runs end by stage four, so a
 * column per stage would print the late ones off a few dozen guesses; three is
 * enough to show a flat card fading and a ×mult holding.
 */
const BANDS = [
  { label: "1-2", upTo: 2 },
  { label: "3-5", upTo: 5 },
  { label: "6+", upTo: Number.POSITIVE_INFINITY },
] as const

const measured = RELICS.filter(
  (relic) =>
    (relic.onTile || relic.onGuess || relic.solveBonus) && (!only || only.includes(relic.id)),
)

/** Banked round totals, and how many of the guesses in them the card moved. */
type Tally = { base: number; with: number; guesses: number; fired: number }
const blank = (): Tally => ({ base: 0, with: 0, guesses: 0, fired: 0 })
const tallies = new Map(measured.map((relic) => [relic.id, BANDS.map(blank)]))
/** The round in progress, per card, under each tray. Flushed when it ends. */
const open = new Map(measured.map((relic) => [relic.id, { base: 0, with: 0 }]))
const roundsIn = BANDS.map(() => 0)

/** One row priced against one tray, with nothing the real guess did carried over. */
function price(
  state: RunState,
  relics: RelicInstance[],
  index: number,
): { score: number; solveBonus: number } {
  const guess = state.round.guesses[index]
  if (!guess) return { score: 0, solveBonus: 1 }
  const before: RunState = {
    ...state,
    relics,
    round: { ...state.round, guesses: state.round.guesses.slice(0, index) },
  }
  return scoreGuess({
    state: before,
    // Cloned, because a boss or a modifier may write to a tile while scoring.
    tiles: guess.tiles.map((tile) => ({ ...tile })),
    word: guess.word,
    guessIndex: index,
    guessesLeft: state.round.maxGuesses - index - 1,
    solved: guess.word === state.round.answer,
    events: [],
  })
}

function record(before: RunState, after: RunState): void {
  const index = before.round.guesses.length
  // `after`'s round, since that is the one holding the guess, but `before`'s
  // relics: a card sold or grown by this guess's own round end is not what
  // scored it.
  const played: RunState = { ...after, relics: before.relics, letters: before.letters }
  const band = BANDS.findIndex((b) => before.stage <= b.upTo)
  const solved = after.round.guesses[index]?.word === after.round.answer
  const over = solved || index + 1 >= after.round.maxGuesses
  if (over) roundsIn[band] = (roundsIn[band] ?? 0) + 1
  for (const relic of measured) {
    const tray = before.relics.filter((held) => held.id !== relic.id)
    const base = price(played, tray, index)
    const withCard = price(played, [...tray, { id: relic.id }], index)
    const tally = tallies.get(relic.id)?.[band]
    const pile = open.get(relic.id)
    if (!tally || !pile) continue
    tally.guesses++
    if (withCard.score !== base.score || withCard.solveBonus !== base.solveBonus) tally.fired++
    pile.base += base.score
    pile.with += withCard.score
    if (!over) continue
    // Rounded the way `reduce` rounds the real pile.
    tally.base += solved ? Math.round(pile.base * base.solveBonus) : pile.base
    tally.with += solved ? Math.round(pile.with * withCard.solveBonus) : pile.with
    pile.base = 0
    pile.with = 0
  }
}

for (let seed = 1; seed <= SEEDS; seed++) {
  const player = blindPlayer(policy)
  let state = startRun(seed, words, 0).state
  for (let step = 0; step < CAP; step++) {
    if (state.phase === "game_over" || state.phase === "victory") break
    const batch = player.next(state, words)
    if (!batch || batch.length === 0) break
    for (const action of batch) {
      const next = reduce(state, action, words).state
      // Read off the round the guess landed in, before a cleared round swaps it
      // out: a winning guess moves the phase on in the same reduce.
      if (state.phase === "round" && next.round.guesses.length > state.round.guesses.length) {
        record(state, next)
      }
      state = next
    }
  }
}

const RARITY = ["common", "uncommon", "rare", "legendary"]
const rank = (relic: Relic) => RARITY.indexOf(relic.rarity)
const ratio = (tally: Tally | undefined) =>
  tally && tally.base > 0 ? `×${(tally.with / tally.base).toFixed(2)}` : "-"
const sum = (list: Tally[]): Tally =>
  list.reduce(
    (total, t) => ({
      base: total.base + t.base,
      with: total.with + t.with,
      guesses: total.guesses + t.guesses,
      fired: total.fired + t.fired,
    }),
    blank(),
  )
const overall = (relic: Relic) => {
  const all = sum(tallies.get(relic.id) ?? [])
  return all.base > 0 ? all.with / all.base : 0
}

const rows = [...measured].sort((a, b) => rank(a) - rank(b) || overall(b) - overall(a))
const head = ["relic", "rarity", "fires", ...BANDS.map((b) => b.label), "all"]
const body = rows.map((relic) => {
  const bands = tallies.get(relic.id) ?? []
  const all = sum(bands)
  return [
    relic.id + (relic.growth ? " *" : ""),
    relic.rarity,
    `${((100 * all.fired) / Math.max(1, all.guesses)).toFixed(0)}%`,
    ...bands.map(ratio),
    ratio(all),
  ]
})
const widths = head.map((_, col) =>
  Math.max(...[head, ...body].map((row) => row[col]?.length ?? 0)),
)
const line = (row: string[]) =>
  row
    .map((cell, col) =>
      col === 0 ? cell.padEnd(widths[col] ?? 0) : cell.padStart(widths[col] ?? 0),
    )
    .join("  ")

console.log(
  `content v${CONTENT_VERSION}, ${policy}, ${SEEDS} seeds, ascension 0; ` +
    `rounds by stage ${BANDS.map((b, i) => `${b.label}: ${roundsIn[i]}`).join(", ")}`,
)
console.log(line(head))
for (const row of body) console.log(line(row))
console.log("* grows: measured fresh, so this is its floor")
