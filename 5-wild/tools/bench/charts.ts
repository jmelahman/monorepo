/**
 * Every chart and table in the benchmark post, from the episodes.
 *
 *   bun tools/bench/charts.ts [--post <file.md>] [--preview <file.html>] [--seeds <n>] [--dry]
 *
 * The post is `blog/llms-play-5-wild.md` in the site beside this checkout. Its
 * charts are inline SVG, each named by the `aria-labelledby` on its `<svg>`,
 * and its two tables sit under a line reading `Ascension N:`. This replaces
 * each one where it stands and touches nothing else, so the prose around a
 * chart is still written by hand and still has to be read after a rerun: a
 * sentence quoting a median does not follow the median.
 *
 * It began as a dozen throwaway scripts, one per chart, each with its own list
 * of who had played. A sixth model meant editing every one, and the ones nobody
 * edited went on drawing four. So nothing here names a model. A player is a
 * results directory with the first `--seeds` seeds (five) filed at one
 * ascension, and it is on every chart; a directory with fewer is listed on the
 * way out and drawn nowhere, since a bar over three runs beside bars over five
 * is a comparison of sample sizes. The baselines are left out: they are bots,
 * and `bench:report` is where they are compared.
 *
 * Every chart is rows under ascension headings, or a grid of them, because
 * that is the one layout that survives a roster that keeps growing: a row
 * costs height, and a page has that. The lead chart is one exception, bars
 * side by side as a leaderboard is expected to be, and it narrows its bars
 * rather than wrapping them. The scatter of cost against score is the other,
 * and what a growing roster costs it is at `scatter`.
 *
 * Everything but cost and speed is read by replaying the episode against the
 * engine, the way `verify` does, so no chart can disagree with the film of the
 * same run. Cost and speed are the two an episode cannot give; see
 * `measured.ts`.
 *
 * The colours are the post's, which is one ink, one accent and the game's own
 * three tile colours on a page that has no dark mode. They live here because
 * the SVG is inline and carries no stylesheet.
 */

import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs"
import { join } from "node:path"
import type { Episode } from "../../src/bench/session"
import { DEFAULT_LIMITS, Session } from "../../src/bench/session"
import { CATEGORIES, categoryOf } from "../../src/engine/categories"
import { RELICS } from "../../src/engine/relics"
import { en } from "../../src/ui/lang/en"
import { RESULTS, ROOT } from "./host"
import { MEASURED } from "./measured"
import { modelName, providerName } from "./names"
import { wordsFor } from "./words"

const args = process.argv.slice(2)
const flag = (name: string) => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}
const POST =
  flag("--post") ?? join(ROOT, "..", "jmelahman.github.io", "blog", "llms-play-5-wild.md")
const SEEDS = Number(flag("--seeds") ?? 5)
const LEDGER = join(ROOT, "bench", "videos", "uploaded.json")
const WATCH = "https://www.youtube.com/watch?v="

const INK = "#2c2c2c"
const ACC = "#8c0000"
const MUTED = "#8a8878"
const GRID = "#e3e1cf"
const BG = "#fbfaef"
// A bar that has dots laid over it: dark enough to read as a bar on the page,
// pale enough that a ring drawn on it is still a ring.
const BAR = "#d9d6c1"
const W = 720
/** Where the marks start in every chart of rows: the width of the longest name. */
const L = 215

// ---------------------------------------------------------------------------
// The players, and what replaying their runs says
// ---------------------------------------------------------------------------

type Round = {
  n: number
  target: number
  score: number
  guesses: number
  solved: boolean
  won: boolean
}
type Run = {
  seed: number
  cleared: number
  won: boolean
  score: number
  wallMs: number
  video: string | undefined
  rounds: Round[]
}
const SOURCES = [
  ["base", "round payout", "#787c7e"],
  ["unusedGuesses", "unused guesses", "#538d4e"],
  ["interest", "interest", "#b59f3b"],
  ["scoring", "scoring", ACC],
  ["sold", "sales", "#2f6f8f"],
  ["relics", "relic payouts", "#6b4c8a"],
] as const
type Source = (typeof SOURCES)[number][0]
type Player = {
  name: string
  /** One word, for a column header: Opus, Sol, Gemini. */
  short: string
  /** Who served it: Claude Code, OpenCode Go, OpenRouter. */
  provider: string
  dir: string
  ascension: number
  runs: Run[]
  actions: Record<string, number>
  /** Relic id to the number of runs that held it at any point. */
  relics: Record<string, number>
  words: Record<string, number>
  shapes: Record<string, number>
  gold: {
    rounds: number
    earned: Record<Source, number>
    spent: number
    stage: { earned: number; rounds: number }[]
  }
}
type Group = { ascension: number; title: string; players: Player[] }

const words = wordsFor("en")
const count = (tally: Record<string, number>, key: string, by = 1) => {
  tally[key] = (tally[key] ?? 0) + by
}
const sum = (values: readonly number[]) => values.reduce((a, b) => a + b, 0)
const mean = (values: readonly number[]) => (values.length ? sum(values) / values.length : 0)
const median = (values: readonly number[]) =>
  [...values].sort((a, b) => a - b)[Math.floor(values.length / 2)] ?? 0

function play(player: Player, episode: Episode, video: string | undefined): void {
  const session = new Session(episode.seed, words, episode.ascension, {
    ...DEFAULT_LIMITS,
    endless: episode.endless,
  })
  const run: Run = {
    seed: episode.seed,
    cleared: episode.result.roundsCleared,
    won: episode.result.won,
    score: episode.result.score,
    wallMs: episode.wallMs,
    video,
    rounds: [],
  }
  const held = new Set<string>()
  // The round a step belongs to. Counted from the first guess of each rather
  // than read off the state, because the shop after round three is still
  // stage one's business and the state has already moved on.
  let n = 0
  let fresh = true
  for (const step of episode.steps) {
    if (step.type === "guess" && fresh) {
      n++
      fresh = false
    }
    // `collect` pays the reward and clears it in one step, so the breakdown
    // has to be read before the step that announces it.
    const reward = session.state.reward
    const outcome = session.apply(step)
    const events = outcome.ok ? outcome.events : []
    count(player.actions, step.type)
    for (const relic of session.state.relics) held.add(relic.id)

    const stage = player.gold.stage[Math.min(7, Math.floor((Math.max(n, 1) - 1) / 3))]
    for (const event of events) {
      if (event.type !== "gold") continue
      if (event.delta < 0) {
        player.gold.spent -= event.delta
        continue
      }
      if (stage) stage.earned += event.delta
      if (event.reason === "round cleared") {
        player.gold.rounds++
        if (stage) stage.rounds++
        player.gold.earned.base += reward?.base ?? 0
        player.gold.earned.unusedGuesses += reward?.unusedGuesses ?? 0
        player.gold.earned.interest += reward?.interest ?? 0
        player.gold.earned.relics += reward?.relics ?? 0
      } else if (event.reason === "sold") player.gold.earned.sold += event.delta
      else player.gold.earned.scoring += event.delta
    }

    if (step.type !== "guess") continue
    count(player.words, step.word)
    count(player.shapes, categoryOf(step.word).id)
    const won = events.some((event) => event.type === "round_won")
    if (!won && !events.some((event) => event.type === "round_lost")) continue
    const round = session.state.round
    run.rounds.push({
      n: run.rounds.length + 1,
      target: round.target,
      score: round.score,
      guesses: round.guesses.length,
      solved: round.solved,
      won,
    })
    fresh = true
  }
  for (const id of held) count(player.relics, id)
  player.runs.push(run)
}

/**
 * The two rungs the post compares, and the only two drawn. A suite filed at any
 * other is an experiment about one rule, not a third class of player: GPT-6.1
 * Sol replayed its five seeds at ascension 9 to price the solve rule alone, and
 * the first run of this tool after it filed put an "Ascension 9" heading with
 * one row under it on every chart in the post. That finding is a paragraph and
 * a table the post writes by hand.
 */
const CHARTED = [0, 10]

const ledger: Record<string, string> = existsSync(LEDGER)
  ? JSON.parse(readFileSync(LEDGER, "utf8"))
  : {}
const incomplete: string[] = []
const players: Player[] = []
for (const dir of readdirSync(RESULTS).sort()) {
  if (dir.startsWith("baseline-")) continue
  const filed = new Map<number, number[]>()
  for (const name of readdirSync(join(RESULTS, dir))) {
    const match = /^seed-(\d+)-a(\d+)\.json$/.exec(name)
    if (!match) continue
    const ascension = Number(match[2])
    if (!CHARTED.includes(ascension)) continue
    filed.set(ascension, [...(filed.get(ascension) ?? []), Number(match[1])])
  }
  for (const [ascension, seeds] of filed) {
    const have = seeds.filter((seed) => seed <= SEEDS).length
    if (have < SEEDS) {
      incomplete.push(`${dir} a${ascension} (${have}/${SEEDS})`)
      continue
    }
    const read = (seed: number) =>
      JSON.parse(
        readFileSync(join(RESULTS, dir, `seed-${seed}-a${ascension}.json`), "utf8"),
      ) as Episode
    const name = modelName(read(1).label)
    const player: Player = {
      name,
      short: name.replace(/^(Claude|GPT-[\d.]+) /, "").split(" ")[0] ?? name,
      provider: providerName(read(1).label),
      dir,
      ascension,
      runs: [],
      actions: {},
      relics: {},
      words: {},
      shapes: {},
      gold: {
        rounds: 0,
        earned: { base: 0, unusedGuesses: 0, interest: 0, scoring: 0, sold: 0, relics: 0 },
        spent: 0,
        stage: Array.from({ length: 8 }, () => ({ earned: 0, rounds: 0 })),
      },
    }
    for (let seed = 1; seed <= SEEDS; seed++)
      play(player, read(seed), ledger[`${dir}-seed-${seed}-a${ascension}.mp4`])
    players.push(player)
  }
}

// One word is not always enough: GPT-6 Sol and GPT-6.1 Sol are both Sol, and two
// columns under one name are one column read twice. Those keep their whole
// names, and only those, since the header has room for a few long ones and not
// for every one.
for (const player of players) {
  const twins = players.filter(
    (other) => other.short === player.short && other.ascension === player.ascension,
  )
  if (twins.length > 1) for (const twin of twins) twin.short = twin.name.replace(/^Claude /, "")
}

const cleared = (player: Player) => player.runs.map((run) => run.cleared)
const wins = (player: Player) => player.runs.filter((run) => run.won).length
const rounds = (player: Player) => player.runs.flatMap((run) => run.rounds)
const heading = (ascension: number) =>
  ascension === 10
    ? "Ascension 10, the hardest"
    : ascension === 0
      ? "Ascension 0, the easiest"
      : `Ascension ${ascension}`

// Hardest first, and best first within one, by the median run's final score.
// It was the mean of rounds cleared, and the mean put a model that won one run
// in five above a model that won three, by 19.6 rounds to 19.4, on the strength
// of a single run the second lost in round two. Five runs is few enough that
// one of them should not be able to do that. Then it was the median of rounds
// cleared, which ranks these ten players exactly as the median score does, but
// ties wherever two medians are both a win, at 24, and a score does not. Rounds
// cleared break a tie now. The order is the ranking, so it is
// the same on every chart: a reader who has learned where Sonnet sits on the
// first should find it there on the tenth.
const groups: Group[] = [...new Set(players.map((player) => player.ascension))]
  .sort((a, b) => b - a)
  .map((ascension) => ({
    ascension,
    title: heading(ascension),
    players: players
      .filter((player) => player.ascension === ascension)
      .sort(
        (a, b) =>
          median(b.runs.map((run) => run.score)) - median(a.runs.map((run) => run.score)) ||
          median(cleared(b)) - median(cleared(a)) ||
          mean(cleared(b)) - mean(cleared(a)),
      ),
  }))
const everyone = groups.flatMap((group) => group.players)

// ---------------------------------------------------------------------------
// SVG
// ---------------------------------------------------------------------------

const esc = (text: string) => text.replace(/&/g, "&amp;").replace(/</g, "&lt;")
const fix = (value: number) => value.toFixed(1)
const comma = (value: number) => Math.round(value).toLocaleString("en-US")

function text(
  x: number,
  y: number,
  body: string,
  { size = 12, fill = INK, anchor = "start", bold = false } = {},
): string {
  const weight = bold ? ' font-weight="700"' : ""
  return `<text x="${fix(x)}" y="${fix(y)}" font-size="${size}" fill="${fill}" text-anchor="${anchor}"${weight}>${esc(body)}</text>`
}

function svg(id: string, title: string, height: number, body: string[]): string {
  return [
    `<svg class="results" style="width:100%;height:auto;margin:1em 0" viewBox="0 0 ${W} ${Math.ceil(height)}" role="img" aria-labelledby="${id}" font-family="inherit">`,
    `<title id="${id}">${esc(title)}</title>`,
    ...body,
    "</svg>",
  ].join("\n")
}

/**
 * A win is a filled dot and anything short of one a ring, on every chart that
 * has dots. The colour is the accent on all of them; it is an argument for
 * the key of the scatter, whose two sample dots are nobody's and are grey.
 */
const paint = (won: boolean, colour: string = ACC) =>
  won
    ? `fill="${colour}" stroke="${BG}" stroke-width="2" paint-order="stroke"`
    : `fill="${BG}" stroke="${colour}" stroke-width="2.5"`

function dot(cx: number, cy: number, run: Run, tip: string): string {
  const circle = `<circle cx="${fix(cx)}" cy="${fix(cy)}" r="5.5" ${paint(run.won)}><title>${esc(tip)}${run.video ? " (watch)" : ""}</title></circle>`
  return run.video ? `<a href="${WATCH}${run.video}">${circle}</a>` : circle
}

/**
 * The rows of a chart: a heading per ascension and a row per player under it.
 * Returns where the last one ended, which is all a caller needs to place its
 * axis and its note.
 */
function rows(
  body: string[],
  top: number,
  rowHeight: number,
  draw: (player: Player, y: number) => boolean,
  from: readonly Group[] = groups,
): number {
  let y = top
  for (const group of from) {
    const at = body.length
    body.push(text(0, y + 14, group.title, { fill: ACC, bold: true }))
    let drawn = 0
    let rowY = y + 22
    for (const player of group.players) {
      if (!draw(player, rowY)) continue
      drawn++
      rowY += rowHeight
    }
    // A heading over nothing, as when no label at an ascension has a cost.
    if (!drawn) body.length = at
    else y = rowY + 6
  }
  return y
}

type Bar = { value: number; label: string; tip: string; fill?: string }
/**
 * A bar a player, lying down. `key` is for a chart whose bars are not all the
 * accent: what each colour means, written on the first heading's line from
 * the right edge inward, where a heading never reaches.
 */
function bars(
  id: string,
  title: string,
  note: string,
  of: (player: Player) => Bar | null,
  key: readonly (readonly [label: string, fill: string])[] = [],
): string {
  const all = everyone.map(of).filter((bar) => bar !== null)
  const max = Math.max(...all.map((bar) => bar.value), 1e-9)
  // Room on the right for the value written after the longest bar.
  const scale = (W - 150 - L) / max
  const body: string[] = []
  let keyX = W
  for (const [label, fill] of [...key].reverse()) {
    keyX -= label.length * 5.8
    body.push(text(keyX, 14, label, { size: 11, fill: MUTED }))
    keyX -= 16
    body.push(`<rect x="${fix(keyX)}" y="4" width="11" height="11" rx="2" fill="${fill}"/>`)
    keyX -= 16
  }
  const bottom = rows(body, 0, 24, (player, y) => {
    const bar = of(player)
    if (!bar) return false
    const width = Math.max(2, bar.value * scale)
    body.push(text(0, y + 13, player.name, { size: 13 }))
    body.push(
      `<rect x="${L}" y="${y + 2}" width="${fix(width)}" height="14" rx="2" fill="${bar.fill ?? ACC}"><title>${esc(bar.tip)}</title></rect>`,
    )
    body.push(text(L + width + 8, y + 13, bar.label))
    return true
  })
  body.push(text(L, bottom + 10, note, { size: 11, fill: MUTED }))
  return svg(id, title, bottom + 16, body)
}

type Cell = { label: string; shade: number; tip: string }
/**
 * A table drawn as a heat map: a row per player, a column per thing counted,
 * the number in the cell and the shade behind it. `shade` is 0 to 1 and the
 * caller's to scale, since a share of guesses and a count of rerolls do not
 * share a ceiling. `extra` is a reference row under the players, set apart.
 */
function grid(
  body: string[],
  top: number,
  columns: readonly string[],
  cell: (player: Player, column: number) => Cell | null,
  note: string,
  extra?: { label: string; cells: (Cell | null)[] },
): number {
  const width = (W - L) / columns.length
  columns.forEach((column, j) => {
    body.push(
      text(L + j * width + width / 2, top + 12, column, {
        size: 11,
        fill: MUTED,
        anchor: "middle",
      }),
    )
  })
  const paint = (cells: (Cell | null)[], y: number) => {
    cells.forEach((value, j) => {
      if (!value) return
      const shade = Math.max(0.06, Math.min(1, value.shade))
      const x = L + j * width
      body.push(
        `<rect x="${fix(x + 1)}" y="${y}" width="${fix(width - 2)}" height="22" rx="2" fill="${ACC}" fill-opacity="${shade.toFixed(2)}"><title>${esc(value.tip)}</title></rect>`,
      )
      body.push(
        text(x + width / 2, y + 15, value.label, {
          size: 11,
          fill: shade > 0.6 ? "#ffffff" : INK,
          anchor: "middle",
        }),
      )
    })
  }
  let bottom = rows(body, top + 14, 26, (player, y) => {
    body.push(text(0, y + 15, player.name, { size: 13 }))
    paint(
      columns.map((_, j) => cell(player, j)),
      y,
    )
    return true
  })
  if (extra) {
    body.push(text(0, bottom + 15, extra.label, { size: 13, fill: MUTED }))
    paint(extra.cells, bottom)
    bottom += 32
  }
  body.push(text(L, bottom + 10, note, { size: 11, fill: MUTED }))
  return bottom + 16
}

// ---------------------------------------------------------------------------
// The charts
// ---------------------------------------------------------------------------

const charts: Record<string, string> = {}

/**
 * The lead: each player's median final score as a bar, which is the number
 * they are ranked by (see `groups`), and the five runs themselves laid over
 * the bar as dots a reader can switch off.
 *
 * It has had more forms than any other chart here, and they are worth a line
 * each. On rounds cleared out of 24: a red bar of the mean, which shouted over
 * the red dots on it; no bar, a mean tick and a whisker, and then no whisker;
 * the median as the tick; that one lying down, a row a player, where runs
 * that ended on the same round piled up the row and five wins had to overlap
 * to fit it. Each was a fair picture of five runs and none of them read as a
 * leaderboard at a glance, which a bar does. Then the bar again, pale so the
 * dots stay the loudest thing on it, and on points instead of rounds, because
 * points separate what rounds cannot: Opus and Sonnet are 19.4 and 19.6
 * rounds, and 3.5M and 621K points.
 *
 * A log scale, as on the score chart and for its reason: on a linear one every
 * lost run is a dot on the floor. It costs the bar its meaning as a length,
 * since a log scale has no zero and the bar stands on 1K because the axis
 * does.
 *
 * The bar is the median and not the mean. A mean of points is the winning run
 * divided by five: wherever one run won, the bar stood above the other four
 * dots, GLM's at 409K over runs of 95K and under, and the bars did not descend
 * in the order they stand in.
 *
 * There was a whisker of one standard deviation either side of the mean,
 * three times, the last in the dots' own column, and it went each time. Five
 * runs against a ceiling do not have a deviation that fits: Opus's mean plus
 * one is 29 rounds of a game that has 24, and on points a model with one win
 * has a deviation larger than its mean, so six whiskers in seven ran to the
 * floor. The dots are the honest version of the same spread, which is why
 * they are on until somebody turns them off, and each is a run that can be
 * watched.
 *
 * Under each name is nothing. The mean was there, and the median and the wins
 * under it; the bar and the dots say all three, and the numbers themselves
 * are in each bar's tooltip.
 *
 * The switch is a checkbox in a `foreignObject` and one `:has()` rule, both
 * inside the SVG. The post is static and this tool replaces a chart from
 * `<svg` to `</svg>`, so a control beside the chart would be the post's to
 * keep; one inside it is regenerated with it. No script: a checkbox already
 * knows whether it is ticked.
 */
const short = (value: number) =>
  value >= 1e6
    ? `${(value / 1e6).toFixed(1)}M`
    : value >= 1e3
      ? `${Math.round(value / 1e3)}K`
      : String(Math.round(value))

function leaderboard(): string {
  const body: string[] = [
    "<style>svg:has(.runs-shown:not(:checked)) .run{display:none}</style>",
    `<foreignObject x="${W - 130}" y="0" width="130" height="22"><label style="display:flex;justify-content:flex-end;align-items:center;gap:4px;font-size:11px;color:${MUTED};cursor:pointer"><input type="checkbox" class="runs-shown" checked style="margin:0;accent-color:${ACC}"> show each run</label></foreignObject>`,
  ]
  const left = 34
  const plot = 150
  const ticks: [number, string][] = [
    [1e3, "1K"],
    [1e4, "10K"],
    [1e5, "100K"],
    [1e6, "1M"],
    [1e7, "10M"],
  ]
  let y = 0
  for (const group of groups) {
    body.push(text(0, y + 14, group.title, { fill: ACC, bold: true }))
    const top = y + 30
    const yOf = (score: number) => top + plot - (plot * (Math.log10(Math.max(score, 1000)) - 3)) / 4
    for (const [tick, label] of ticks) {
      body.push(
        `<line x1="${left}" x2="${W}" y1="${fix(yOf(tick))}" y2="${fix(yOf(tick))}" stroke="${tick === 1000 ? MUTED : GRID}" stroke-width="1"/>`,
      )
      body.push(text(left - 6, yOf(tick) + 4, label, { size: 11, fill: MUTED, anchor: "end" }))
    }
    const pitch = Math.min(170, (W - left) / group.players.length)
    const half = Math.min(42, pitch / 2 - 15)
    group.players.forEach((player, k) => {
      const cx = left + pitch * (k + 0.5)
      const scores = player.runs.map((run) => run.score)
      const middle = median(scores)
      const tip = `${player.name}: a median of ${short(middle)} points, mean ${short(mean(scores))}, ${wins(player)} of ${SEEDS} runs won`
      body.push(
        `<g><title>${esc(tip)}</title>` +
          `<rect x="${fix(cx - half)}" y="${fix(yOf(middle))}" width="${fix(half * 2)}" height="${fix(yOf(1000) - yOf(middle))}" fill="${BAR}"/></g>`,
      )
      // Runs that ended near each other step out to either side rather than
      // stack, or two scores a hair apart are one dot.
      const placed: { x: number; y: number }[] = []
      body.push('<g class="run">')
      for (const run of [...player.runs].sort((a, b) => b.score - a.score)) {
        const at = yOf(run.score)
        const free = [0, 1, -1, 2, -2]
          .map((step) => cx + step * 13)
          .find((x) => placed.every((other) => Math.hypot(other.x - x, other.y - at) >= 12.5))
        const x = free ?? cx
        placed.push({ x, y: at })
        body.push(
          dot(
            x,
            at,
            run,
            `${player.name}, seed ${run.seed}: ${run.won ? "won" : `${run.cleared} of 24 rounds`}, ${comma(run.score)} points`,
          ),
        )
      }
      body.push("</g>")
      const [head = "", ...tail] = player.name.split(" ")
      const base = top + plot + 16
      body.push(text(cx, base, head, { size: 11.5, anchor: "middle" }))
      body.push(text(cx, base + 13, tail.join(" "), { size: 11.5, anchor: "middle" }))
    })
    y = top + plot + 16 + 13 + 22
  }
  body.push(
    `<text x="${left}" y="${y}" font-size="11" fill="${MUTED}">↑ higher is better · final score, log scale · median of five runs<tspan class="run"> · ● a run that won   ○ a run that lost</tspan></text>`,
  )
  return svg(
    "results-title",
    "Final score of each model, where higher is better: the median of five runs as a bar, and each run as a dot.",
    y + 6,
    body,
  )
}
charts["results-title"] = leaderboard()

{
  // Final score, one dot a run. A log axis because the game's targets grow
  // several hundredfold over a run and so do the scores that meet them: on a
  // linear one every lost run is a dot on the left edge.
  const body: string[] = []
  const right = W - 20
  const xOf = (score: number) => L + ((right - L) * (Math.log10(Math.max(score, 1000)) - 3)) / 4
  const lines = body.length
  const bottom = rows(body, 0, 28, (player, y) => {
    body.push(text(0, y + 18, player.name, { size: 13 }))
    body.push(`<line x1="${L}" x2="${right}" y1="${y + 14}" y2="${y + 14}" stroke="${GRID}"/>`)
    for (const run of player.runs)
      body.push(
        dot(
          xOf(run.score),
          y + 14,
          run,
          `${player.name}, seed ${run.seed}: ${comma(run.score)} points`,
        ),
      )
    return true
  })
  const ticks = ["1K", "10K", "100K", "1M", "10M"].flatMap((label, k) => [
    `<line x1="${fix(xOf(10 ** (k + 3)))}" x2="${fix(xOf(10 ** (k + 3)))}" y1="6" y2="${bottom}" stroke="${GRID}"/>`,
    text(xOf(10 ** (k + 3)), bottom + 12, label, { size: 11, fill: MUTED, anchor: "middle" }),
  ])
  body.splice(lines, 0, ...ticks)
  body.push(
    text(L, bottom + 28, "final score of each run, log scale · ● won   ○ lost", {
      size: 11,
      fill: MUTED,
    }),
  )
  charts["score-title"] = svg(
    "score-title",
    "Final score of each of five runs for each model, on a log scale.",
    bottom + 34,
    body,
  )
}

{
  const body: string[] = []
  const top = (player: Player) =>
    Object.entries(player.words).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
  const height = grid(
    body,
    0,
    ["most played", "2nd", "3rd", "4th", "5th"],
    (player, j) => {
      const entry = top(player)[j]
      if (!entry) return null
      const [word, times] = entry
      const played = rounds(player).length
      return {
        label: `${word.toUpperCase()} ${times}`,
        // Against rounds, not guesses: a word played in every round is an
        // opener, and that is the habit the chart is looking for.
        shade: (times / played) * 0.6,
        tip: `${player.name}: ${word.toUpperCase()} played ${times} times in ${played} rounds`,
      }
    },
    "each model's five most-played words and how many times it played them, over five runs",
  )
  charts["words-title"] = svg(
    "words-title",
    "The five most-played words for each model across its five runs.",
    height,
    body,
  )
}

{
  const body: string[] = []
  const share =
    (tally: Record<string, number>, of: string, who: string) =>
    (id: string): Cell => {
      const total = sum(Object.values(tally))
      const percent = (100 * (tally[id] ?? 0)) / total
      return {
        label: `${percent.toFixed(0)}%`,
        // Distinct is the shape of most words, so it gets its own scale or it
        // would be the only dark column and the other four all one pale.
        shade: id === "distinct" ? (percent - 40) / 40 : percent / 30,
        tip: `${who}: ${en.category[id]?.name ?? id} is ${percent.toFixed(1)}% ${of} (${tally[id] ?? 0} of ${total})`,
      }
    }
  const answers: Record<string, number> = {}
  for (const answer of words.answers) count(answers, categoryOf(answer).id)
  const height = grid(
    body,
    0,
    CATEGORIES.map((category) => en.category[category.id]?.name ?? category.id),
    (player, j) => share(player.shapes, "of guesses", player.name)(CATEGORIES[j]?.id ?? ""),
    "share of guesses in each word shape · a guess scores as the first shape on this list it matches",
    {
      label: "Every answer in the game",
      cells: CATEGORIES.map((category) =>
        share(answers, "of answers", "Every answer in the game")(category.id),
      ),
    },
  )
  charts["shapes-title"] = svg(
    "shapes-title",
    "Share of each model's guesses in each of the five word shapes, beside the share of the game's answers in each shape.",
    height,
    body,
  )
}

charts["guesses-title"] = bars(
  "guesses-title",
  "Guesses per cleared round for each model. Lower is better; six is the limit.",
  "guesses per cleared round (lower is better, six is the limit) · rounds solved of rounds cleared",
  (player) => {
    const won = rounds(player).filter((round) => round.won)
    if (!won.length) return null
    const value = mean(won.map((round) => round.guesses))
    const solved = won.filter((round) => round.solved).length
    return {
      value,
      label: `${value.toFixed(2)} · solved ${solved} of ${won.length}`,
      tip: `${player.name}: ${value.toFixed(2)} guesses per cleared round, solved ${solved} of ${won.length} cleared rounds`,
    }
  },
)

{
  const body: string[] = []
  const ends = ["1", "2", "3", "4", "5", "6", "no solve", "lost"]
  const end = (round: Round) =>
    !round.won ? "lost" : round.solved ? String(round.guesses) : "no solve"
  const height = grid(
    body,
    0,
    ends,
    (player, j) => {
      const all = rounds(player)
      const times = all.filter((round) => end(round) === ends[j]).length
      if (!times) return null
      return {
        label: String(times),
        shade: (times / all.length) * 2,
        tip: `${player.name}: ${times} of ${all.length} rounds, ${ends[j]}`,
      }
    },
    "rounds by how they ended: solved in that many guesses, cleared on score without solving, or lost",
  )
  charts["dist-title"] = svg(
    "dist-title",
    "How each model's rounds ended: solved in one to six guesses, cleared without solving, or lost.",
    height,
    body,
  )
}

{
  // Relics down the side and players across, which is the wrong way round for
  // every other chart here and the only way round for this one: there are four
  // dozen relics and their names are the long ones.
  const body: string[] = []
  const name = (id: string) => en.relic[id]?.name ?? id
  const total = (id: string) => sum(everyone.map((player) => player.relics[id] ?? 0))
  const order = RELICS.map((relic) => relic.id).sort(
    (a, b) => total(b) - total(a) || name(a).localeCompare(name(b)),
  )
  const label = 140
  // Two blocks side by side for as long as a cell stays wide enough for a
  // digit, which is about a dozen players. One block of 47 rows is a chart
  // taller than a screen, so it is the fallback and not the choice.
  const blocks = (W / 2 - 16 - label) / everyone.length >= 16 ? 2 : 1
  const cell = Math.min(40, Math.floor((W / blocks - 16 - label) / everyone.length))
  const perBlock = Math.ceil((order.length + 1) / blocks)
  // The headers lean at 50 degrees, so the room they need above the grid is
  // the longest of them times the sine of that. It was a fixed 62, sized for
  // "Sonnet A10", and "GPT-6.1 Sol A10" went off the top of the chart.
  const header = (player: Player) => `${player.short} A${player.ascension}`
  const top = 14 + Math.ceil(Math.max(...everyone.map((player) => header(player).length)) * 4.6)
  const row = 20
  for (let block = 0; block < blocks; block++) {
    const x0 = block * (W / blocks)
    everyone.forEach((player, i) => {
      const x = x0 + label + i * cell + cell / 2
      // Slanted, because a column is as wide as a number and a name is not.
      body.push(
        `<text x="${fix(x)}" y="${top - 8}" font-size="11" fill="${MUTED}" transform="rotate(-50 ${fix(x)} ${top - 8})">${esc(header(player))}</text>`,
      )
    })
    const ids: (string | null)[] = order.slice(block * perBlock, (block + 1) * perBlock)
    if (block === blocks - 1) ids.push(null)
    ids.forEach((id, r) => {
      const y = top + r * row
      if (id === null) {
        body.push(text(x0, y + 17, "Different relics held", { fill: ACC, bold: true }))
        everyone.forEach((player, i) => {
          body.push(
            text(
              x0 + label + i * cell + cell / 2,
              y + 17,
              String(Object.keys(player.relics).length),
              {
                size: 11,
                fill: ACC,
                anchor: "middle",
                bold: true,
              },
            ),
          )
        })
        return
      }
      body.push(text(x0, y + 13, name(id), { fill: total(id) ? INK : MUTED }))
      everyone.forEach((player, i) => {
        const runs = player.relics[id] ?? 0
        const x = x0 + label + i * cell
        const shade = runs ? 0.15 + (0.85 * runs) / SEEDS : 0
        const tip = runs
          ? `${player.name}, ascension ${player.ascension}: ${name(id)} held in ${runs} of ${SEEDS} runs`
          : `${player.name}, ascension ${player.ascension}: never held ${name(id)}`
        body.push(
          `<rect x="${x + 1}" y="${y}" width="${cell - 2}" height="${row - 2}" rx="2" fill="${runs ? ACC : GRID}" fill-opacity="${runs ? shade.toFixed(2) : "0.45"}"><title>${esc(tip)}</title></rect>`,
        )
        if (runs)
          body.push(
            text(x + cell / 2, y + 13, String(runs), {
              size: 11,
              fill: shade > 0.6 ? "#ffffff" : INK,
              anchor: "middle",
            }),
          )
      })
    })
  }
  const bottom = top + (perBlock + (blocks === 1 ? 1 : 0)) * row + 10
  body.push(
    text(
      0,
      bottom + 10,
      `runs, of ${SEEDS}, in which the model held the relic · blank is never · all ${order.length} relics, most-held first`,
      { size: 11, fill: MUTED },
    ),
  )
  charts["relics-title"] = svg(
    "relics-title",
    "Every relic in the game against every model, with the number of its five runs the model held it in.",
    bottom + 16,
    body,
  )
}

{
  const body: string[] = []
  const kinds = [
    ["Purchases", "buy"],
    ["Relics sold", "sell_relic"],
    ["Rerolls", "reroll"],
    ["Consumables used", "use_consumable"],
  ] as const
  // Per stage and not per run, which it was: a run is as long as the player
  // is good, so per run Opus's 25.6 purchases against GLM's 9.4 said mostly
  // that Opus saw more shops, which the leaderboard had said already. A stage
  // is three rounds played, pooled over the runs, so a run that died a round
  // into its fifth has four and a third of them and not five: the shop comes
  // after a round, and a stage barely entered was barely shopped in.
  const stages = (player: Player) => Math.max(rounds(player).length, 1) / 3
  const per = (player: Player, kind: string) => (player.actions[kind] ?? 0) / stages(player)
  const height = grid(
    body,
    0,
    kinds.map(([label]) => label),
    (player, j) => {
      const [label, kind] = kinds[j] ?? kinds[0]
      const value = per(player, kind)
      // Each column against its own busiest player: purchases run to four
      // a stage and rerolls to under one, and one scale would blank the second.
      const most = Math.max(...everyone.map((other) => per(other, kind)), 1e-9)
      // Two places: a reroll every dozen stages is 0.08, and at one it is 0.1
      // beside a player's 0.1 that rerolls half as often again.
      return {
        label: value.toFixed(2),
        shade: (value / most) * 0.85,
        tip: `${player.name}: ${value.toFixed(2)} ${label.toLowerCase()} per stage, ${player.actions[kind] ?? 0} over ${rounds(player).length} rounds`,
      }
    },
    `per stage of three rounds played, over ${SEEDS} runs · each column shaded against its own highest`,
  )
  charts["shop-title"] = svg(
    "shop-title",
    "Shop actions per stage played for each model: purchases, relics sold, rerolls and consumables used.",
    height,
    body,
  )
}

{
  const body: string[] = []
  const right = W - 110
  const earned = (player: Player) =>
    sum(Object.values(player.gold.earned)) / Math.max(1, player.gold.rounds)
  const scale = (right - L) / Math.max(10, Math.ceil(Math.max(...everyone.map(earned))))
  let x = 0
  for (const [, label, colour] of SOURCES) {
    body.push(`<rect x="${fix(x)}" y="4" width="12" height="12" rx="2" fill="${colour}"/>`)
    body.push(text(x + 17, 14, label))
    x += 17 + label.length * 6.6 + 18
  }
  const lines = body.length
  const bottom = rows(body, 24, 30, (player, y) => {
    const per = Math.max(1, player.gold.rounds)
    body.push(text(0, y + 15, player.name, { size: 13 }))
    let at = L
    for (const [source, label, colour] of SOURCES) {
      const value = player.gold.earned[source] / per
      const width = value * scale
      if (width <= 0) continue
      body.push(
        `<rect x="${fix(at + 1)}" y="${y}" width="${fix(Math.max(width - 2, 1))}" height="20" rx="2" fill="${colour}"><title>${esc(`${player.name}: ${value.toFixed(2)} gold a round from ${label}`)}</title></rect>`,
      )
      if (width > 26)
        body.push(
          text(at + width / 2, y + 14, value.toFixed(1), {
            size: 11,
            fill: "#ffffff",
            anchor: "middle",
          }),
        )
      at += width
    }
    body.push(text(at + 8, y + 10, `${earned(player).toFixed(1)} earned`, { size: 11 }))
    body.push(
      text(at + 8, y + 22, `${(player.gold.spent / per).toFixed(1)} spent`, {
        size: 11,
        fill: MUTED,
      }),
    )
    return true
  })
  const ticks: string[] = []
  for (let gold = 0; L + gold * scale <= right + 1; gold += 2) {
    ticks.push(
      `<line x1="${fix(L + gold * scale)}" x2="${fix(L + gold * scale)}" y1="28" y2="${bottom}" stroke="${GRID}"/>`,
    )
    ticks.push(
      text(L + gold * scale, bottom + 12, String(gold), {
        size: 11,
        fill: MUTED,
        anchor: "middle",
      }),
    )
  }
  body.splice(lines, 0, ...ticks)
  body.push(
    text(L, bottom + 28, "gold earned per round cleared, by where it came from", {
      size: 11,
      fill: MUTED,
    }),
  )
  charts["gold-title"] = svg(
    "gold-title",
    "Gold earned per round cleared by each model, split by source, with gold spent beside it.",
    bottom + 34,
    body,
  )
}

{
  const body: string[] = []
  const stages = Array.from({ length: 8 }, (_, stage) => `stage ${stage + 1}`)
  body.push(text(0, 12, "Guesses", { size: 13, bold: true }))
  const middle = grid(
    body,
    0,
    stages,
    (player, stage) => {
      const won = rounds(player).filter(
        (round) => round.won && Math.floor((round.n - 1) / 3) === stage,
      )
      if (!won.length) return null
      const value = mean(won.map((round) => round.guesses))
      return {
        label: value.toFixed(1),
        shade: (value - 2.5) / 3.5,
        tip: `${player.name}, stage ${stage + 1}: ${value.toFixed(2)} guesses per cleared round over ${won.length} rounds`,
      }
    },
    "guesses per cleared round in each stage · blank where no run got that far",
  )
  body.push(text(0, middle + 22, "Gold", { size: 13, bold: true }))
  const height = grid(
    body,
    middle + 10,
    stages,
    (player, stage) => {
      const at = player.gold.stage[stage]
      if (!at?.rounds) return null
      const value = at.earned / at.rounds
      return {
        label: value.toFixed(1),
        shade: (value - 4) / 9,
        tip: `${player.name}, stage ${stage + 1}: ${value.toFixed(1)} gold earned per cleared round over ${at.rounds} rounds`,
      }
    },
    "gold earned per cleared round in each stage, from every source",
  )
  charts["stages-title"] = svg(
    "stages-title",
    "Guesses per cleared round and gold earned per cleared round in each of the eight stages, for each model.",
    height,
    body,
  )
}

const key = (player: Player) => `${player.dir}@a${player.ascension}`
const played = (run: Run) => run.cleared + (run.won ? 0 : 1)
const estimated: string[] = []
const seconds = (player: Player) => {
  const measured = MEASURED[key(player)]?.seconds
  if (measured !== undefined) return measured
  estimated.push(`${player.name} a${player.ascension}`)
  return Math.round(median(player.runs.map((run) => run.wallMs / 1000 / played(run))))
}
const speeds = new Map(everyone.map((player) => [player, seconds(player)]))
// A speed nobody measured is marked as one, since the note under the chart
// promises the waits are out and wall time cannot promise that.
const guessed = (player: Player) => MEASURED[key(player)]?.seconds === undefined

/**
 * Cost against score, a dot a player, and the one chart here that is neither
 * rows nor a grid. It was two charts of bars, cents a round and seconds a
 * round, and then one of cost against speed, and none of them asked what a
 * reader of a price list asks, which is what the money buys. Up is the median
 * final score, the number the players are ranked by (see `groups`), so the
 * chart is the leaderboard with the bill beside it. Speed is in each dot's
 * tooltip, and has the chart after this one to itself.
 *
 * Score is on a log scale, as on the leaderboard and for its reason: on a
 * linear one every player that lost its median run is a dot on the floor.
 * Cost was too, and is linear now. A log axis made Haiku's 0.4 cents and
 * Luna's 0.6 look as far apart as Sonnet's 13.5 and Opus's 20.6, which is
 * true as a ratio and not as money, and it has no zero, so the two models
 * played for free stood in a column of their own across a break in the axis.
 * On a linear one a cent is a cent's width and free is where the axis starts.
 *
 * Only the hardest ascension played is on it. Both were, in two colours, and
 * the two do not share a scale whatever the axis says: a score at ascension 0
 * is made under easier rules than one at ascension 10, so a dot was above or
 * below dots of its own colour only, and the picture invited the one
 * comparison it could not support. The easier ascension's costs are still in
 * `measured.ts`, and its speeds are on the chart after this one.
 *
 * A dot is filled if the player won any of its runs and a ring if it won none,
 * which is the convention of `dot` moved up from a run to a player, because a
 * cost is measured once per label and there is no run to hang it on.
 *
 * A model played for free is at 0¢. A player with no cost at all is not on the
 * chart: nobody knows where it goes, and zero is a claim. It is named on the
 * way out.
 *
 * The names are placed, not just written beside the dots, since two models a
 * few percent apart on both axes are a dot's width apart on the page. Each takes
 * whichever of ten places round its dot lands on the fewest dots and names.
 * That can still be beaten by a crowd, and a rerun with a new model on it is
 * worth a look at `--preview` for that reason.
 */
function scatter(): string {
  const TOP = 44
  const BOTTOM = 424
  /** The frame, and zero a dot's width inside it so a free model is not on the line. */
  const LEFT = 40
  const ZERO = LEFT + 12
  const cents = (player: Player) => MEASURED[key(player)]?.cents
  /** `groups` is hardest first. */
  const shown = groups[0]
  const priced = (shown?.players ?? []).filter((player) => cents(player) !== undefined)

  // Cost, linear from nothing: a tick every 1, 2 or 5 of whichever power of ten
  // gives no more than six of them.
  const dearest = Math.max(...priced.map((player) => cents(player) ?? 0), 1e-9)
  const every =
    [1, 2, 5]
      .flatMap((step) => [0.01, 0.1, 1, 10, 100].map((power) => step * power))
      .sort((a, b) => a - b)
      .find((step) => dearest / step <= 6) ?? dearest
  const x = {
    ticks: Array.from({ length: Math.floor(dearest / every) + 1 }, (_, i) =>
      Number((i * every).toPrecision(6)),
    ),
    at: (value: number) => ZERO + (value / dearest) * (W - 12 - ZERO),
  }
  // Score, logarithmic: a third of the way to the next value past the data on
  // each side, so no dot sits on the frame, and a tick a decade.
  const score = (player: Player) => median(player.runs.map((run) => run.score))
  const seen = priced.map((player) => Math.max(score(player), 1))
  const lo = Math.log(Math.min(...seen) / 1.3)
  const hi = Math.log(Math.max(...seen) * 1.3)
  const y = {
    ticks: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
      .map((decade) => 10 ** decade)
      .filter((tick) => Math.log(tick) >= lo && Math.log(tick) <= hi),
    at: (value: number) =>
      BOTTOM - ((Math.log(Math.max(value, 1)) - lo) / (hi - lo)) * (BOTTOM - TOP),
  }

  const body: string[] = []
  // The heading the other charts give an ascension, and the key beside it.
  if (shown) body.push(text(0, 14, shown.title, { fill: ACC, bold: true }))
  let keyX = (shown?.title.length ?? 0) * 7.2 + 18
  const legend = (label: string, won: boolean) => {
    body.push(`<circle cx="${keyX + 6}" cy="10" r="5.5" ${paint(won, MUTED)}/>`)
    body.push(text(keyX + 16, 14, label, { fill: MUTED, size: 11 }))
    keyX += 16 + label.length * 5.6 + 18
  }
  legend("won a run", true)
  legend("won none", false)

  body.push(
    text(0, 34, `median final score of ${SEEDS} runs, on a log scale`, {
      size: 11,
      fill: MUTED,
    }),
  )
  for (const tick of y.ticks) {
    body.push(
      `<line x1="${LEFT}" x2="${W}" y1="${fix(y.at(tick))}" y2="${fix(y.at(tick))}" stroke="${GRID}"/>`,
    )
    body.push(text(LEFT - 6, y.at(tick) + 4, short(tick), { size: 11, fill: MUTED, anchor: "end" }))
  }
  for (const tick of x.ticks) {
    body.push(
      `<line x1="${fix(x.at(tick))}" x2="${fix(x.at(tick))}" y1="${TOP}" y2="${BOTTOM}" stroke="${GRID}"/>`,
    )
    body.push(
      text(x.at(tick), BOTTOM + 16, `${tick}¢`, { size: 11, fill: MUTED, anchor: "middle" }),
    )
  }
  body.push(
    `<line x1="${LEFT}" x2="${LEFT}" y1="${TOP}" y2="${BOTTOM}" stroke="${MUTED}"/>`,
    `<line x1="${LEFT}" x2="${W}" y1="${BOTTOM}" y2="${BOTTOM}" stroke="${MUTED}"/>`,
  )
  body.push(
    text((LEFT + W) / 2, BOTTOM + 34, "US cents per round played", {
      size: 11,
      fill: MUTED,
      anchor: "middle",
    }),
  )

  type Box = { x0: number; y0: number; x1: number; y1: number }
  const hit = (a: Box, b: Box) => a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1
  const points = priced.map((player) => {
    const price = cents(player) ?? 0
    return { player, price, cx: x.at(price), cy: y.at(score(player)) }
  })
  // Georgia at 12px, near enough: the page's own face is not known here.
  const width = (player: Player) => player.name.length * 5.7
  const options = ({ player, cx, cy }: (typeof points)[number]) => {
    const w = width(player)
    return [
      { anchor: "start", tx: cx + 10, ty: cy + 4, x0: cx + 10 },
      { anchor: "end", tx: cx - 10, ty: cy + 4, x0: cx - 10 - w },
      { anchor: "middle", tx: cx, ty: cy - 11, x0: cx - w / 2 },
      { anchor: "middle", tx: cx, ty: cy + 19, x0: cx - w / 2 },
      { anchor: "start", tx: cx + 7, ty: cy - 8, x0: cx + 7 },
      { anchor: "start", tx: cx + 7, ty: cy + 16, x0: cx + 7 },
      { anchor: "end", tx: cx - 7, ty: cy - 8, x0: cx - 7 - w },
      { anchor: "end", tx: cx - 7, ty: cy + 16, x0: cx - 7 - w },
      // A line further off, for a dot hemmed in on every side at arm's length.
      { anchor: "middle", tx: cx, ty: cy - 24, x0: cx - w / 2 },
      { anchor: "middle", tx: cx, ty: cy + 32, x0: cx - w / 2 },
    ].map((place) => ({
      ...place,
      box: { x0: place.x0, y0: place.ty - 10, x1: place.x0 + w, y1: place.ty + 3 },
    }))
  }
  type Place = ReturnType<typeof options>[number]
  const dots: Box[] = points.map(({ cx, cy }) => ({
    x0: cx - 7,
    y0: cy - 7,
    x1: cx + 7,
    y1: cy + 7,
  }))
  const chosen: (Place | undefined)[] = points.map(() => undefined)
  // A name across a dot hides a player and a name across a name hides only
  // letters, so the first costs more. Three passes, each name moved with every
  // other where the last pass left it: the first is the greedy one, and the
  // later two let a name that took the obvious place give it up to a
  // neighbour that had nowhere else.
  for (let pass = 0; pass < 3; pass++)
    points.forEach((point, i) => {
      const others = chosen.flatMap((place, j) => (place && j !== i ? [place.box] : []))
      const cost = ({ box }: Place) =>
        (box.x0 < LEFT + 2 || box.x1 > W || box.y0 < TOP || box.y1 > BOTTOM ? 100 : 0) +
        5 * dots.filter((other) => hit(box, other)).length +
        2 * others.filter((other) => hit(box, other)).length
      chosen[i] = options(point).reduce((a, b) => (cost(b) < cost(a) ? b : a))
    })
  const names: string[] = []
  points.forEach(({ player, price, cx, cy }, i) => {
    const place = chosen[i]
    if (place) names.push(text(place.tx, place.ty, player.name, { anchor: place.anchor }))
    const pace = `${guessed(player) ? "~" : ""}${speeds.get(player) ?? 0} s`
    const tip = `${player.name}, ascension ${player.ascension}: median ${comma(score(player))} points, at ${price ? `${price.toFixed(1)}¢` : "no cost"} and ${pace} a round, won ${wins(player)} of ${SEEDS}`
    body.push(
      `<circle cx="${fix(cx)}" cy="${fix(cy)}" r="5.5" ${paint(wins(player) > 0)}><title>${esc(tip)}</title></circle>`,
    )
  })
  // Over the grid, which would otherwise rule a line through every name on a
  // tick: the page's colour as an outline, under the ink.
  body.push(`<g stroke="${BG}" stroke-width="3" paint-order="stroke">`, ...names, "</g>")
  return svg(
    "value-title",
    `Median final score against cost per round played, for each model at ascension ${shown?.ascension ?? 0}. Score is on a logarithmic scale.`,
    BOTTOM + 42,
    body,
  )
}
charts["value-title"] = scatter()

/**
 * Seconds a round, as the bars it has always been. They stood up for a day, in
 * the leaderboard's layout, when cost left for the scatter and this was the
 * one chart of the pair still a list. Standing, the six players under 35
 * seconds were bars a few pixels tall beside one of 455, and their names
 * wrapped under them; lying down the name is on the bar's own line and the
 * number at its end.
 *
 * A bar is coloured by who served the model, since that is half of what it
 * measures: a slow bar is a slow model or a slow queue, and the six longest
 * here came through two providers and none through the third. The colours are
 * the post's own, dealt in the order the providers first appear, so the accent
 * goes to whoever served the top of the leaderboard.
 */
const providers = [...new Set(everyone.map((player) => player.provider))]
const hue = (provider: string) =>
  [ACC, "#2f6f8f", "#b59f3b", "#538d4e", "#6b4c8a", "#787c7e"][providers.indexOf(provider) % 6] ??
  ACC
charts["speed-title"] = bars(
  "speed-title",
  "Seconds of play per round for each model, coloured by the provider that served it.",
  `seconds per round played, shop included, waits excluded${everyone.some(guessed) ? " · ~ from wall time, not measured" : ""}`,
  (player) => {
    const value = speeds.get(player) ?? 0
    const label = `${guessed(player) ? "~" : ""}${value} s`
    return {
      value,
      label,
      tip: `${player.name}, through ${player.provider}: ${label} per round`,
      fill: hue(player.provider),
    }
  },
  providers.map((provider) => [provider, hue(provider)] as const),
)

// ---------------------------------------------------------------------------
// The tables, and putting it all in the post
// ---------------------------------------------------------------------------

function table(group: Group): string {
  const seeds = Array.from({ length: SEEDS }, (_, i) => `Seed ${i + 1}`)
  const lines = [
    `| Player | ${seeds.join(" | ")} | Mean | Median | Won |`,
    `| --- | ${seeds.map(() => "---").join(" | ")} | --- | --- | --- |`,
  ]
  for (const player of group.players) {
    const cells = player.runs.map((run) => {
      const value = run.won ? `**${run.cleared}**` : String(run.cleared)
      return run.video ? `[${value}](${WATCH}${run.video})` : value
    })
    lines.push(
      `| ${player.name} | ${cells.join(" | ")} | ${mean(cleared(player)).toFixed(1)} | ${median(cleared(player))} | ${wins(player)}/${SEEDS} |`,
    )
  }
  return lines.join("\n")
}

for (const group of groups) {
  console.log(group.title)
  for (const player of group.players)
    console.log(
      `  ${player.name.padEnd(22)} ${cleared(player).join(" ").padEnd(16)} mean ${mean(cleared(player)).toFixed(1)}, ${wins(player)}/${SEEDS} won  (${player.dir})`,
    )
}
if (incomplete.length)
  console.log(`\nnot drawn, fewer than ${SEEDS} runs: ${incomplete.join(", ")}`)
if (estimated.length) console.log(`speed from wall time, not measured: ${estimated.join(", ")}`)
const unpriced = everyone.filter((player) => MEASURED[key(player)]?.cents === undefined)
if (unpriced.length)
  console.log(
    `no cost in measured.ts: ${unpriced.map((player) => `${player.name} a${player.ascension}`).join(", ")}`,
  )

const preview = flag("--preview")
if (preview) {
  writeFileSync(
    preview,
    `<body style="background:${BG};font-family:Georgia,serif;width:${W}px;margin:20px">${Object.values(charts).join("<hr>")}</body>`,
  )
  console.log(`\nwrote ${preview}`)
}

if (!args.includes("--dry")) {
  let post = readFileSync(POST, "utf8")
  const missing: string[] = []
  for (const [id, chart] of Object.entries(charts)) {
    const found = new RegExp(`<svg [^>]*aria-labelledby="${id}"[\\s\\S]*?</svg>`).exec(post)
    if (!found) {
      missing.push(id)
      continue
    }
    post = post.slice(0, found.index) + chart + post.slice(found.index + found[0].length)
  }
  for (const group of groups) {
    const found = new RegExp(`(Ascension ${group.ascension}:\\n\\n)(\\|.*\\n)+`).exec(post)
    if (!found) {
      missing.push(`the "Ascension ${group.ascension}:" table`)
      continue
    }
    post =
      post.slice(0, found.index) +
      found[1] +
      table(group) +
      "\n" +
      post.slice(found.index + found[0].length)
  }
  writeFileSync(POST, post)
  console.log(`\nwrote ${POST}`)
  // Not an error: the post decides which charts it carries, and one cut from
  // it should not have to be cut from here the same day.
  if (missing.length) console.log(`not in the post, so not written: ${missing.join(", ")}`)
}
