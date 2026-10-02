import type { Color, GameEvent, Payout, RunState } from "../../../engine"
import { keyboardColors, MULT_FOR_COLOR } from "../../../engine"
import type { TileColor } from "../../audio"
import { h } from "../../dom"
import { formatNumber as num } from "../../format"
import { categoryLevel, growthBadge, payoutBadge, relicCard, ui } from "../../lang"
import { hasFx } from "../../skin"
import { meterFill } from "../../views"
import { centerOf, fxDom, place } from "../layer"
import { juiced, ms, reduced, replay, tween } from "../motion"
import { heat, roll } from "../numbers"
import { burst, clearParticles, type Kind } from "../particles"
import { shake } from "../shake"
import { Chain, skip, skipping, step } from "../timeline"
import type { SceneContext } from "./context"

/**
 * The scoring show: a submitted guess, replayed over the board from its events.
 *
 * Moved here out of `App.animate` unchanged, so that the phone plays exactly
 * the sequence it always has; the table's version of the show is built on top
 * of this one, behind `juiced()`, and a phone never reaches it.
 */

/**
 * Per-event pacing, in ms. Slow enough to read, fast enough to not be a cutscene.
 *
 * The phone's tile takes 380ms to turn and shows its color at 190. Its next
 * event waits 200, so the payment and the readout can land at the reveal, with
 * ten ms before the following tile starts. The flips still overlap by 180ms.
 * Without that small lead, the readout announces a tile before its color does.
 *
 * These, and every other duration below that is a motion rather than a reading
 * time, are what the game is *authored* at. What it plays at is these divided by
 * the animation speed the player chose; `ms` is where that happens and
 * `../../speed` is why. Nothing here changes when the setting does.
 *
 * `solve` is the outlier on purpose: it is the last beat before the reward screen
 * takes the board away, and it has to outlast `COUNT_UP` by enough that the pile's
 * new total is legible standing still rather than glimpsed mid-climb.
 */
export const PACE = { tile: 200, relic: 150, solve: 900, total: 400 }

/**
 * The tile turn. It runs longer than the gap between tiles on purpose, so the
 * reveals overlap into a cascade rather than a queue of separate flips.
 *
 * `total` must stay in step with the `.tile.flip` animation in the stylesheet.
 * CSS owns the motion, this owns when the class comes back off, and a mismatch
 * either clips the flip or leaves the tile stuck mid-turn.
 */
export const FLIP = { total: 380, half: 190 }

/** How long a tile's `+chips +mult` badge lives. Matches `gain-rise` in the CSS. */
const GAIN = 400

const TILE_COLORS: readonly TileColor[] = ["green", "yellow", "gray"]

/**
 * Play a submitted guess's events over the board already on screen. Resolves
 * when the sequence ends, or at once after a tap anywhere on the screen.
 */
export async function playScoring(events: readonly GameEvent[], ctx: SceneContext): Promise<void> {
  const { screen, state } = ctx

  screen.addEventListener("pointerdown", skip)

  const row = screen.querySelector(`.row[data-row="${state.round.guesses.length - 1}"]`)
  const tiles = [...(row?.querySelectorAll(".tile") ?? [])]
  for (const tile of tiles) tile.classList.add("pending")
  // Held back the same way the colors are, and released with the last of
  // them: the boss's summary of a row is only meaningful after the row it
  // summarizes has been seen, and it would otherwise be legible for the whole
  // length of the cascade it is the answer to.
  const note = row?.querySelector(".row-note")
  note?.classList.add("pending")
  // The keyboard is drawn from the committed state, so on Enter it already
  // wears the colors this row is about to reveal, one turn of the first tile
  // ahead of any tile showing its own. Both looks hold the row's letters back
  // at their previous color and let each one go as its tile turns.
  const held = holdKeys(screen, state)
  // One `finally` for both looks. The phone's cleanup used to be the last lines
  // of its own path, so a throw part-way left the row's tiles `pending`, the
  // boss's note hidden and the row's keys held at their old colours until the
  // next full render; the table's had a `finally` from the start.
  try {
    if (juiced()) await playTable(events, ctx, tiles, note, held)
    else await playPhone(events, ctx, tiles, note, held)
  } finally {
    // Belt and braces as well as cleanup: a guess that produced no tile events
    // at all, or a skip taken before the cascade reached the end, must not
    // leave the note hidden until the next full rebuild happens to drop it.
    for (const tile of tiles) tile.classList.remove("pending")
    note?.classList.remove("pending")
    held.releaseAll()
    screen.removeEventListener("pointerdown", skip)
  }
}

/** The phone's sequence: even steps, the readout and the total, no table effects. */
async function playPhone(
  events: readonly GameEvent[],
  ctx: SceneContext,
  tiles: readonly Element[],
  note: Element | null | undefined,
  held: Held,
): Promise<void> {
  const { screen, state, sound } = ctx
  const chipsEl = screen.querySelector(".readout .chips")
  const multEl = screen.querySelector(".readout .mult")
  const scoreEl = screen.querySelector(".hud .score")
  // The figures already on screen, so each step can react to the half that
  // moved rather than to both. Which half moved is the information: a gray
  // tile pays chips and nothing else, a green one pays both, and a player who
  // never sees the difference has to be told it in a tutorial instead.
  let shownChips = 0
  let shownMult = 1
  const readout = (chips: number, mult: number) => {
    if (chipsEl && chips !== shownChips) {
      chipsEl.textContent = num(chips)
      replay(chipsEl, "bumped", 320)
    }
    if (multEl && mult !== shownMult) {
      multEl.textContent = num(mult)
      replay(multEl, "bumped", 320)
    }
    shownChips = chips
    shownMult = mult
  }
  // Wound back for the same reason the total below it is: the board was drawn
  // after the guess was committed, so the readout starts out holding the
  // figure this is about to build up to.
  if (chipsEl) chipsEl.textContent = num(0)
  if (multEl) multEl.textContent = num(1)

  // The bar under the total is driven off the same numbers the count-up walks
  // through, so it fills in step with the digits instead of trailing them.
  const meterEl = screen.querySelector<HTMLElement>(".hud .meter-fill")
  const scoreBox = screen.querySelector(".hud-score")
  const target = state.round.target
  const meter = (value: number) => {
    meterEl?.style.setProperty("--fill", String(meterFill(value, target)))
    scoreBox?.classList.toggle("met", value >= target)
  }

  // The state was committed before any of this ran, so the HUD is already
  // showing the total the animation is about to build up to. Wind it back to
  // the pre-guess figure first, or the reveal spoils its own punchline.
  const scored = events.find((event) => event.type === "guess_scored")
  if (scoreEl && scored) scoreEl.textContent = num(scored.total - scored.score)
  /** The figure on screen, so the solve bonus knows what it is multiplying. */
  let onScreen = scored ? scored.total - scored.score : state.round.score
  meter(onScreen)

  // How many things have fired so far this guess. The trigger cue climbs a
  // rung for each, so a long chain builds instead of repeating one blip.
  let fired = 0
  for (const event of events) {
    switch (event.type) {
      case "tile": {
        const tile = tiles[event.index]
        reveal(tile, event.index, ctx)
        held.reveal(event.index, tile)
        // The badge and the counters say what this tile paid at the same moment
        // its color appears. Reduced motion has no turn to wait for, and a skip
        // should not leave a timer writing an older tile over the finished row.
        const paid = () => {
          tileGain(tile, event.gained)
          readout(event.chips, event.mult)
        }
        if (skipping() || reduced()) paid()
        else setTimeout(paid, ms(FLIP.half))
        if (event.index === tiles.length - 1) revealNote(note)
        await step(PACE.tile)
        break
      }
      case "mod": {
        // The tile itself lights up rather than a card in the tray: the thing
        // that fired is the letter, and it is already on screen.
        const tile = tiles[event.index]
        tile?.classList.add("fired")
        floater(screen, payoutBadge(event.paid))
        sound.cue({ name: "trigger", kind: "mod", n: fired++ })
        readout(event.chips, event.mult)
        await step(PACE.relic)
        tile?.classList.remove("fired")
        break
      }
      case "relic": {
        const slot = screen.querySelector(`.relic[data-slot="${event.slot}"]`)
        slot?.classList.add("fired")
        floater(screen, payoutBadge(event.paid))
        sound.cue({ name: "trigger", kind: "relic", n: fired++ })
        readout(event.chips, event.mult)
        await step(PACE.relic)
        slot?.classList.remove("fired")
        break
      }
      case "category": {
        // Lights the line that was already naming this shape on the board, so
        // the label the player read before submitting is the thing that pays.
        const line = screen.querySelector(".category")
        line?.classList.add("fired")
        floater(screen, categoryLevel(event.id, event.level))
        sound.cue({ name: "trigger", kind: "category", n: fired++ })
        readout(event.chips, event.mult)
        await step(PACE.relic)
        line?.classList.remove("fired")
        break
      }
      case "relic_grew": {
        // Lands after the guess has finished scoring, because that is when it
        // happens: the round ended, and this card is worth more next time. No
        // readout, because nothing about this guess's chips or mult moved, which is
        // exactly what distinguishes growing from firing.
        const slot = screen.querySelector(`.relic[data-slot="${event.slot}"]`)
        slot?.classList.add("fired")
        floater(screen, growthBadge(event))
        sound.cue({ name: "trigger", kind: "grew", n: fired++ })
        await step(PACE.relic)
        slot?.classList.remove("fired")
        break
      }
      case "solve_bonus": {
        // Arrives after the guess has already been counted onto the total, so
        // this is the pile itself multiplying, the biggest number movement in
        // the game, and the one the whole round was building toward.
        floater(screen, ui().board.solveFactor(event.factor))
        screen.querySelector(".readout")?.classList.add("solved")
        sound.cue({ name: "solve" })
        roll(scoreEl, onScreen, event.total, { also: meter })
        emphasize(screen, event.total / Math.max(1, target))
        onScreen = event.total
        await step(PACE.solve)
        break
      }
      case "guess_scored": {
        // The single most important number in the game, so it is the one
        // thing that animates its value rather than snapping to it.
        const from = event.total - event.score
        roll(scoreEl, from, event.total, { also: meter })
        emphasize(screen, event.score / Math.max(1, target))
        sound.cue({ name: "score", ratio: event.score / Math.max(1, target) })
        onScreen = event.total
        await step(PACE.total)
        break
      }
      case "letter_destroyed":
        floater(screen, ui().board.letterBroken(event.letter))
        sound.cue({ name: "break" })
        await step(PACE.relic)
        break
      case "relic_destroyed":
        // No card to light: the screen was rebuilt from a tray that no longer
        // holds it. The floater names it instead, and the break sound is the
        // same one a letter makes, because it is the same kind of loss.
        floater(screen, ui().board.relicGone(relicCard(event.id).name))
        sound.cue({ name: "break" })
        await step(PACE.relic)
        break
      default:
        break
    }
  }
}

type Held = {
  /** The tile at `index` has begun to turn: its letter's key follows at the trough. */
  reveal: (index: number, tile: Element | undefined) => void
  /** Everything still held, now. Idempotent. */
  releaseAll: () => void
}

/**
 * Keeps the keyboard from answering the guess before the board does.
 *
 * Only the keys whose color this row actually changes are touched: a letter
 * already green from an earlier guess has nothing to spoil, and an eliminated
 * one (a consumable's doing, gray on the key from the start) is left alone.
 * Each is put back to what it wore before the guess, and given its new color
 * at the same trough its tile shows its own, keyed on the tile whose color the
 * key is going to be: a letter played twice, gray and then green, must not
 * light its key on the gray one.
 */
function holdKeys(screen: HTMLElement, state: RunState): Held {
  const guesses = state.round.guesses
  const guess = guesses[guesses.length - 1]
  const held = new Map<string, { key: HTMLElement; final: Color; before: Color | undefined }>()
  const scheduled = new Set<string>()
  if (guess) {
    const before = keyboardColors(guesses.slice(0, -1))
    const eliminated = new Set(state.round.eliminated)
    const keys = new Map<string, HTMLElement>()
    for (const key of screen.querySelectorAll<HTMLElement>(".keyboard .key:not(.wide)")) {
      const letter = key.firstChild?.textContent?.toLowerCase()
      if (letter) keys.set(letter, key)
    }
    for (const { letter } of guess.tiles) {
      const key = keys.get(letter)
      if (!key || held.has(letter) || eliminated.has(letter)) continue
      const final = TILE_COLORS.find((name) => key.classList.contains(name))
      const was = before.get(letter)
      if (!final || final === was) continue
      key.classList.remove(final)
      if (was) key.classList.add(was)
      held.set(letter, { key, final, before: was })
    }
  }

  const release = (letter: string) => {
    const entry = held.get(letter)
    if (!entry) return
    held.delete(letter)
    if (entry.before) entry.key.classList.remove(entry.before)
    entry.key.classList.add(entry.final)
    if (juiced() && !skipping()) tween(entry.key, KEY_POP, { duration: 320, easing: "linear" })
  }

  return {
    reveal(index, tile) {
      const letter = guess?.tiles[index]?.letter
      const entry = letter === undefined ? undefined : held.get(letter)
      if (letter === undefined || !entry || scheduled.has(letter)) return
      if (!tile?.classList.contains(entry.final)) return
      scheduled.add(letter)
      if (skipping() || reduced()) release(letter)
      else setTimeout(() => release(letter), ms(FLIP.half))
    },
    releaseAll() {
      for (const letter of [...held.keys()]) release(letter)
    },
  }
}

/** A key taking its color: a pop with a little overshoot, table only. */
const KEY_POP: Keyframe[] = [
  { scale: "1", easing: "cubic-bezier(0.34, 1.56, 0.64, 1)" },
  { scale: "1.22", offset: 0.35, easing: "cubic-bezier(0.34, 1.56, 0.64, 1)" },
  { scale: "1" },
]

/**
 * Lets the row's note in at the trough of the last tile's turn, which is the
 * moment that tile's color appears. Same timing as `tileGain`, for the same
 * reason: the answer and the thing it is an answer to arrive together.
 */
function revealNote(note: Element | null | undefined): void {
  if (!note) return
  if (skipping() || reduced()) {
    note.classList.remove("pending")
    return
  }
  setTimeout(() => note.classList.remove("pending"), ms(FLIP.half))
}

/**
 * Wordle's turn-over, done with a scale rather than a pair of stacked faces:
 * the color is swapped at the trough, where the tile is edge-on and there is
 * nothing to see, which is the whole trick.
 */
function reveal(tile: Element | undefined, index: number, ctx: SceneContext): void {
  if (!tile) return
  const color = TILE_COLORS.find((name) => tile.classList.contains(name)) ?? "gray"
  ctx.sound.cue({ name: "tile", index, color })

  if (skipping() || reduced()) {
    tile.classList.remove("pending")
    return
  }

  tile.classList.add("flip")
  // Timers rather than awaits: the flips are meant to overlap, so this one
  // must keep running while the next tile starts. Both are harmless if the
  // screen is replaced first, since the node is simply detached by then.
  setTimeout(() => tile.classList.remove("pending"), ms(FLIP.half))
  setTimeout(() => tile.classList.remove("flip"), ms(FLIP.total))
}

/**
 * What a tile just paid, said at the tile.
 *
 * Every other effect in the game announces itself. A relic lights up and
 * floats its number, a modifier lights the letter it rode in on, and the
 * tiles, which are where most of a guess actually comes from, said nothing.
 * The readout moved and the player was left to infer which of the five
 * letters had moved it.
 *
 * Both halves are named, and the chips half is named even when it is zero,
 * because a zero is the whole point under a boss that stops paying for
 * vowels. The mult half only appears when there is one: gray pays no
 * multiplier, and its silence next to a green tile's `+3 mult` is the clearest
 * statement of the rule this game has.
 *
 * The badge hangs off the row and is placed from the tile's own box, which is
 * the same lesson learned twice: a child of the tile would be scaled edge-on
 * by the very flip it is announcing, and a grid item, even one placed
 * explicitly into the tile's cell, perturbs the auto-placement of the five
 * tiles around it and wraps the row. Absolute, off measurements, disturbs
 * neither. It now lasts two tile beats, instead of 760ms, so the first payment
 * leaves as the third arrives rather than all five hanging over the board
 * together at the end of the cascade.
 */
function tileGain(tile: Element | undefined, chips: number): void {
  const row = tile?.parentElement
  if (!(tile instanceof HTMLElement) || !row) return
  const color = TILE_COLORS.find((name) => tile.classList.contains(name))
  const mult = MULT_FOR_COLOR[color ?? "gray"]

  const show = () => {
    const node = document.createElement("div")
    node.className = "tile-gain"
    node.style.left = `${tile.offsetLeft + tile.offsetWidth / 2}px`
    // Starts just inside the tile rather than above it, so it is unambiguously
    // this tile's number before it rises away over the row above.
    node.style.top = `${tile.offsetTop + tile.offsetHeight * 0.18}px`
    node.append(h("span", { class: "gain-chips" }, `+${chips}`))
    if (mult > 0) node.append(h("span", { class: "gain-mult" }, `+${mult}`))
    row.append(node)
    setTimeout(() => node.remove(), ms(GAIN))
  }

  // Skipping runs the whole guess at once, so the badges would all land
  // together and then all expire together, which is noise rather than a
  // breakdown. The player asked for the end; give them it.
  if (skipping()) return
  show()
}

/**
 * Weight of the reaction, scaled by what the guess was worth against the
 * target. A chip guess twitches; a guess that clears the round on its own
 * shakes the screen, in the Smoke Room only.
 *
 * The shake is the one piece of this scene that asks which look is on. It ran
 * under every look, since the phone's sequence is also what Classic and
 * Tabletop play on the table, and the owner had it out of both: Tabletop is
 * the look that "holds still" (see `Fx` in `skin.ts`) and was the only thing
 * on it jolting, and Classic is the plain board. The pop on the readout stays
 * everywhere, because it is the reaction the ratio is for.
 */
function emphasize(screen: HTMLElement, ratio: number): void {
  const readout = screen.querySelector(".readout")
  readout?.classList.remove("popped")
  void (readout as HTMLElement | null)?.offsetWidth
  if (readout instanceof HTMLElement) {
    // Up to 1.2, from 1.5. The blocks live in the header now, between the bar
    // and the boss's band, and at 1.5 a 103px block grew 26px a side over
    // both; at 1.2 a round-clearing guess still lunges, into the gaps only.
    readout.style.setProperty("--pop", String(1 + Math.min(0.2, ratio * 0.25)))
    readout.classList.add("popped")
  }
  if (ratio < 0.5 || reduced() || !hasFx()) return
  screen.style.setProperty("--shake", `${Math.min(8, 3 + ratio * 4).toFixed(1)}px`)
  screen.classList.add("shaking")
  setTimeout(() => screen.classList.remove("shaking"), ms(420))
}

function floater(screen: HTMLElement, text: string): void {
  const host = screen.querySelector(".readout")
  if (!host) return
  const node = document.createElement("div")
  node.className = "floater"
  node.textContent = text
  host.append(node)
  setTimeout(() => node.remove(), ms(900))
}

/* ---------------------------------------------------------------------------
 * The table's version of the show.
 *
 * Everything above this line is the phone's and is what a phone plays; nothing
 * below it is reachable unless `juiced()`. The two share the events, the sound
 * cues and the pending/flip contract with the stylesheet (a tile's color is on
 * before it is shown and comes off `.pending` at the trough), and nothing else.
 *
 * Three rules keep it honest:
 *
 * - Every keyframe list here carries its own easing and every tween runs at
 *   `linear`. WAAPI's option easing warps the whole iteration, not each
 *   segment, and with the default ease-out a keyframe authored at offset 0.6
 *   is reached at about a fifth of the way through the clock. The collisions
 *   below are timed against the clock, so the segments must be too.
 * - Individual transform properties (`translate`, `scale`, `rotate`), never
 *   `transform`: the flip owns `transform` on a tile and the pop owns it on a
 *   readout block, and the individual ones compose with those where a second
 *   `transform` would replace them.
 * - Whatever it puts in `#fx` it tracks, and removes when the show ends,
 *   whether that was by running out or by a tap. `tween` finishes on a skip
 *   but only the ones already running, so nothing here starts a tween, a burst
 *   or a timer after `skipping()` turns true.
 * ------------------------------------------------------------------------ */

/**
 * The gap after a tile. Longer than the phone's 170 by exactly enough that a
 * tile's readout update, which waits for its color to appear, lands before the
 * next event's: 190 is the trough, and a mod firing 170 after the last tile
 * would otherwise write its cumulative figure and then be overwritten by the
 * tile's older one. Flips (380) still overlap by half, so it is still a
 * cascade and not a queue.
 */
const TILE_GAP = 200
/** Authored ms from the finale starting to the pile being swept into the total. */
const LUNGE_HIT = 300
/** The solve step, longer than the phone's 900 to let the line finish leaving. */
const SOLVE_HOLD = 1250
/** The solve's impact delay: the sweep of the payline that once ran the row, kept so the sound leads the sparks. */
const PAYLINE_SWEEP = 300
/** Chips the stack under the readout will show; the rest still land, on top. */
const STACK_MAX = 10
/** One chip's thickness in px, which is also the pitch of the stack. */
const CHIP_PITCH = 7

const BACK = "cubic-bezier(0.34, 1.56, 0.64, 1)"
const OUT = "cubic-bezier(0.23, 1, 0.32, 1)"

/**
 * A table token as a string, for the canvas: particles are painted with plain
 * colors and cannot read a custom property, so the palettes are resolved from
 * `:root.table` once per show and follow the stylesheet when it changes. The
 * fallback is the token's own value and only matters in a test or a stripped page.
 */
const token = (name: string, fallback: string) =>
  getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback

const GLASS = ["#dfe8ff", "#9fb4d9", "#ffffff", "#7fd6ff"]

/**
 * The score's ink as it heats: the foreground, the accent, the cash colour, then
 * the multiplier's red. Four stops on `heat`, interpolated by hand because WAAPI cannot tween through a
 * `color-mix` and a per-frame `style.color` wants a plain string. The last
 * stop is the red the multiplier wears, which is what the number is turning
 * into.
 */
const HEAT_STOPS: readonly (readonly [number, number, number, number])[] = [
  [0, 235, 231, 223],
  [0.35, 155, 232, 216],
  [0.7, 216, 181, 99],
  [1, 255, 90, 100],
]

function heatColor(x: number): string {
  const t = Math.max(0, Math.min(1, x))
  for (let i = 1; i < HEAT_STOPS.length; i++) {
    const [t1, r1, g1, b1] = HEAT_STOPS[i] as readonly number[] as [number, number, number, number]
    if (t > t1 && i < HEAT_STOPS.length - 1) continue
    const [t0, r0, g0, b0] = HEAT_STOPS[i - 1] as readonly number[] as [
      number,
      number,
      number,
      number,
    ]
    const k = Math.max(0, Math.min(1, (t - t0) / (t1 - t0)))
    const mix = (a: number, b: number) => Math.round(a + (b - a) * k)
    return `rgb(${mix(r0, r1)} ${mix(g0, g1)} ${mix(b0, b1)})`
  }
  return "rgb(255 90 140)"
}

type Tone = "chips" | "mult" | "gold" | "dim" | "words"

const toneOf = (paid: Payout): Tone => {
  switch (paid.kind) {
    case "chips":
      return "chips"
    case "mult":
    case "times":
      return "mult"
    case "gold":
      return "gold"
    default:
      return "dim"
  }
}

async function playTable(
  events: readonly GameEvent[],
  ctx: SceneContext,
  tiles: readonly Element[],
  note: Element | null | undefined,
  held: Held,
): Promise<void> {
  const { screen, state, sound } = ctx
  const target = state.round.target

  // What the show has put on the table, so the end of it can take it all off.
  const made = new Set<HTMLElement>()
  const timers = new Set<ReturnType<typeof setTimeout>>()
  const restore: (() => void)[] = []

  const bulb = token("--lit", "#58c9b3")
  const brass = token("--coin", "#d8b563")
  const brassHi = token("--coin-hi", "#f0dc9c")
  const ivory = token("--paper", "#ebe7df")
  const GOLDS = [bulb, brassHi, brass]
  /** A dead placard's shards: the plate's own dark and the paper gone dull. */
  const ASH = [
    token("--paper-lo", "#858b98"),
    token("--panel", "#0c0e13"),
    token("--line", "#1d212b"),
  ]
  const sparksFor = (tone: Tone): readonly string[] => {
    switch (tone) {
      case "chips":
        return [ivory, token("--chips", "#3f9dff")]
      case "mult":
        return [ivory, token("--mult", "#ff4f58")]
      case "dim":
        return [ivory, token("--paper-lo", "#858b98")]
      default:
        return GOLDS
    }
  }

  const chain = new Chain()
  const fx = (kind: Kind, at: { x: number; y: number }, options: Parameters<typeof burst>[2]) => {
    if (!skipping()) burst(kind, at, options)
  }
  const tw = (node: Element | null | undefined, frames: Keyframe[], duration: number, delay = 0) =>
    skipping() ? Promise.resolve() : tween(node, frames, { duration, delay, easing: "linear" })
  /** Run `fn` after an authored delay, or right now if the player has skipped. */
  const later = (fn: () => void, authored: number) => {
    if (skipping()) {
      fn()
      return
    }
    const id = setTimeout(() => {
      timers.delete(id)
      fn()
    }, ms(authored))
    timers.add(id)
  }

  /**
   * A number or a word popped at a point, in `#fx` so the render at either
   * end of the show cannot delete it, and so it can leave the box of whatever
   * it came from: a relic's payout used to be drawn in the readout, which
   * read as the readout paying rather than the card.
   */
  const pop = (
    tone: Tone,
    text: string,
    at: { x: number; y: number },
    { rise = 56, drift = 0, scale = 1, delay = 0, bare = false } = {},
  ) => {
    if (skipping()) return
    // `bare` is the figure alone, in its tone's colour, with no tag behind it:
    // a relic's payout, which sits over a card that is already lit.
    const node = h("div", { class: `fx-pop ${tone}${bare ? " bare" : ""}` }, text)
    made.add(node)
    const dx = `${drift}px`
    const run = tw(
      node,
      [
        { opacity: 0, transform: `translate(${dx}, 10px) scale(0.3)`, easing: BACK },
        {
          opacity: 1,
          transform: `translate(${dx}, ${-rise * 0.3}px) scale(${1.3 * scale})`,
          offset: 0.2,
        },
        {
          opacity: 1,
          transform: `translate(${dx}, ${-rise * 0.55}px) scale(${scale})`,
          offset: 0.62,
          easing: OUT,
        },
        { opacity: 0, transform: `translate(${dx}, ${-rise}px) scale(${0.9 * scale})` },
      ],
      780,
      delay,
    )
    place(node, at, run)
    run.then(() => made.delete(node))
  }

  const above = (node: Element, lift = 0) => {
    const rect = node.getBoundingClientRect()
    return { x: rect.left + rect.width / 2, y: Math.max(30, rect.top - lift) }
  }

  // ---- the readout ------------------------------------------------------
  const chipsEl = screen.querySelector<HTMLElement>(".readout .chips")
  const multEl = screen.querySelector<HTMLElement>(".readout .mult")
  const scoreEl = screen.querySelector<HTMLElement>(".hud .score")
  const scoreBox = screen.querySelector<HTMLElement>(".hud-score")
  const meterEl = screen.querySelector<HTMLElement>(".hud .meter-fill")
  let shownChips = 0
  let shownMult = 1

  /**
   * A block taking a hit: a scale punch and a flash, and nothing that turns. It
   * used to kick one way and ring back the other, which is a card being flicked;
   * a sign being struck just brightens and settles. `power` grows with the
   * chain, so the tenth trigger lands harder than the first.
   */
  const hit = (el: HTMLElement | null, power: number) => {
    if (!el) return
    const p = Math.min(1.6, power)
    void tw(
      el,
      [
        { scale: "1", filter: "brightness(1)", easing: OUT },
        { scale: String(1 + 0.13 * p), filter: "brightness(1.5)", offset: 0.25, easing: OUT },
        { scale: "0.98", filter: "brightness(1.1)", offset: 0.6, easing: OUT },
        { scale: "1", filter: "brightness(1)" },
      ],
      340,
    )
  }
  const readout = (chips: number, mult: number, power = 1) => {
    if (chipsEl && chips !== shownChips) {
      chipsEl.textContent = num(chips)
      hit(chipsEl, power)
    }
    if (multEl && mult !== shownMult) {
      multEl.textContent = num(mult)
      hit(multEl, power)
    }
    shownChips = chips
    shownMult = mult
  }
  // Wound back, as on the phone: the board was drawn after the commit.
  if (chipsEl) chipsEl.textContent = num(0)
  if (multEl) multEl.textContent = num(1)

  const meter = (value: number) => {
    meterEl?.style.setProperty("--fill", String(meterFill(value, target)))
    scoreBox?.classList.toggle("met", value >= target)
  }
  const scored = events.find((event) => event.type === "guess_scored")
  if (scoreEl && scored) scoreEl.textContent = num(scored.total - scored.score)
  let onScreen = scored ? scored.total - scored.score : state.round.score
  meter(onScreen)

  // ---- the chip stack -----------------------------------------------------
  /**
   * Chips are chips: every `+chips` is a token dropped from where it was earned
   * onto a stack that grows in the rail under the block, and the stack is what
   * is swept into the total at the finale. Ellipses from above, each with a lip
   * of its own colour for thickness, so a stack reads as a stack and not as a
   * column of pills. They are DOM in `#fx` and not particles because each is
   * placed, tracked and removed by hand; the cap keeps the count at ten.
   */
  const stack: HTMLElement[] = []
  const stackSpot = (n: number) => {
    const rect = chipsEl?.getBoundingClientRect()
    if (!rect) return null
    // Deterministic lean, so a replay of the same guess stacks the same way.
    const lean = ((n * 37) % 7) - 3
    return {
      x: rect.left + rect.width / 2 + lean,
      y: rect.bottom + 16 + (STACK_MAX - Math.min(n, STACK_MAX - 1)) * CHIP_PITCH,
    }
  }
  const drop = (from: { x: number; y: number }, gained: number) => {
    if (skipping()) return
    const land = stackSpot(stack.length)
    if (!land) return
    const keep = stack.length < STACK_MAX
    const chip = h("div", { class: `fx-chip${gained >= 10 ? " big" : ""}` })
    chip.style.left = `${land.x}px`
    chip.style.top = `${land.y}px`
    fxDom().append(chip)
    made.add(chip)
    if (keep) stack.push(chip)
    const dx = from.x - land.x
    const dy = from.y - land.y
    const run = tw(
      chip,
      [
        { opacity: 0, transform: `translate(${dx}px, ${dy}px) scale(0.55)`, easing: "ease-in" },
        { opacity: 1, transform: "translate(0px, 0px) scale(1)", offset: 0.58, easing: OUT },
        { opacity: 1, transform: "translate(0px, -7px) scale(1)", offset: 0.76, easing: "ease-in" },
        { opacity: 1, transform: "translate(0px, 0px) scale(1)" },
      ],
      440,
    )
    run.then(() => {
      if (keep) return
      chip.remove()
      made.delete(chip)
    })
  }

  // ---- keys and letters -------------------------------------------------
  const keyFor = (letter: string) =>
    [...screen.querySelectorAll<HTMLElement>(".keyboard .key:not(.wide)")].find(
      (key) => key.firstChild?.textContent?.toLowerCase() === letter,
    )
  // A broken letter's key is already drawn broken by the render, which would
  // spend the loss before the shards fly. Put back when its event plays.
  for (const event of events) {
    if (event.type !== "letter_destroyed") continue
    const key = keyFor(event.letter)
    if (key?.classList.contains("broken")) {
      key.classList.remove("broken")
      restore.push(() => key.classList.add("broken"))
    }
  }

  // ---- tiles ------------------------------------------------------------
  // The relic's nudge, on a tile. It was a five-swing wobble from -11deg at
  // 1.2x over 560ms, under a filled 1-1.3x badge and 9 sparks; the relics were
  // taken down to one swing each way and a bare figure, and the owner asked for
  // a modifier firing to match, so the two ways a guess gets paid extra now
  // look like one gesture on two kinds of thing.
  const wobbleTile = (tile: Element) =>
    tw(
      tile,
      [
        { rotate: "0deg", scale: "1", easing: OUT },
        { rotate: "-3deg", scale: "1.05", offset: 0.2, easing: OUT },
        { rotate: "1.5deg", offset: 0.5, easing: OUT },
        { rotate: "0deg", scale: "1" },
      ],
      420,
    )

  /**
   * Lift, turn, land. The lift and the landing squash are `translate` and
   * `scale`, the turn is the stylesheet's `.flip` on `transform`, so the
   * three run together; the lift peaks a little before the trough, where the
   * color arrives, and the squash lands just after it.
   */
  const flipTile = (tile: Element, index: number) => {
    const color = TILE_COLORS.find((name) => tile.classList.contains(name)) ?? "gray"
    sound.cue({ name: "tile", index, color })
    if (skipping()) {
      tile.classList.remove("pending")
      return
    }
    tile.classList.add("flip")
    setTimeout(() => tile.classList.remove("pending"), ms(FLIP.half))
    setTimeout(() => tile.classList.remove("flip"), ms(FLIP.total))
    void tw(
      tile,
      [
        { translate: "0 0", scale: "1", easing: OUT },
        { translate: "0 -0.9rem", scale: "1.1", offset: 0.4, easing: "ease-in" },
        { translate: "0 0.12rem", scale: "0.93", offset: 0.68, easing: OUT },
        { translate: "0 0", scale: "1" },
      ],
      440,
    )
  }

  /**
   * The color has appeared: hit the readout. It used to pop a `+chips` and a
   * `+mult` tag off every tile as well, rising out of the row, and the owner
   * asked for them gone: five dark-inked tags climbing off a word read as smoke
   * coming off the board, not as sums, and the readout they fed says the same
   * figures a beat later with its own hit. The relic's, the category's and the
   * total's pops stay, since nothing else says those.
   *
   * No sparks and no shake either. A green tile threw 12 sparks and added 0.05
   * of trauma, a yellow one 5 sparks, and the owner saw the flips jitter. The
   * shake is the likelier culprit: trauma is squared, so 0.05 a tile moves the
   * screen by fractions of a pixel and turns it by thousandths of a degree, and
   * a screen that is turned at all is resampled whole, every tile in the middle
   * of its flip with it. The sparks went at the owner's asking; the shake went
   * with them because it was the same gesture and the one that could do that.
   * The colour landing on the tile is the event, and it needs no help.
   */
  const landTile = (tile: Element, event: Extract<GameEvent, { type: "tile" }>) => {
    const color = TILE_COLORS.find((name) => tile.classList.contains(name)) ?? "gray"
    const mult = MULT_FOR_COLOR[color]
    const at = centerOf(tile)
    const top = above(tile, -6)
    if (event.gained > 0) drop({ x: at.x - (mult > 0 ? 22 : 0), y: top.y }, event.gained)
    readout(event.chips, event.mult, 0.7)
  }

  // ---- the total --------------------------------------------------------
  const boxWidth = () => (scoreBox ? scoreBox.clientWidth * 0.86 : 0)
  const baseFont = scoreEl ? Number.parseFloat(getComputedStyle(scoreEl).fontSize) || 56 : 56

  /**
   * Count the pile from `from` to `to`, its digits growing and heating with
   * how much of `x` (the heat of what is being added) has arrived. Capped so
   * the digits never outgrow the plate: a seven-figure total at 1.4x is wider
   * than the rail, and a number clipped at its edge is a wrong number.
   */
  let shown = ""
  /**
   * The odometer: each digit that changes since the last frame is a fresh
   * cell that drops in from above, the way a wheel clicks over, and the
   * digits that have not moved stay put. The low wheels turn every frame and
   * the high ones once in a long while, which is what the ease-out already
   * does to the number, so the mechanism only has to draw it. Cells are
   * aligned from the right, since a wheel belongs to a place, not to a
   * position in the string.
   */
  const odometer = (el: HTMLElement) => {
    const text = el.textContent ?? ""
    if (skipping() || text === shown) return
    const before = shown
    shown = text
    const cells = [...text].map((ch, i) => {
      const cell = h("span", { class: "d" }, ch)
      const was = before[before.length - (text.length - i)]
      if (was !== undefined && was !== ch && /\d/.test(ch)) {
        cell.animate(
          [
            { transform: "translateY(-45%)", opacity: 0.25 },
            { transform: "translateY(0)", opacity: 1 },
          ],
          { duration: ms(110), easing: "ease-out" },
        )
      }
      return cell
    })
    el.replaceChildren(...cells)
  }
  const climb = (from: number, to: number, x: number) => {
    if (!scoreEl) return
    const el = scoreEl
    let done = false
    shown = el.textContent ?? ""
    const paint = (value: number) => {
      odometer(el)
      meter(value)
      const p = to === from ? 1 : Math.max(0, Math.min(1, (value - from) / (to - from)))
      const level = x * p
      el.style.color = heatColor(level)
      const digits = num(value).length
      const cap = boxWidth() / Math.max(1, digits * 0.66 * baseFont)
      el.style.scale = String(Math.max(1, Math.min(1 + 0.42 * level, cap)))
      if (p < 1 || done) return
      done = true
      const grown = Math.max(1, Math.min(1 + 0.42 * x + 0.12, cap))
      el.style.removeProperty("scale")
      void tw(el, [{ scale: String(grown), easing: BACK }, { scale: "1" }], 420)
      // Held hot for the beat it takes to read, then cooled to the plate's own
      // ink, which is green if the target was met.
      later(() => {
        const hot = el.style.color
        el.style.removeProperty("color")
        const cool = getComputedStyle(el).color
        void tw(el, [{ color: hot, easing: OUT }, { color: cool }], 380)
      }, 300)
    }
    roll(el, from, to, { span: 340 + 520 * x, also: paint })
  }

  /**
   * Whether the bell has rung this show; see `reached`. The score well had a
   * ring of its own here, a dotted frame that stepped round it as the count-up
   * climbed, quicker with the heat, and went solid at the target. It used to set
   * the box on fire before that. The owner wanted the border gone altogether:
   * the count-up's colour and scale already say how hot the hand is.
   */
  let rung = false

  /**
   * The bell: the target has been met. AUDIO PHASE: this is the one place to
   * ring it, `over` being how many targets the total now is (1 exactly at the
   * line), so the bell can be bigger for a hand that cleared it by a mile.
   * Until that phase the bell is only seen, as the score going the solved
   * colour, and there is nothing for this to do.
   */
  const ringBell = (_over: number) => {}
  /**
   * Target met: the bell, and that is all. A payline ran under the plate here
   * and a fountain of coins (14 to 54, by heat) went up from it; the owner found
   * the two, on top of the solve's own stamp, too much commotion for a moment
   * the count-up already announces.
   */
  const reached = (_x: number, over: number) => {
    // Once a show: a solve that lands on a total already over the target is
    // the same bell, not a second one.
    if (rung) return
    rung = true
    ringBell(over)
  }

  // ---- the events -------------------------------------------------------
  let fired = 0
  try {
    for (const event of events) {
      switch (event.type) {
        case "tile": {
          const tile = tiles[event.index]
          if (!tile) break
          flipTile(tile, event.index)
          held.reveal(event.index, tile)
          later(() => landTile(tile, event), FLIP.half)
          if (event.index === tiles.length - 1) revealNote(note)
          await step(TILE_GAP)
          break
        }
        case "mod": {
          const tile = tiles[event.index]
          tile?.classList.add("fired")
          if (tile) {
            void wobbleTile(tile)
            const tone = toneOf(event.paid)
            pop(tone, payoutBadge(event.paid), above(tile, 10), {
              rise: 16,
              scale: 0.9 + Math.min(0.15, chain.length * 0.02),
              bare: true,
            })
            fx("spark", centerOf(tile), {
              count: 5,
              speed: 200,
              life: 0.35,
              size: 2.5,
              colors: sparksFor(tone),
            })
          }
          sound.cue({ name: "trigger", kind: "mod", n: fired++ })
          readout(event.chips, event.mult, 0.8 + chain.length * 0.06)
          await chain.next(PACE.relic)
          tile?.classList.remove("fired")
          break
        }
        case "relic": {
          const slot = screen.querySelector(`.relic[data-slot="${event.slot}"]`)
          slot?.classList.add("fired")
          if (slot) {
            const tone = toneOf(event.paid)
            // A nudge, not a wobble. It was a five-swing shake from -9deg at
            // 1.14x over 680ms, a 1.15-1.45x badge and 12 sparks, and with a
            // tray of five relics firing in a chain the rail never held still;
            // the owner asked for it smaller and quieter. One swing each way
            // at a third of the angle still says which card paid.
            void tw(
              slot,
              [
                { rotate: "0deg", scale: "1", easing: OUT },
                { rotate: "-3deg", scale: "1.05", offset: 0.2, easing: OUT },
                { rotate: "1.5deg", offset: 0.5, easing: OUT },
                { rotate: "0deg", scale: "1" },
              ],
              420,
            )
            // The badge is born on the card's top edge: this is the card paying.
            const at = above(slot, -14)
            pop(tone, payoutBadge(event.paid), at, {
              rise: 16,
              scale: 0.9 + Math.min(0.15, chain.length * 0.02),
              bare: true,
            })
            fx("spark", centerOf(slot), {
              count: 5,
              speed: 200,
              life: 0.35,
              size: 2.5,
              colors: sparksFor(tone),
            })
          }
          sound.cue({ name: "trigger", kind: "relic", n: fired++ })
          readout(event.chips, event.mult, 0.8 + chain.length * 0.06)
          await chain.next(PACE.relic)
          slot?.classList.remove("fired")
          break
        }
        case "category": {
          const line = screen.querySelector(".category")
          line?.classList.add("fired")
          if (line) {
            void tw(
              line,
              [
                { scale: "1", filter: "brightness(1)", easing: OUT },
                { scale: "1.07", filter: "brightness(1.8)", offset: 0.22, easing: OUT },
                { scale: "1.02", filter: "brightness(1.25)", offset: 0.55, easing: OUT },
                { scale: "1", filter: "brightness(1)" },
              ],
              460,
            )
            pop("words", categoryLevel(event.id, event.level), centerOf(line), { rise: 40 })
          }
          sound.cue({ name: "trigger", kind: "category", n: fired++ })
          readout(event.chips, event.mult, 0.8 + chain.length * 0.06)
          await chain.next(PACE.relic)
          line?.classList.remove("fired")
          break
        }
        case "relic_grew": {
          const slot = screen.querySelector(`.relic[data-slot="${event.slot}"]`)
          slot?.classList.add("fired")
          if (slot) {
            void tw(
              slot,
              [
                { scale: "1", translate: "0 0", easing: OUT },
                { scale: "1.05", translate: "0 -0.25rem", offset: 0.3, easing: OUT },
                { scale: "1", translate: "0 0" },
              ],
              380,
            )
            // Toned down with the payout above: it was a 1.16x hop on an
            // overshoot, a 1.1x badge and five coins.
            pop("gold", growthBadge(event), above(slot, -14), {
              rise: 16,
              scale: 0.9,
              bare: true,
            })
          }
          sound.cue({ name: "trigger", kind: "grew", n: fired++ })
          await chain.next(PACE.relic)
          slot?.classList.remove("fired")
          break
        }
        case "guess_scored": {
          const from = event.total - event.score
          const x = heat(event.score, target)
          // The blocks pull back, then lunge together and meet in the gap
          // between them. What they were nudged by all guess long comes to a
          // point here, and the point is where the shake goes.
          if (chipsEl && multEl && !skipping()) {
            const a = chipsEl.getBoundingClientRect()
            const b = multEl.getBoundingClientRect()
            const reach = Math.max(0, b.left - a.right) / 2 + 6 + 10 * x
            const pull = 8 + 8 * x
            const across = (dir: 1 | -1) => [
              { translate: "0 0", scale: "1", easing: OUT },
              { translate: `${-dir * pull}px 0`, scale: "0.95", offset: 0.34, easing: "ease-in" },
              {
                translate: `${dir * reach}px 0`,
                scale: String(1.1 + 0.1 * x),
                offset: 0.6,
                easing: OUT,
              },
              { translate: `${dir * reach * 0.15}px 0`, scale: "1.02", offset: 0.82, easing: OUT },
              { translate: "0 0", scale: "1" },
            ]
            void tw(chipsEl, across(1), 500)
            void tw(multEl, across(-1), 500)
          }
          // A payline used to run the row here while the blocks pulled back,
          // and another, thicker, on a solve. Over a word just entered a line
          // straight through it read as the word struck out, and the owner
          // asked for it gone; the one under the score plate went later with
          // the coins (see `reached`).
          later(() => {
            // Sparks went off where the blocks met, 14 to 60 by heat, and a
            // white shockwave ring (90 to 320px) with them. The ring went
            // first, the owner finding the sparks enough on their own, and
            // then the sparks: the lunge and the shake carry the hit.
            shake(0.16 + 0.78 * x)
            sound.cue({ name: "score", ratio: event.score / Math.max(1, target) })
            if (scoreBox) {
              void tw(
                scoreBox,
                [
                  { scale: "1", filter: "brightness(1.8)", easing: OUT },
                  {
                    scale: String(1.03 + 0.05 * x),
                    filter: "brightness(1.3)",
                    offset: 0.3,
                    easing: OUT,
                  },
                  { scale: "1", filter: "brightness(1)" },
                ],
                420,
              )
            }
            climb(from, event.total, x)
            if (event.total >= target) reached(x, event.total / Math.max(1, target))
          }, LUNGE_HIT)
          onScreen = event.total
          // The climb's span is only known once it has started, so its share of
          // the wait is its worst case: 340 + 520 at full heat, from the heat.
          await step(LUNGE_HIT + 340 + 520 * x + 420)
          break
        }
        case "solve_bonus": {
          const x = heat(event.total, target)
          const box = screen.querySelector(".grid-wrap")
          const at = box ? centerOf(box) : { x: innerWidth / 2, y: innerHeight / 2 }
          // A sign lit over the board here, JACKPOT over the factor on a plate
          // of its own, scaled in from 0.3 and held for 1.76s. The owner had it
          // out at once: the readout going to cash and the sparks already say
          // the word was solved, and the factor is in the total.
          sound.cue({ name: "solve" })
          later(() => {
            // Impact: the pile is swept into the total. A shockwave ring (520px)
            // and a spill of 70 coins went off here as well, beside the sparks,
            // the shake and a JACKPOT sign; the owner cut all three but the sparks
            // and the shake.
            fx("spark", at, { count: 60, speed: 760, life: 0.6, size: 4.5, colors: GOLDS })
            shake(0.95)
            screen.querySelector(".readout")?.classList.add("solved")
            hit(chipsEl, 1.5)
            hit(multEl, 1.5)
            climb(onScreen, event.total, x)
            onScreen = event.total
            if (event.total >= target) reached(x, event.total / Math.max(1, target))
          }, PAYLINE_SWEEP)
          await step(SOLVE_HOLD)
          break
        }
        case "letter_destroyed": {
          const key = keyFor(event.letter)
          const row = tiles[0]?.parentElement
          const tile = [...(row?.querySelectorAll(".tile") ?? [])].find(
            (node) => node.firstChild?.textContent?.toLowerCase() === event.letter,
          )
          key?.classList.add("broken")
          if (key) {
            void tw(
              key,
              [
                { scale: "1.3", rotate: "-8deg", filter: "brightness(2.2)", easing: OUT },
                {
                  scale: "0.9",
                  rotate: "5deg",
                  filter: "brightness(1.2)",
                  offset: 0.4,
                  easing: OUT,
                },
                { scale: "1", rotate: "0deg", filter: "brightness(1)" },
              ],
              480,
            )
            fx("shard", centerOf(key), { count: 22, speed: 520, life: 0.9, size: 7, colors: GLASS })
            pop("dim", ui().board.letterBroken(event.letter), above(key, 6), { rise: 46 })
          }
          if (tile)
            fx("shard", centerOf(tile), {
              count: 16,
              speed: 420,
              life: 0.8,
              size: 6,
              colors: GLASS,
            })
          shake(0.3)
          sound.cue({ name: "break" })
          await step(PACE.relic + 100)
          break
        }
        case "relic_destroyed": {
          // The tray was rebuilt without it, so what burns is a stand-in card
          // set where the tray ends: this is the loss being *shown*, and the
          // floater names which card it was.
          const name = relicCard(event.id).name
          const tray = screen.querySelector(".relics")
          if (tray && !skipping()) {
            const cards = [...tray.querySelectorAll(".relic:not(.empty)")]
            const last = cards[cards.length - 1]
            const trayRect = tray.getBoundingClientRect()
            const ref = (last ?? tray.querySelector(".relic"))?.getBoundingClientRect()
            const width = ref?.width ?? 96
            const height = ref?.height ?? trayRect.height
            const cx = last
              ? (ref?.right ?? trayRect.left) + 8 + width / 2
              : trayRect.left + width / 2
            const at = { x: cx, y: trayRect.top + height / 2 }
            const ghost = h("div", { class: "fx-placard" }, h("span", {}, name))
            ghost.style.width = `${width}px`
            ghost.style.height = `${height}px`
            made.add(ghost)
            // The placard's lamp goes out: a stutter of the bulb, then dark, and
            // the card sinks a hair as it dies. No fire, no spin: nothing
            // burned, the sign lost its power.
            const burn = async () => {
              await tw(
                ghost,
                [
                  { transform: "scale(1)", filter: "brightness(1)", easing: OUT },
                  { transform: "scale(1.04)", filter: "brightness(1.6)", offset: 0.18 },
                  { transform: "scale(1)", filter: "brightness(0.7)", offset: 0.3 },
                  { transform: "scale(1.02)", filter: "brightness(1.3)", offset: 0.42 },
                  {
                    transform: "scale(1)",
                    filter: "brightness(0.35) grayscale(0.8)",
                    offset: 0.58,
                  },
                  { transform: "scale(1)", filter: "brightness(0.2) grayscale(1)", offset: 0.8 },
                  {
                    transform: "scale(0.96) translateY(0.4rem)",
                    filter: "brightness(0.15) grayscale(1)",
                    opacity: 0,
                  },
                ],
                720,
              )
            }
            const done = burn()
            place(ghost, at, done)
            done.then(() => made.delete(ghost))
            later(() => {
              fx("shard", at, { count: 14, speed: 300, life: 0.8, size: 5, colors: ASH })
              shake(0.2)
            }, 380)
            pop(
              "dim",
              ui().board.relicGone(name),
              { x: at.x, y: Math.max(34, at.y - height / 2 + 10) },
              { rise: 26 },
            )
            sound.cue({ name: "break" })
            await done
          } else {
            sound.cue({ name: "break" })
          }
          await step(PACE.relic)
          break
        }
        default:
          break
      }
    }
  } finally {
    // Taken down whether the show ran out, was tapped through or threw. The canvas is
    // only wiped on a skip: what a finished show throws is meant to outlive it
    // over the reward screen, and what a skipped one throws would be a show the
    // player has said they do not want to watch.
    for (const id of timers) clearTimeout(id)
    timers.clear()
    for (const node of made) node.remove()
    made.clear()
    for (const undo of restore) undo()
    for (const el of [chipsEl, multEl]) {
      el?.style.removeProperty("translate")
      el?.style.removeProperty("scale")
      el?.style.removeProperty("rotate")
    }
    scoreEl?.style.removeProperty("scale")
    // The odometer's cells back to one plain text node, so the next render
    // and the next show start from the same shape the phone leaves behind.
    if (scoreEl) scoreEl.replaceChildren(document.createTextNode(scoreEl.textContent ?? ""))
    if (skipping()) clearParticles()
  }
}
