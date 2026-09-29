import { centerOf, fxDom, stand } from "../layer"
import { juiced, tactile, tween } from "../motion"
import { burst } from "../particles"
import { shake } from "../shake"

/**
 * The board on the table: letters dropping in, backspace, a refused row, the
 * board dealt at the start of a round, and the keyboard answering a press.
 * Owned by phase 3 of `plans/ui-overhaul.md`; the stylesheet half is
 * `styles/table/board.css` and `keyboard.css`, and the split between them is
 * worth stating because it is the only design decision in here.
 *
 * The letter's own landing is CSS (`.tile.land`, which `patchDraft` puts on the
 * arriving tile), because a class the render already adds costs nothing to
 * answer and survives being interrupted by the next keystroke. What CSS cannot
 * do is the things that are not on the tile: the ghost of a letter that has
 * already been cleared, the wave that runs along a row once it is full, the dust
 * a chip kicks up, the tilt of the whole screen when a word is refused. Those
 * are here, all through `tween` and `burst` so that reduced motion, the pace
 * setting and tap-to-skip are honoured without this file asking.
 *
 * Every hook is called on the phone too, and returns at once there: check
 * `juiced()` first. The two listeners at the foot are bound once at import and
 * do the same.
 *
 * Nothing here animates `.grid`, its width or its font size. Tiles get
 * `transform`, `translate`, `opacity` and `filter`, and the row's wave rides on
 * `translate` so that it composes with the landing (`transform`) instead of
 * replacing it. See the long section of CLAUDE.md on the board flash before
 * adding anything that measures or resizes.
 */

/**
 * What each tile last held, so that backspace can draw the chip that just left.
 * By the time `erased` runs `patchDraft` has already blanked the tile, and the
 * letter is nowhere in the DOM. Keyed by node: a rebuilt board is new nodes and
 * an empty map, so a stale letter can never be drawn on a tile that never had
 * it, at the price of no ghost for the one letter typed before a render.
 */
const held = new WeakMap<HTMLElement, string>()

/**
 * Chips kick up dust in the foreground colour, the accent and the cash colour:
 * the palette the table is drawn in, and nothing brighter. Literal here because
 * particles take colours as strings; they are the tokens `--fg`, `--accent-hi`
 * and `--accent` of the Smoke Room.
 */
const DUST = ["#ebe7df", "#9be8d8", "#58c9b3"] as const
const RED = ["#ff6b73", "#ff3d48", "#ffb3b8"] as const

/** A letter just landed in `tile` (after `patchDraft` drew it). */
export function typed(tile: HTMLElement): void {
  if (!tactile()) return
  held.set(tile, tile.textContent ?? "")
  // Tabletop keeps the wave and the ghost and drops what is thrown off: dust
  // and the streak are the Smoke Room's, a board game's tiles kick up nothing.
  const fancy = juiced()
  const rect = tile.getBoundingClientRect()
  // Dust from the chip's foot, out to both sides. Few and short: a word is
  // five of these in about a second, and the point is the impact, not a burst.
  const foot = { x: rect.left + rect.width / 2, y: rect.bottom - rect.height * 0.08 }
  if (fancy)
    burst("spark", foot, {
      count: 5,
      speed: 170,
      angle: -Math.PI / 2,
      spread: Math.PI * 1.1,
      life: 0.26,
      size: 2,
      gravity: 520,
      colors: DUST,
    })

  // The fifth letter completes the row: a wave runs along it, each chip up by a
  // hair as it passes, and lets the word settle. A brightness pulse rode on it
  // once and was dropped: it put a compositing layer under each chip and was
  // the one long task left at 4x CPU throttle. Delayed a beat so the last
  // letter's own landing (230ms) is seen first and the wave carries on from it.
  // Only in a row that is now full, and only from the last tile.
  const row = tile.parentElement
  if (!row || tile.nextElementSibling) return
  const tiles = Array.from(row.children)
  if (!tiles.every((child) => child.classList.contains("filled"))) return
  if (fancy) glint(row)
  tiles.forEach((child, index) => {
    void tween(
      child,
      [
        { translate: "0 0" },
        { translate: "0 -0.4rem", offset: 0.38 },
        { translate: "0 0.06rem", offset: 0.72 },
        { translate: "0 0" },
      ],
      { duration: 300, delay: 110 + index * 34, easing: "cubic-bezier(0.34, 1.56, 0.64, 1)" },
    )
  })
}

/**
 * A row completes: a bright streak crosses it left to right, the way light
 * runs across a brass plate when the cabinet is nudged. It is a band in the fx
 * layer over the row's box, clipped to it, because a render rebuilds the row
 * (and anything parented to it) a moment after the last key, and one streak
 * whose element outlives that is the whole point. The band is a gradient on a
 * child that only ever translates, so it is composited and the row is not
 * repainted. Starts as the last chip lands (the wave's own delay) and takes
 * 420ms, so the chips rise under it rather than before it.
 */
function glint(row: Element): void {
  const rect = row.getBoundingClientRect()
  const clip = document.createElement("div")
  clip.className = "row-glint"
  clip.style.cssText = `left:${rect.left}px;top:${rect.top}px;width:${rect.width}px;height:${rect.height}px`
  const band = document.createElement("i")
  clip.append(band)
  fxDom().append(clip)
  const done = () => clip.remove()
  tween(
    band,
    [
      { translate: "-120% 0", opacity: 1 },
      { translate: "340% 0", opacity: 1 },
    ],
    { duration: 420, delay: 120, easing: "cubic-bezier(0.45, 0, 0.35, 1)" },
  ).then(done, done)
}

/** A letter just left the row being typed. */
export function erased(row: HTMLElement): void {
  if (!tactile()) return
  // The tile that was just blanked is the first one without a letter in it.
  const gone = Array.from(row.children).find((tile) => !tile.classList.contains("filled"))
  if (!(gone instanceof HTMLElement)) return
  const letter = held.get(gone)
  held.delete(gone)
  const at = centerOf(gone)
  if (juiced())
    burst("spark", at, {
      count: 4,
      speed: 130,
      angle: -Math.PI / 2,
      spread: Math.PI * 2,
      life: 0.22,
      size: 2,
      gravity: 200,
      colors: DUST,
    })
  if (!letter) return
  // The chip itself, drawn again in the layer where a render cannot take it,
  // and flicked up and away. `stand` copies the empty tile's box; the class and
  // the letter are put back on the copy, and the font size with them, since a
  // node outside `.grid` has none of the container-query size the board sets.
  const { ghost } = stand(gone)
  ghost.className = "tile filled"
  ghost.textContent = letter
  ghost.style.fontSize = getComputedStyle(gone).fontSize
  // At rest the copy is invisible, so the frame after its animation ends (and
  // before the promise below has removed it) cannot show it once more.
  ghost.style.opacity = "0"
  const flight = tween(
    ghost,
    [
      { transform: "translateY(0) scale(1) rotate(0deg)", opacity: 1 },
      {
        transform: "translateY(-0.6rem) scale(1.12) rotate(-4deg)",
        opacity: 1,
        offset: 0.3,
      },
      { transform: "translateY(-2.4rem) scale(0.7) rotate(-14deg)", opacity: 0 },
    ],
    { duration: 220, easing: "cubic-bezier(0.23, 1, 0.32, 1)" },
  )
  const remove = () => ghost.remove()
  flight.then(remove, remove)
}

/** The row being typed was refused (after its `rejected` class went on). */
export function rejected(row: HTMLElement): void {
  if (!juiced()) return
  // The row's own shake and red flash are CSS on `.row.rejected`. Here is what
  // CSS cannot reach: a jolt through the whole table, and red sparks thrown off
  // the row. The trauma is small (0.22 of 1, squared by the shake before it is
  // used) so it reads as a knock rather than an impact, which is what a
  // refusal is: nothing has been lost.
  shake(0.22)
  burst("spark", centerOf(row), {
    count: 12,
    speed: 300,
    angle: -Math.PI / 2,
    spread: Math.PI * 2,
    life: 0.36,
    size: 2.5,
    gravity: 500,
    colors: RED,
  })
}

/**
 * A round screen just arrived from somewhere that was not a round.
 *
 * The board is five reels spinning up and stopping one after another, left to
 * right, the way a slot cabinet settles once the lever is let go. Every tile of
 * a column starts from the same blurred, stretched, low pose, drifts up past its
 * stop as if still turning, and comes to rest with the same small overshoot the
 * typed letter has, so the deal and the typing are one motion vocabulary. Reels
 * stop 95ms apart; within a column a row lags the last by 14ms, which is what
 * makes a reel read as a strip and not as six separate things. The last column
 * starts at 0.38s and lands at about 0.95s: a player who begins typing is not
 * held up (the tiles are live throughout), and it is slow enough to be seen.
 * The sockets are dark on a dark cabinet, so a reel moving in them is nearly
 * invisible; the socket's rim runs brass-bright while it turns and settles back
 * to its own colour as it stops, which is what makes the spin readable. The
 * blur is on the first 45% only, and every animation is finished at once by
 * a tap, since `tween` registers with `settle`. It replaced a card deal down a
 * diagonal, which was a shoe and not a machine.
 */
export function dealt(screen: HTMLElement): void {
  if (!tactile()) return
  if (!juiced()) {
    dealtByHand(screen)
    return
  }
  const rows = Array.from(screen.querySelectorAll(".grid .row"))
  rows.forEach((row, r) => {
    Array.from(row.children).forEach((tile, c) => {
      if (!tile.classList.contains("tile")) return
      void tween(
        tile,
        [
          {
            opacity: 0,
            transform: "translateY(1.1rem) scaleY(1.25)",
            filter: "blur(0.12rem)",
            borderColor: "var(--accent-hi)",
          },
          {
            opacity: 1,
            transform: "translateY(-0.7rem) scaleY(1.15)",
            filter: "blur(0.1rem)",
            borderColor: "var(--accent-hi)",
            offset: 0.3,
          },
          {
            transform: "translateY(0.55rem) scaleY(1.1)",
            filter: "blur(0.08rem)",
            borderColor: "var(--accent-hi)",
            offset: 0.45,
          },
          { transform: "translateY(0.16rem) scaleY(1)", filter: "blur(0)", offset: 0.75 },
          { transform: "none", filter: "blur(0)" },
        ],
        { duration: 560, delay: c * 95 + r * 14, easing: "cubic-bezier(0.23, 1, 0.32, 1)" },
      )
    })
  })
  // The keys rise a row at a time, three animations and not twenty-six: at 4x
  // CPU throttle the per-key version put the arrival's task at ~70ms, and the
  // row is what reads anyway.
  screen.querySelectorAll(".keyboard .key-row").forEach((keys, r) => {
    void tween(
      keys,
      [
        { opacity: 0, transform: "translateY(0.9rem) scale(0.94)" },
        { opacity: 1, transform: "translateY(-0.1rem) scale(1.01)", offset: 0.7 },
        { opacity: 1, transform: "none" },
      ],
      { duration: 300, delay: 260 + r * 90, easing: "cubic-bezier(0.23, 1, 0.32, 1)" },
    )
  })
}

/**
 * The tactile deal, for a skin whose board is a wooden tray: each tile is set
 * down from a hand's height, column by column, and settles with one small
 * bounce. No blur, no glowing rim and no reel overshoot, which is the Smoke
 * Room's machine; here it is a tile meeting a table, so it drops and stops. The
 * whole deal is 24ms a tile step and about 0.6s end to end, and it is
 * `opacity` and `transform` only, like everything else on the board.
 */
function dealtByHand(screen: HTMLElement): void {
  screen.querySelectorAll(".grid .row").forEach((row, r) => {
    Array.from(row.children).forEach((tile, c) => {
      if (!tile.classList.contains("tile")) return
      void tween(
        tile,
        [
          { opacity: 0, transform: "translateY(-0.9rem) scale(1.06)" },
          { opacity: 1, transform: "translateY(0.06rem) scale(0.99)", offset: 0.6 },
          { transform: "none" },
        ],
        { duration: 340, delay: c * 60 + r * 24, easing: "cubic-bezier(0.33, 1, 0.68, 1)" },
      )
    })
  })
  screen.querySelectorAll(".keyboard .key-row").forEach((keys, r) => {
    void tween(
      keys,
      [
        { opacity: 0, transform: "translateY(0.5rem)" },
        { opacity: 1, transform: "none" },
      ],
      { duration: 240, delay: 200 + r * 70, easing: "cubic-bezier(0.33, 1, 0.68, 1)" },
    )
  })
}

/* ------------------------------------------------------------- the keys */

/** The key a physical keypress stands for, on the live keyboard. */
function keyFor(name: string): HTMLElement | null {
  const keyboard = document.querySelector(".round-screen .keyboard")
  if (!keyboard) return null
  if (name === "Enter") return keyboard.querySelector<HTMLElement>(".key.wide")
  if (name === "Backspace")
    return keyboard.querySelector<HTMLElement>(".key-row .key.wide:last-child")
  if (!/^[a-zA-Z]$/.test(name)) return null
  const letter = name.toLowerCase()
  for (const key of keyboard.querySelectorAll<HTMLElement>(".key:not(.wide)")) {
    // The first text node is the letter; the pips are elements after it.
    if (key.firstChild?.textContent?.trim().toLowerCase() === letter) return key
  }
  return null
}

/**
 * A key going down: squashed flat and sprung back past its height. For a
 * physical press it also drops onto its lip and flashes, since nothing sets
 * `:active` on a key the player never touched; for a pointer the stylesheet's
 * `:active` already does the drop and only the squash is added, so the two do
 * not both drive `translate`.
 */
function press(key: HTMLElement, physical: boolean): void {
  if (key.classList.contains("broken")) return
  void tween(
    key,
    [
      { transform: "scale(1, 1)", ...(physical ? { filter: "brightness(1.45)" } : {}) },
      {
        transform: "scale(1.07, 0.9)",
        offset: 0.28,
        ...(physical ? { filter: "brightness(1.3)" } : {}),
      },
      { transform: "scale(0.98, 1.04)", offset: 0.62 },
      { transform: "scale(1, 1)", ...(physical ? { filter: "brightness(1)" } : {}) },
    ],
    { duration: 230, easing: "cubic-bezier(0.34, 1.56, 0.64, 1)" },
  )
  if (physical) {
    void tween(
      key,
      [{ translate: "0 0" }, { translate: "0 0.25rem", offset: 0.25 }, { translate: "0 0" }],
      { duration: 160, easing: "cubic-bezier(0.23, 1, 0.32, 1)" },
    )
  }
}

/**
 * Bound once, at import, because the hooks above only fire on typing and the
 * keyboard has to answer the first keystroke of the round as well as the rest.
 * Both bail on `tactile()`, so on a phone (or with reduced motion) they are two
 * listeners that return.
 *
 * The physical press is looked up a frame late. `App`'s own keydown listener
 * runs after this one and, for Enter, rebuilds the screen, taking the key this
 * would have flashed with it; a frame later the live key is the one in the
 * document.
 */
if (typeof window !== "undefined") {
  window.addEventListener("keydown", (event) => {
    if (!tactile() || event.repeat || event.metaKey || event.ctrlKey || event.altKey) return
    const name = event.key
    requestAnimationFrame(() => {
      // Under a sheet the keys are not being played.
      if (document.querySelector(".overlay, .sheet")) return
      const key = keyFor(name)
      if (key) press(key, true)
    })
  })
  document.addEventListener(
    "pointerdown",
    (event) => {
      if (!tactile() || !(event.target instanceof Element)) return
      const key = event.target.closest<HTMLElement>(".round-screen .key")
      if (key) press(key, false)
    },
    { capture: true, passive: true },
  )
}
