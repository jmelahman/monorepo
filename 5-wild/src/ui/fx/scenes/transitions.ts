import type { Action, GameEvent } from "../../../engine"
import { h } from "../../dom"
import { money } from "../../format"
import { play, type Snapshot, snapshot } from "../flip"
import { centerOf, fxDom, type Point } from "../layer"
import { juiced, tactile, tween } from "../motion"
import { roll } from "../numbers"
import { burst } from "../particles"
import { shake } from "../shake"
import { begin, skip, skipping, step } from "../timeline"
import { dealt } from "./board"
import { type SceneContext, screenOf } from "./context"
import { shopWillRender } from "./shop"

/**
 * What happens between screens, and the big moments on them, in the fiction the
 * table has: a late-night casino. Screens are pushed across the table by a
 * dealer, the round's card is dealt face down and turned, a win is a payout
 * ticket printed out of a slot, a boss is a neon sign flickering on over a
 * velvet rope, a lost run is the croupier's rake, and a won one is the jackpot.
 * Sheets are brass placards hung on the wall. None of it is a slam or a
 * shockwave: that was the first draft, and it read as another game.
 * Owned by phase 6 of `plans/ui-overhaul.md`; the FLIP wiring and the render
 * hooks are the foundation's.
 *
 *  * `App.render` calls `leaving` before it throws the old screen away and
 * `arrived` after the new one is standing, which is the only window in which
 * both are measurable. `playMoments` is the third hook, after the render of a
 * dispatched action, and the only one that is handed the events.
 *
 * Three rules make all of it safe to interrupt, which matters because any
 * render at all may land mid-moment (a sheet opened over the reward, a tap on
 * collect while the lines are still counting):
 *
 * - Every tween starts from somewhere else and ends at the node's own resting
 *   style (see `tween`), so a moment that is skipped, interrupted or never
 *   played leaves the screen exactly as its view drew it. Nothing here hides a
 *   node and relies on a later step to show it again.
 * - Everything a moment puts in `#fx` is registered in `props`, and `leaving`
 *   removes all of it before the next render starts. The phase-4 scene and the
 *   scoring scene share `#fx` and are never touched: only what was registered
 *   here is ours to remove.
 * - A moment that is waiting on `step` checks `alive` after every wait, because
 *   a render increments `generation` and the screen the moment was about to
 *   decorate is not there any more.
 *
 * The screen it leaves is not cloned. `App` has already detached it by the time
 * `arrived` runs, but a detached node is still a node: the ghost is that very
 * node, put back in `#fx` at the box it had, so a board with sixty tiles costs
 * nothing to ghost and a screen is never built twice. The one cost is that
 * re-attaching restarts every CSS animation on it, which the stylesheet's
 * `.fx-ghost` rule stills; see `moments.css`.
 */

export type Leaving = {
  /** The old screen's kind (see `screenKind` in `app.ts`), or null if none. */
  from: string | null
  rects: Snapshot
  /** The old screen and where it stood, for the ghost. Null off the table. */
  old: HTMLElement | null
  box: DOMRect | null
  /** The sheet that was open, so a close can be seen to happen. */
  sheet: HTMLElement | null
}

const OUT = "cubic-bezier(0.23, 1, 0.32, 1)"
const BACK = "cubic-bezier(0.34, 1.56, 0.64, 1)"
const INOUT = "cubic-bezier(0.65, 0, 0.35, 1)"

/** Everything a moment has put in `#fx`, so the next render can take it down. */
const props = new Set<HTMLElement>()
/** Bumped by every render; a moment holding an older one has been overtaken. */
let generation = 0
/** A moment holds the skip flag while it plays, and hands it back. */
let running = false
/**
 * Where the reward's total line stood, for the coins to leave from. Measured in
 * `leaving` because by the time the shop is up the line is gone; read once by
 * the `collect` that follows and then forgotten by the next render.
 */
let payer: Point | null = null

const alive = (mine: number): boolean => mine === generation

/** Put a node in `#fx` and remember it. */
function prop<T extends HTMLElement>(node: T): T {
  props.add(node)
  fxDom().append(node)
  return node
}

function drop(node: HTMLElement): void {
  props.delete(node)
  node.remove()
}

function clearProps(): void {
  generation++
  for (const node of props) node.remove()
  props.clear()
  // The skip flag is module state and a tap on the reward sets it. Whoever
  // plays next (the shop's pack opening, the next guess) starts unskipped, but
  // only because somebody remembered to say so; better that the moment which
  // set it hands it back.
  if (running) {
    running = false
    begin()
  }
}

/**
 * Which kind of screen, ignoring everything that only describes its state. The
 * intro card that starts asking about the tutorial and stops again is one
 * screen; the app's `screenKind` tells them apart because `reuseBoard` needs it
 * to, and an arrival must not.
 */
function family(kind: string | null): string | null {
  if (!kind) return null
  const names = kind.split(" ")
  for (const name of ["round-screen", "shop-screen", "intro", "title", "center"]) {
    // The classes are `round-screen` and `shop-screen`, and every comparison
    // below says `"round"` and `"shop"`: until this mapped them, `dealt`, the
    // crumble, the zoom and the shop fade were all unreachable.
    if (names.includes(name)) return name.replace("-screen", "")
  }
  return kind
}

/** Stable pseudo-randomness in 0..1, so a crumble is the same crumble twice. */
const scatter = (i: number): number => {
  const x = Math.sin(i * 127.1 + 311.7) * 43758.5453
  return x - Math.floor(x)
}

export function leaving(root: HTMLElement, from: string | null): Leaving {
  // Phase 4: the shop keeps clones of its cards (and binds its pointer tilt) here.
  shopWillRender(root)
  clearProps()
  const rects = snapshot(root)
  payer = null
  if (!juiced()) return { from, rects, old: null, box: null, sheet: null }
  const old = screenOf(root)
  const total = old?.querySelector(".reward-line.total")
  if (total) payer = centerOf(total)
  return {
    from,
    rects,
    old,
    box: old ? old.getBoundingClientRect() : null,
    sheet: root.querySelector<HTMLElement>(":scope > .overlay"),
  }
}

export function arrived(root: HTMLElement, to: string | null, was: Leaving): void {
  if (!juiced()) {
    // Tabletop has no scene changes, but a round is still dealt: the tiles are
    // set down. `dealt` knows to do the plain version when it is not juiced.
    if (tactile() && family(to) === "round" && family(was.from) !== "round") {
      const screen = screenOf(root)
      if (screen) dealt(screen)
    }
    return
  }
  play(root, was.rects)
  const screen = screenOf(root)
  if (!screen) return
  sheets(root, was)
  const before = family(was.from)
  const now = family(to)
  if (before === now) return
  if (now === "round") dealt(screen)
  const old = was.old && was.box && !was.old.isConnected ? was.old : null
  if (old && was.box) {
    if (before === "round" && now === "center" && screen.querySelector(".banner.lose")) {
      crumble(screen, old, was.box)
    } else {
      leave(adopt(old, was.box), before === "intro" && now === "round" ? "zoom" : "slide")
    }
  }
  if (now === "shop") {
    // Opacity only: the shop scene measures the shelf right after this, and a
    // slide would put every card somewhere else while it does.
    void tween(screen, [{ opacity: 0 }, { opacity: 1 }], {
      duration: 240,
      delay: 90,
      easing: "ease-out",
    })
  } else if (now === "intro") introIn(screen)
  else if (now === "title") titleIn(screen)
}

/** After the render of a dispatched action, with what it did. */
export function playMoments(
  _action: Action,
  events: readonly GameEvent[],
  ctx: SceneContext,
): void {
  if (!juiced()) return
  const won = events.some((event) => event.type === "run_won")
  const cleared = events.some((event) => event.type === "round_won")
  const paid = events.find(
    (event): event is Extract<GameEvent, { type: "gold" }> =>
      event.type === "gold" && event.reason === "round cleared",
  )
  if (won) runWon(ctx)
  else if (cleared && ctx.state.phase === "reward") roundWon(ctx)
  if (paid && !won) purse(ctx, paid.delta)
}

/* ------------------------------------------------------------- ghosts */

/**
 * The old screen, put back in `#fx` where it stood. Ids come off because the
 * live screen may share them; `inert` because it is a picture, not a thing.
 */
function adopt(old: HTMLElement, box: DOMRect): HTMLElement {
  old.removeAttribute("id")
  for (const inner of old.querySelectorAll("[id]")) inner.removeAttribute("id")
  old.inert = true
  old.classList.add("fx-ghost")
  // The shake may have left the old screen mid-wobble; the box was measured
  // with it, and a ghost that is leaving does not need to keep it.
  old.style.removeProperty("translate")
  old.style.removeProperty("rotate")
  Object.assign(old.style, {
    position: "absolute",
    left: `${box.left}px`,
    top: `${box.top}px`,
    width: `${box.width}px`,
    height: `${box.height}px`,
    margin: "0",
    pointerEvents: "none",
  })
  return prop(old)
}

/**
 * Quick, and never in the way. The new screen is live from the frame it is
 * built, so nothing here can hold input: the ghost is a picture over it for a
 * quarter of a second, and it is `pointer-events: none` throughout.
 */
function leave(ghost: HTMLElement, style: "slide" | "zoom"): void {
  const frames: Keyframe[] =
    style === "zoom"
      ? [
          { opacity: 1, transform: "none" },
          { opacity: 0, transform: "scale(1.1)" },
        ]
      : [
          { opacity: 1, transform: "none" },
          { opacity: 0, transform: "translateX(-3.5rem) scale(0.97)" },
        ]
  void tween(ghost, frames, { duration: style === "zoom" ? 240 : 280, easing: INOUT }).then(() =>
    drop(ghost),
  )
}

/* ------------------------------------------------------------- sheets */

/**
 * A sheet opening springs up out of the table, and one closing sinks back into
 * it. Opening is asked of the node the render built: a sheet the render marked
 * `settled` was already open and is only being rebuilt around a button tap, and
 * replaying its entry there is the bug `settled` exists to stop. The pack sheet
 * is the shop scene's, which opens it with its own choreography.
 */
function sheets(root: HTMLElement, was: Leaving): void {
  const overlay = root.querySelector<HTMLElement>(":scope > .overlay")
  if (overlay) {
    if (overlay.classList.contains("settled") || overlay.querySelector(".pack-sheet")) return
    void tween(overlay, [{ opacity: 0 }, { opacity: 1 }], { duration: 160, easing: "ease-out" })
    void tween(
      overlay.querySelector(".sheet"),
      [
        { opacity: 0, transform: "translateY(1.25rem) scale(0.86)" },
        { opacity: 1, transform: "none" },
      ],
      { duration: 380, easing: BACK },
    )
    return
  }
  const gone = was.sheet
  if (!gone || gone.isConnected || gone.querySelector(".pack-sheet")) return
  gone.removeAttribute("id")
  for (const inner of gone.querySelectorAll("[id]")) inner.removeAttribute("id")
  gone.inert = true
  gone.classList.add("fx-ghost")
  prop(gone)
  void tween(
    gone.querySelector(".sheet"),
    [
      { opacity: 1, transform: "none" },
      { opacity: 0, transform: "translateY(0.9rem) scale(0.94)" },
    ],
    { duration: 200, easing: "ease-in" },
  )
  void tween(gone, [{ opacity: 1 }, { opacity: 0 }], { duration: 200, easing: "ease-in" }).then(
    () => drop(gone),
  )
}

/* -------------------------------------------------------------- title */

/**
 * The masthead drops onto the table tile by tile. The idle loops after it are
 * CSS (`moments.css`), on `translate` and `rotate` so they compose with this.
 */
function titleIn(screen: HTMLElement): void {
  screen.querySelectorAll(".title-tile").forEach((tile, i) => {
    void tween(
      tile,
      [
        { opacity: 0, transform: "translateY(-3rem) rotate(-10deg) scale(0.8)" },
        { opacity: 1, transform: "translateY(0.25rem) scale(1.04)", offset: 0.72 },
        { opacity: 1, transform: "none" },
      ],
      { duration: 460, delay: 60 + i * 70, easing: OUT },
    )
  })
  // Its resting pose is a tilt, so the tween ends on the tilt and not on none.
  void tween(
    screen.querySelector(".title-score"),
    [
      { opacity: 0, transform: "rotate(6deg) scale(0.2)" },
      { opacity: 1, transform: "rotate(6deg) scale(1)" },
    ],
    { duration: 380, delay: 500, easing: BACK },
  )
}

/* -------------------------------------------------------------- intro */

/**
 * The round's card, dealt. The stage's three cards come first, one after the
 * other, then the card spins in and its token is tossed onto it. A boss is the
 * same card thrown harder: it drops in from above, the emblem slams down onto
 * it, and the table answers with a wash, a shockwave, a shake and a banner.
 * Everything a player has to read is on the card at rest; this is the trip.
 */
function introIn(screen: HTMLElement): void {
  begin()
  running = true
  const mine = generation
  const card = screen.querySelector<HTMLElement>(".intro-card")
  const boss = card?.classList.contains("boss-card") ?? false

  void tween(
    screen.querySelector(".intro-stage"),
    [
      { opacity: 0, transform: "translateY(-0.75rem)" },
      { opacity: 1, transform: "none" },
    ],
    { duration: 300, easing: OUT },
  )
  screen.querySelectorAll(".track-round").forEach((cell, i) => {
    void tween(
      cell,
      [
        { opacity: 0, transform: "translateY(-2.25rem) rotate(-9deg) scale(0.88)" },
        { opacity: 1, transform: "none" },
      ],
      { duration: 420, delay: 80 + i * 110, easing: BACK },
    )
  })
  void tween(
    screen.querySelector(".intro-actions"),
    [
      { opacity: 0, transform: "translateY(1rem)" },
      { opacity: 1, transform: "none" },
    ],
    { duration: 320, delay: boss ? 700 : 420, easing: OUT },
  )

  const token = card?.querySelector<HTMLElement>(":scope > .round-token")
  if (boss) {
    void tween(
      card,
      [
        { opacity: 0, transform: "translateY(-3.5rem) scale(1.3)" },
        { opacity: 1, transform: "translateY(0.2rem) scale(0.98)", offset: 0.72 },
        { opacity: 1, transform: "none" },
      ],
      { duration: 340, delay: 60, easing: OUT },
    )
    void tween(
      token,
      [
        { opacity: 0, transform: "scale(3.4) rotate(-35deg)" },
        { opacity: 1, transform: "scale(0.92) rotate(3deg)", offset: 0.7 },
        { opacity: 1, transform: "none" },
      ],
      { duration: 300, delay: 260, easing: "ease-in" },
    )
    const flash = prop(h("div", { class: "fx-flash boss" }))
    void tween(
      flash,
      [
        { opacity: 0 },
        { opacity: 1, offset: 0.14 },
        { opacity: 0.7, offset: 0.45 },
        { opacity: 0 },
      ],
      { duration: 1300, easing: "linear" },
    ).then(() => drop(flash))
    const banner = prop(
      h("div", { class: "fx-banner" }, card?.querySelector(".intro-name")?.textContent ?? ""),
    )
    void tween(
      banner,
      [
        { transform: "translateX(-105%) skewX(-12deg)", easing: OUT },
        { transform: "none", offset: 0.2 },
        { transform: "none", offset: 0.72, easing: "ease-in" },
        { transform: "translateX(105%) skewX(-12deg)" },
      ],
      { duration: 1150, easing: "linear" },
    ).then(() => drop(banner))
    void (async () => {
      await step(560)
      if (!alive(mine) || skipping()) return
      const at = token ? centerOf(token) : card ? centerOf(card) : null
      if (!at) return
      burst("ring", at, { size: 280, life: 0.6, colors: ["#ff7d8e"] })
      burst("ring", at, { size: 170, life: 0.45, colors: ["#ffffff"] })
      burst("spark", at, {
        count: 28,
        speed: 640,
        life: 0.55,
        colors: ["#ffd0d6", "#ff5d73", "#ffffff"],
      })
      burst("ember", at, { count: 14, speed: 160, spread: Math.PI * 1.4, life: 1.1 })
      shake(0.6)
    })()
  } else {
    void tween(
      card,
      [
        {
          opacity: 0,
          transform: "perspective(900px) translateY(1.5rem) rotateY(-75deg) scale(0.88)",
        },
        {
          opacity: 1,
          transform: "perspective(900px) translateY(-0.25rem) rotateY(6deg) scale(1.02)",
          offset: 0.7,
        },
        { opacity: 1, transform: "none" },
      ],
      { duration: 540, delay: 140, easing: OUT },
    )
    void tween(
      token,
      [
        { opacity: 0, transform: "rotate(-300deg) scale(0.1)" },
        { opacity: 1, transform: "none" },
      ],
      { duration: 560, delay: 260, easing: BACK },
    )
  }

  // The target counts up to itself. Only when it is a plain integer: an
  // abbreviated one ("1.2M") has no digits to climb, and is shown as it is.
  const target = card?.querySelector<HTMLElement>(".intro-target")
  const text = target?.textContent ?? ""
  if (target && /^\d{1,3}(?:[,.   ]\d{3})*$/.test(text)) {
    const value = Number(text.replace(/\D/g, ""))
    target.textContent = "0"
    void (async () => {
      await step(boss ? 600 : 320)
      if (!alive(mine)) return
      roll(target, 0, value, { span: 520 })
      running = false
    })()
  } else running = false
}

/* ---------------------------------------------------------- round won */

/** A full-window wash that fades, for an impact. */
function flash(kind: "boss" | "win", duration: number): void {
  const node = prop(h("div", { class: `fx-flash ${kind}` }))
  void tween(node, [{ opacity: 0 }, { opacity: 1, offset: 0.2 }, { opacity: 0 }], {
    duration,
    easing: "linear",
  }).then(() => drop(node))
}

/**
 * The reward screen after a winning guess. The banner settles in, the panel
 * rises, then the lines pay in one at a time, each counting up from nothing,
 * quickening as they go, and the total lands last with a few sparks. Tap
 * anywhere and it is all there.
 *
 * It was louder, and the owner asked for it toned down with the rest of the
 * scoring show. The banner slammed in from 2.6x with a 6deg twist; a green
 * ring, 24 sparks, a full-window flash and a shake went off behind it; every
 * line threw up to 7 coins; the total fired a fountain of 26, three rains of 8
 * from the top edge, a gold ring and a second shake; and the button bounced to
 * 1.09x at the end. That was a jackpot's worth of noise for the most frequent
 * win in the game, and it all repeated every round. What is left is the count
 * itself, which is the information, and one small beat on the total.
 *
 * The lines and the panel are tweened, never hidden: their delays are the
 * schedule, and a tap finishes every one of them at once (`settle`), so the
 * screen the player ends up on is the one the view drew.
 */
function roundWon(ctx: SceneContext): void {
  const { screen, sound } = ctx
  begin()
  running = true
  const mine = generation
  screen.addEventListener("pointerdown", skip, { once: true })

  const banner = screen.querySelector<HTMLElement>(".banner")
  void tween(
    banner,
    [
      { opacity: 0, transform: "translateY(-0.75rem) scale(1.04)" },
      { opacity: 1, transform: "none" },
    ],
    { duration: 360, delay: 140, easing: OUT },
  )
  void tween(
    screen.querySelector(".panel"),
    [
      { opacity: 0, transform: "translateY(1.5rem) scale(0.96)" },
      { opacity: 1, transform: "none" },
    ],
    { duration: 380, delay: 380, easing: OUT },
  )

  const lines = [...screen.querySelectorAll<HTMLElement>(".reward-line")]
  // Each gap is 0.9 of the last down to 120ms, the chain's own arithmetic, so
  // a shelf of relic payouts does not take twice as long as a bare round.
  const stamps: number[] = []
  let at = 640
  lines.forEach((_, i) => {
    stamps.push(at)
    at += Math.max(120, 260 * 0.9 ** i) + (i === lines.length - 2 ? 160 : 0)
  })
  lines.forEach((line, i) => {
    void tween(
      line,
      [
        { opacity: 0, transform: "translateX(-1.5rem)" },
        { opacity: 1, transform: "none" },
      ],
      { duration: 260, delay: stamps[i] ?? 0, easing: OUT },
    )
  })
  const button = screen.querySelector<HTMLElement>(":scope > .primary")
  void tween(
    button,
    [
      { opacity: 0, transform: "translateY(0.75rem)" },
      { opacity: 1, transform: "none" },
    ],
    { duration: 300, delay: at, easing: OUT },
  )

  // Amounts start at nothing and count to what the view drew.
  const paid = lines.map((line) => {
    const span = line.lastElementChild
    const match = /^\$(\d+)$/.exec(span?.textContent ?? "")
    if (!(span instanceof HTMLElement) || !match) return null
    span.textContent = money(0)
    return { span, value: Number(match[1]) }
  })

  void (async () => {
    let clock = 0
    for (const [i, line] of lines.entries()) {
      await step((stamps[i] ?? clock) - clock)
      clock = stamps[i] ?? clock
      if (!alive(mine)) return
      const entry = paid[i]
      const isTotal = line.classList.contains("total")
      if (entry) {
        roll(entry.span, 0, entry.value, { span: isTotal ? 520 : 300, format: money })
        if (!skipping()) {
          // The total's one beat: a handful of sparks, short and slow, in the
          // cash colours. The lines above it land on their sound alone.
          if (isTotal)
            burst("spark", centerOf(entry.span), {
              count: 10,
              speed: 260,
              life: 0.45,
              size: 2.5,
              colors: ["#ffd166", "#ffffff"],
            })
          sound.cue({ name: "coin" })
        }
      }
    }
    running = false
  })()
}

/* ---------------------------------------------------------- collecting */

/**
 * The reward paid into the purse: coins fly from where the total stood to the
 * gold counter, each landing pings it, and the number climbs while they arrive.
 * The counter was drawn at its new figure by the render, so it is wound back to
 * the old one first and rolled forward; skipped, interrupted or reduced, it is
 * simply the new one.
 */
function purse(ctx: SceneContext, delta: number): void {
  const target = ctx.screen.querySelector<HTMLElement>(".hud-gold")
  if (!target || delta <= 0) return
  begin()
  running = true
  const mine = generation
  const to = ctx.state.gold
  const from = to - delta
  const dest = centerOf(target)
  const origin = payer ?? { x: innerWidth / 2, y: innerHeight * 0.55 }
  const count = Math.min(12, 5 + delta)
  const stagger = 45
  target.textContent = money(from)

  for (let k = 0; k < count; k++) {
    const coin = prop(h("div", { class: "fx-coin" }))
    coin.style.position = "absolute"
    coin.style.left = `${origin.x}px`
    coin.style.top = `${origin.y}px`
    coin.style.translate = "-50% -50%"
    const out = { x: (scatter(k) - 0.5) * 260, y: -(60 + scatter(k + 9) * 120) }
    void tween(
      coin,
      [
        { transform: "translate(0px, 0px) scale(0.5)", opacity: 1, easing: OUT },
        {
          transform: `translate(${out.x}px, ${out.y}px) scale(1)`,
          offset: 0.35,
          easing: "cubic-bezier(0.5, 0, 1, 0.6)",
        },
        {
          transform: `translate(${dest.x - origin.x}px, ${dest.y - origin.y}px) scale(0.7)`,
          opacity: 1,
        },
      ],
      { duration: 620, delay: k * stagger, easing: "linear" },
    ).then(() => {
      drop(coin)
      if (!alive(mine) || skipping()) return
      burst("spark", dest, { count: 4, speed: 200, life: 0.3, size: 2.5 })
      void tween(target, [{ transform: "scale(1.18)" }, { transform: "none" }], {
        duration: 160,
        easing: OUT,
      })
      if (k % 3 === 0) ctx.sound.cue({ name: "coin" })
      if (k === count - 1) running = false
    })
  }
  void (async () => {
    await step(360)
    if (!alive(mine)) return
    roll(target, from, to, { span: count * stagger + 300, format: money })
  })()
}

/* ---------------------------------------------------------- game over */

/**
 * The board comes apart. The old screen is the ghost, and its tiles and keys
 * fall off it under gravity, each at its own moment and turn, throwing shards
 * in the colour it was; the room goes grey behind them and the ghost fades out
 * once they are gone. The end screen is already underneath, its banner dropping
 * in as the last of the board goes.
 */
function crumble(screen: HTMLElement, old: HTMLElement, box: DOMRect): void {
  // A moment of its own, so it starts unskipped: the guess that lost the run
  // may have been skipped through, and the skip would otherwise still be up and
  // swallow this scene's shake (see `shake`), which asks the clock.
  begin()
  const ghost = adopt(old, box)
  const mine = generation
  const bits = [...ghost.querySelectorAll<HTMLElement>(".grid .tile, .keyboard .key")]
  const spots = bits.map((bit) => bit.getBoundingClientRect())
  let last = 0
  bits.forEach((bit, i) => {
    const spot = spots[i]
    if (!spot) return
    const delay = 120 + scatter(i) * 380
    const sideways = (scatter(i + 97) - 0.5) * 220
    const drop_ = innerHeight - spot.top + 120
    const turn = (scatter(i + 31) - 0.5) * 260
    // Fall time goes with the root of the distance, which is what gravity does.
    const duration = 300 + Math.sqrt(drop_) * 14
    last = Math.max(last, delay + duration)
    void tween(
      bit,
      [
        { transform: "none", opacity: 1, easing: "ease-out" },
        {
          transform: `translate(${sideways * 0.06}px, -0.9rem) rotate(${turn * 0.06}deg)`,
          opacity: 1,
          offset: 0.14,
          easing: "cubic-bezier(0.5, 0, 1, 0.6)",
        },
        { transform: `translate(${sideways}px, ${drop_}px) rotate(${turn}deg)`, opacity: 0.85 },
      ],
      { duration, delay, easing: "linear" },
    ).then(() => {
      // Back at rest for the frame between this ending and the ghost going.
      bit.style.visibility = "hidden"
    })
    if (i % 2 === 0 && bit.classList.contains("tile")) {
      const colour = getComputedStyle(bit).backgroundColor
      setTimeout(
        () => {
          if (!alive(mine)) return
          burst(
            "shard",
            { x: spot.left + spot.width / 2, y: spot.top + spot.height / 2 },
            { count: 4, speed: 260, life: 0.7, colors: [colour, "#ffffff"] },
          )
        },
        delay * 0.6 + 20,
      )
    }
  })
  void tween(ghost, [{ opacity: 1 }, { opacity: 1, offset: 0.55 }, { opacity: 0 }], {
    duration: last + 60,
    easing: "linear",
  }).then(() => drop(ghost))

  const gloom = prop(h("div", { class: "fx-gloom" }))
  void tween(
    gloom,
    [{ opacity: 0 }, { opacity: 1, offset: 0.18 }, { opacity: 1, offset: 0.6 }, { opacity: 0 }],
    { duration: last + 500, easing: "linear" },
  ).then(() => drop(gloom))
  shake(0.75)

  // The end screen, arriving as the board leaves.
  void tween(
    screen.querySelector(".banner"),
    [
      { opacity: 0, transform: "translateY(-3.5rem) scale(1.15)" },
      { opacity: 1, transform: "translateY(0.4rem)", offset: 0.7 },
      { opacity: 1, transform: "none" },
    ],
    { duration: 520, delay: 520, easing: OUT },
  )
  void tween(
    screen.querySelector(".panel"),
    [
      { opacity: 0, transform: "translateY(1.25rem)" },
      { opacity: 1, transform: "none" },
    ],
    { duration: 380, delay: 820, easing: OUT },
  )
  void tween(
    screen.querySelector(":scope > .primary"),
    [
      { opacity: 0, transform: "translateY(0.75rem)" },
      { opacity: 1, transform: "none" },
    ],
    { duration: 300, delay: 1050, easing: OUT },
  )
}

/* ------------------------------------------------------------ run won */

const FIREWORKS: readonly (readonly [number, number, readonly string[]])[] = [
  [0.5, 0.3, ["#fff4c2", "#ffd166", "#ff9f1c"]],
  [0.24, 0.4, ["#c9f5ff", "#4cc9f0", "#ffffff"]],
  [0.76, 0.36, ["#ffd0e0", "#ff4d6d", "#ffffff"]],
  [0.38, 0.2, ["#e6d5ff", "#b388ff", "#ffffff"]],
  [0.64, 0.24, ["#d0ffe6", "#06d6a0", "#ffffff"]],
  [0.5, 0.44, ["#fff4c2", "#ff9f1c", "#ff4d6d"]],
]

/**
 * The run is won. The banner is thrown up, confetti fires from both corners,
 * and fireworks go off over the next two seconds, six of them, each a ring and
 * two layers of sparks. About 500 particles at the busiest against a cap of
 * 600, and the cap is what the pool does if a slow frame ever stacks them.
 */
function runWon(ctx: SceneContext): void {
  const { screen } = ctx
  begin()
  running = true
  const mine = generation
  screen.addEventListener("pointerdown", skip, { once: true })
  void tween(
    screen.querySelector(".banner"),
    [
      { opacity: 0, transform: "scale(0.3) rotate(-12deg)" },
      { opacity: 1, transform: "scale(1.12) rotate(2deg)", offset: 0.65 },
      { opacity: 1, transform: "none" },
    ],
    { duration: 620, delay: 100, easing: OUT },
  )
  void tween(
    screen.querySelector(".panel"),
    [
      { opacity: 0, transform: "translateY(1.5rem) scale(0.96)" },
      { opacity: 1, transform: "none" },
    ],
    { duration: 380, delay: 400, easing: OUT },
  )
  void tween(
    screen.querySelector(":scope > .primary"),
    [
      { opacity: 0, transform: "translateY(0.75rem)" },
      { opacity: 1, transform: "none" },
    ],
    { duration: 300, delay: 700, easing: OUT },
  )
  flash("win", 900)
  const w = innerWidth
  const height = innerHeight
  burst(
    "confetti",
    { x: w * 0.04, y: height * 0.92 },
    { count: 55, angle: -1.15, spread: 0.7, speed: 1250, gravity: 900 },
  )
  burst(
    "confetti",
    { x: w * 0.96, y: height * 0.92 },
    { count: 55, angle: -Math.PI + 1.15, spread: 0.7, speed: 1250, gravity: 900 },
  )
  void (async () => {
    for (const [i, [x, y, colors]] of FIREWORKS.entries()) {
      await step(i === 0 ? 200 : 300)
      if (!alive(mine) || skipping()) return
      const at = { x: w * x, y: height * y }
      burst("ring", at, { size: 220, life: 0.5, colors: [colors[1] ?? "#ffffff"] })
      burst("spark", at, { count: 36, speed: 460, life: 1, gravity: 320, colors })
      burst("spark", at, { count: 18, speed: 240, life: 0.8, gravity: 200, size: 2.5, colors })
      if (i === 2) {
        burst(
          "confetti",
          { x: w / 2, y: -20 },
          { count: 60, angle: Math.PI / 2, spread: Math.PI * 1.2, speed: 250, life: 3 },
        )
      }
    }
    running = false
  })()
}
