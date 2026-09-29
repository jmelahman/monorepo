import type { GameEvent } from "../../../engine"
import { MULT_FOR_COLOR } from "../../../engine"
import type { TileColor } from "../../audio"
import { h } from "../../dom"
import { formatNumber as num } from "../../format"
import { categoryLevel, growthBadge, payoutBadge, relicCard, ui } from "../../lang"
import { meterFill } from "../../views"
import { ms, reduced, replay } from "../motion"
import { roll } from "../numbers"
import { skip, skipping, step } from "../timeline"
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
 * These, and every other duration below that is a motion rather than a reading
 * time, are what the game is *authored* at. What it plays at is these divided by
 * the animation speed the player chose; `ms` is where that happens and
 * `../../speed` is why. Nothing here changes when the setting does.
 *
 * `solve` is the outlier on purpose: it is the last beat before the reward screen
 * takes the board away, and it has to outlast `COUNT_UP` by enough that the pile's
 * new total is legible standing still rather than glimpsed mid-climb.
 */
export const PACE = { tile: 170, relic: 150, solve: 900, total: 400 }

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
const GAIN = 760

const TILE_COLORS: readonly TileColor[] = ["green", "yellow", "gray"]

/**
 * Play a submitted guess's events over the board already on screen. Resolves
 * when the sequence ends, or at once after a tap anywhere on the screen.
 */
export async function playScoring(events: readonly GameEvent[], ctx: SceneContext): Promise<void> {
  const { screen, state, sound } = ctx

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
        // Held until the turn is half done, which is when the color appears.
        // Saying what the tile paid before showing what color it came up
        // would answer the question in the wrong order.
        tileGain(tile, event.gained)
        if (event.index === tiles.length - 1) revealNote(note)
        readout(event.chips, event.mult)
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

  for (const tile of tiles) tile.classList.remove("pending")
  // Belt and braces: a guess that produced no tile events at all, or a skip
  // taken before the cascade reached the end, must not leave the note hidden
  // until the next full rebuild happens to drop it.
  note?.classList.remove("pending")
  screen.removeEventListener("pointerdown", skip)
}

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
 * neither.
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
  // together and then all expire together, and five of them stacked on one row
  // is noise, not information. The player asked for the end; give them it.
  if (skipping()) return
  if (reduced()) show()
  else setTimeout(show, ms(FLIP.half))
}

/**
 * Weight of the reaction, scaled by what the guess was worth against the
 * target. A chip guess twitches; a guess that clears the round on its own
 * shakes the screen.
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
  if (ratio < 0.5 || reduced()) return
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
