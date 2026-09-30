/**
 * What a card does to *winning*, measured the same way every time.
 *
 *   bun run builds [--seeds 1000] [--policy solver|farmer|builder]
 *                  [--ban snowball,hoarder] [--ascension 0] [--jobs 16]
 *
 * `bun run relics` prices a card by what it adds to a guess the bot already
 * made. That is the right question for a flat card and the wrong one for a
 * card that grows, and wrong again for the question the build work is asking,
 * which is whether a run *without* a given card can still be won. So this plays
 * whole runs and counts wins, and prints them split by which door the run held.
 *
 * A door is a way to keep pace with a target that grows ×2.2 a stage: a card
 * or pair of cards that scales across the run. At v32 there were three, and a
 * run that held none of them won no game in 1,000 (solver, ascension 0). The
 * table is `DOORS` below; a new grower joins it the day it ships, so its share
 * of the wins is read beside the others'. Category levels are not a door here,
 * though they grow, because every run can buy them: the build table and the
 * last line read them instead.
 *
 * `--ban` takes cards off the shelf rather than out of the catalog. The shop is
 * rolled as it always is, and a banned relic is struck from the shelf or the
 * pack after the roll, the slot left empty as though bought. That keeps every
 * other slot's roll where it was, so a ban changes which card is missing and
 * nothing else: the catalog's own draw order is the vectors', and a harness that
 * shifted it would be measuring a different set of seeds. The cost is a
 * shelf one card short on the visits a banned card turned up, which is a small
 * tax on every ban and the same tax on all of them.
 *
 * Runs are split across processes, one per core by default, because the
 * question this answers is a small rate: half a percent is five wins in 1,000
 * seeds, which is inside the noise, and 5,000 is the least worth quoting for a
 * door. Seeds are 1 to `--seeds`, the same as `bun run relics`, so the two
 * harnesses read the same games.
 */

import { execFile } from "node:child_process"
import { availableParallelism } from "node:os"
import type { RunState } from "../src/engine"
import { CONTENT_VERSION, reduce, STAGES, startRun } from "../src/engine"
import type { Policy } from "../test/helpers/blind"
import { blindPlayer } from "../test/helpers/blind"
import { realWords as words } from "../test/helpers/words"

const args = process.argv.slice(2)
const flag = (name: string): string | undefined => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}

const SEEDS = Number(flag("--seeds")) || 1000
const POLICIES: readonly Policy[] = ["solver", "farmer", "builder"]
const policy: Policy = POLICIES.find((p) => p === flag("--policy")) ?? "solver"
const banned = new Set(flag("--ban")?.split(",").filter(Boolean) ?? [])
const ascension = Number(flag("--ascension")) || 0
const JOBS = Number(flag("--jobs")) || availableParallelism()
const shard = flag("--shard")

/** The same cap `sim.test.ts` uses: a run cannot need more unless it loops. */
const CAP = 6000

/**
 * The cards that scale, grouped into the ways a run can scale. A run holds a
 * door when it held every card in it at once at some point: the Pyromaniac
 * alone is +40 mult and a keyboard with holes in it, and only Scorched Earth
 * beside it turns the holes into growth.
 *
 * The first four were the whole list at v33. The rest are the growers that
 * opened a door for a build that had none: yellow, money, farming, gray, and
 * Habit for the shapes.
 */
const DOORS: Record<string, readonly string[]> = {
  snowball: ["snowball"],
  hoarder: ["hoarder"],
  hot_streak: ["hot_streak"],
  pyromaniac: ["pyromaniac", "scorched_earth"],
  bloodhound: ["bloodhound"],
  mint: ["mint"],
  slow_burn: ["slow_burn"],
  masochist: ["masochist"],
  habit: ["habit"],
}

type Outcome = {
  seed: number
  won: boolean
  /** The stage the run died on, or `STAGES` + 1 if it never did. */
  stage: number
  doors: string[]
  /** What the bot committed to, for the policy that commits. */
  build: string | null
  levels: number
}

/** Strike the banned cards from whatever the run is being offered right now. */
function strike(state: RunState): RunState {
  if (banned.size === 0) return state
  const gone = <T extends { kind: string; id: string }>(item: T | null): T | null =>
    item && item.kind === "relic" && banned.has(item.id) ? null : item
  const shop = state.shop
  const pack = state.pack
  if (!shop?.items.some((item) => item !== gone(item)) && !pack?.options.some((o) => o !== gone(o)))
    return state
  return {
    ...state,
    shop: shop && { ...shop, items: shop.items.map(gone) },
    pack: pack ? { ...pack, options: pack.options.map(gone) } : null,
  }
}

function play(seed: number): Outcome {
  const player = blindPlayer(policy)
  let state = strike(startRun(seed, words, ascension).state)
  const doors = new Set<string>()
  let round = 0
  for (let step = 0; step < CAP; step++) {
    if (state.phase === "game_over" || state.phase === "victory") break
    const batch = player.next(state, words)
    if (!batch || batch.length === 0) break
    for (const action of batch) {
      state = strike(reduce(state, action, words).state)
      const held = new Set(state.relics.map((relic) => relic.id))
      for (const [door, cards] of Object.entries(DOORS)) {
        if (cards.every((card) => held.has(card))) doors.add(door)
      }
      if (state.phase === "round") round = state.stage
    }
  }
  const won = state.phase === "victory" || Boolean(state.won)
  const levels = Object.values(state.levels ?? {}).reduce((sum, level) => sum + level - 1, 0)
  return {
    seed,
    won,
    stage: won ? STAGES + 1 : round,
    doors: [...doors],
    build: player.build?.() ?? null,
    levels,
  }
}

if (shard) {
  const [index, of] = shard.split("/").map(Number) as [number, number]
  const out: Outcome[] = []
  for (let seed = 1 + index; seed <= SEEDS; seed += of) out.push(play(seed))
  process.stdout.write(JSON.stringify(out))
  process.exit(0)
}

const started = performance.now()
const jobs = Math.min(JOBS, SEEDS)
const passed = args.filter((arg, i) => arg !== "--jobs" && args[i - 1] !== "--jobs")
const shardOf = (index: number) =>
  new Promise<Outcome[]>((resolve, reject) =>
    execFile(
      process.execPath,
      [process.argv[1] ?? "", ...passed, "--shard", `${index}/${jobs}`],
      { maxBuffer: 1 << 28 },
      (error, stdout) => (error ? reject(error) : resolve(JSON.parse(stdout))),
    ),
  )
const runs = (await Promise.all(Array.from({ length: jobs }, (_, i) => shardOf(i)))).flat()
runs.sort((a, b) => a.seed - b.seed)

const pct = (n: number, of: number) => `${((100 * n) / Math.max(1, of)).toFixed(1)}%`
const won = runs.filter((run) => run.won)
const lines = [
  `content v${CONTENT_VERSION}, ${policy}, ${runs.length} seeds, ascension ${ascension}` +
    (banned.size ? `, banned ${[...banned].join(",")}` : "") +
    ` (${((performance.now() - started) / 1000).toFixed(0)}s)`,
  `won ${won.length} (${pct(won.length, runs.length)})`,
  "",
  "door            held    won   rate  alone",
]
const row = (name: string, held: Outcome[]) => {
  const w = held.filter((run) => run.won).length
  return `${name.padEnd(14)}${String(held.length).padStart(6)}${String(w).padStart(7)}  ${pct(w, held.length).padStart(5)}`
}
// `alone` is the wins where this was the only door the run held. The `won`
// column counts a run once for every door in it, so a door's figure there
// moves with how often the *others* turn up beside it: Bloodhound read 23 and
// 50 at the same strength, depending on what shipped next to it. Alone is the
// number a door is priced on.
for (const door of Object.keys(DOORS)) {
  const held = runs.filter((run) => run.doors.includes(door))
  const alone = held.filter((run) => run.won && run.doors.length === 1).length
  lines.push(`${row(door, held)}${String(alone).padStart(7)}`)
}
lines.push(
  row(
    "any",
    runs.filter((run) => run.doors.length > 0),
  ),
)
lines.push(
  row(
    "none",
    runs.filter((run) => run.doors.length === 0),
  ),
)

const builds = [...new Set(runs.map((run) => run.build).filter((b): b is string => b !== null))]
if (builds.length) {
  lines.push("", "build           runs    won   rate  none-won  levels")
  for (const build of builds.sort()) {
    const mine = runs.filter((run) => run.build === build)
    const bare = mine.filter((run) => run.won && run.doors.length === 0).length
    const levels = mine.reduce((sum, run) => sum + run.levels, 0) / mine.length
    lines.push(`${row(build, mine)}${String(bare).padStart(10)}${levels.toFixed(1).padStart(8)}`)
  }
}

lines.push("", "died on stage")
for (let stage = 1; stage <= STAGES; stage++) {
  const died = runs.filter((run) => !run.won && run.stage === stage).length
  lines.push(`  ${stage}  ${String(died).padStart(5)}`)
}
const leveled = won.filter((run) => run.levels > 0)
lines.push("", `wins holding a category level: ${leveled.length} of ${won.length}`)
console.log(lines.join("\n"))
