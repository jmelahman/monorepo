import type { Action, GameEvent } from "../../../engine"
import { centerOf, fxDom, type Point, stand } from "../layer"
import { juiced, ms, replay, tween } from "../motion"
import { burst } from "../particles"
import type { SceneContext } from "./context"

/**
 * The shop, relics and packs on the table: dealing, buying, selling, rerolling,
 * tearing a pack open, and the pointer tilt every card answers to. Owned by
 * phase 4 of `plans/ui-overhaul.md`.
 *
 * Called after the render that shows an action's result, only when `juiced()`.
 *
 * The problem this file is shaped around is that a render throws the card away
 * *before* anyone can ask where it was. A bought relic is a shelf card one frame
 * and a tray card the next, and the shelf slot it left is a "sold" placeholder,
 * so FLIP has no shared key to hang the flight on. FLIP by `data-flip` was the
 * first answer and it works for what really persists (the tray sliding shut
 * after a sale, see `shopView`); for what changes identity it cannot.
 *
 * So `shopWillRender` runs in `transitions.leaving`, before the old screen is
 * cleared, and keeps a detached clone of every card with its box. The clones
 * are made with `layer.stand` and taken straight back out of the layer, so
 * nothing is left on screen if the render turns out not to be a shop action
 * (a menu opening, a coach step), and the next call simply replaces them. The
 * scene, running after the render, puts the clone that matters back over the
 * place the card stood and flies it. It also binds the pointer listeners the
 * first time it is called, since every render on the table passes through it,
 * which saves the app a second hook.
 */

type Kept = { ghost: HTMLElement; rect: DOMRect }

type Before = {
  shelf: (Kept | null)[]
  pack: (Kept | null)[]
  relics: (Kept | null)[]
}

let before: Before | null = null

/** A detached clone of a card at its current box, or nothing for an empty seat. */
function keep(node: Element | undefined): Kept | null {
  if (!node || node.classList.contains("sold") || node.classList.contains("empty")) return null
  const kept = stand(node)
  kept.ghost.remove()
  // Not the card's own idle loop, and no tip: the clone is a picture of it.
  kept.ghost.classList.add("shop-ghost")
  kept.ghost.removeAttribute("data-flip")
  kept.ghost.removeAttribute("data-tip")
  return kept
}

const keepAll = (list: ArrayLike<Element>): (Kept | null)[] =>
  Array.from(list, (node) => keep(node))

/**
 * Before the render: remember the cards. Free off the table, and on any screen
 * with no cards on it.
 */
export function shopWillRender(root: HTMLElement): void {
  bindPointer()
  before = null
  if (!juiced()) return
  const shelf = root.querySelectorAll(".shop-items > *")
  const pack = root.querySelectorAll(".pack-options > *")
  const relics = root.querySelectorAll(".shop-screen .relics > .relic")
  if (!shelf.length && !pack.length && !relics.length) return
  before = { shelf: keepAll(shelf), pack: keepAll(pack), relics: keepAll(relics) }
}

/* ---------------------------------------------------------------- helpers */

const ease = (t: number): number => (t < 0.5 ? 4 * t ** 3 : 1 - (-2 * t + 2) ** 3 / 2)
const easeOut = (t: number): number => 1 - (1 - t) ** 3

/** Where the deck is: below and to the right of the window, out of sight. */
const deck = (): Point => ({ x: window.innerWidth + 140, y: window.innerHeight + 60 })

const rectOf = (node: Element): DOMRect => node.getBoundingClientRect()

function rareOf(node: Element): string {
  const value = getComputedStyle(node).getPropertyValue("--rare-c").trim()
  return value || "#8e88ab"
}

/** A stand-in put back in the layer over the box it was kept at. */
function seat(kept: Kept): HTMLElement {
  fxDom().append(kept.ghost)
  return kept.ghost
}

type Waypoint = { o: number; x: number; y: number; s: number; r: number }

/**
 * A path as keyframes: a quadratic arc from one point to another, lifted at the
 * middle, sampled densely enough that the linear segments read as a curve.
 * Time is folded into the samples (`ease`), so the animation itself is linear
 * and the lift-off anticipation can sit in front of the travel.
 */
function arc(
  from: Point,
  to: Point,
  options: { lift: number; scale: [number, number]; spin: number; anticipate?: number },
): Waypoint[] {
  const { lift, scale, spin, anticipate = 0.14 } = options
  const points: Waypoint[] = [{ o: 0, x: 0, y: 0, s: 1, r: 0 }]
  // A card picked up rises and swells before it goes anywhere.
  const rise = { x: 0, y: -14, s: Math.max(1.08, scale[0] * 1.08), r: 0 }
  points.push({ o: anticipate, ...rise })
  const cx = (from.x + to.x) / 2
  const cy = Math.min(from.y, to.y) - lift
  const steps = 14
  for (let i = 1; i <= steps; i++) {
    const t = ease(i / steps)
    const x = (1 - t) ** 2 * from.x + 2 * (1 - t) * t * cx + t ** 2 * to.x
    const y = (1 - t) ** 2 * from.y + 2 * (1 - t) * t * cy + t ** 2 * to.y
    points.push({
      o: anticipate + (1 - anticipate) * (i / steps),
      x: x - from.x,
      y: y - from.y + rise.y * (1 - t),
      s: rise.s + (scale[1] - rise.s) * t,
      r: Math.sin(Math.PI * t) * spin,
    })
  }
  return points
}

const frames = (points: Waypoint[], fade?: number): Keyframe[] =>
  points.map((p) => ({
    offset: p.o,
    transform: `translate(${p.x}px, ${p.y}px) rotate(${p.r}deg) scale(${p.s})`,
    ...(fade === undefined ? {} : { opacity: p.o < fade ? 1 : 1 - (p.o - fade) / (1 - fade) }),
  }))

/**
 * Fly a stand-in over an arc onto a target, and let the real thing it turns
 * into fade up underneath as it lands. Resolves at the landing.
 */
function flight(
  kept: Kept,
  target: Element | Point | null,
  options: {
    duration?: number
    delay?: number
    lift?: number
    spin?: number
    dest?: Element | null
  },
): Promise<void> {
  const ghost = seat(kept)
  const from = { x: kept.rect.left + kept.rect.width / 2, y: kept.rect.top + kept.rect.height / 2 }
  const to = !target ? from : "getBoundingClientRect" in target ? centerOf(target) : target
  const goal = target && "getBoundingClientRect" in target ? rectOf(target) : null
  const shrink = goal ? Math.min(goal.width / kept.rect.width, goal.height / kept.rect.height) : 0.5
  const duration = options.duration ?? 700
  const delay = options.delay ?? 0
  const path = arc(from, to, {
    lift: options.lift ?? 90,
    scale: [1, Math.max(0.3, shrink)],
    spin: options.spin ?? 7,
  })
  // Invisible by its own style once the animation lets go of it, so a frame
  // between the end and the removal cannot show it back where it started.
  ghost.style.opacity = "0"
  const gone = tween(ghost, frames(path, 0.86), { duration, delay, easing: "linear" })
  // The seat it lands in stays empty until it does, then comes up as it fades.
  if (options.dest) {
    void tween(options.dest, [{ opacity: 0 }, { opacity: 0, offset: 0.82 }, { opacity: 1 }], {
      duration,
      delay,
      easing: "linear",
    })
  }
  return gone.then(() => ghost.remove())
}

/** A coin: the price of something, or what it sold for. Kept to a handful. */
function coins(
  from: Point,
  to: Point,
  count: number,
  options: { delay?: number; landed?: () => void } = {},
) {
  const base = options.delay ?? 0
  let last: Promise<void> = Promise.resolve()
  for (let i = 0; i < count; i++) {
    const coin = document.createElement("div")
    coin.className = "shop-coin"
    coin.style.left = `${from.x}px`
    coin.style.top = `${from.y}px`
    coin.style.opacity = "0"
    fxDom().append(coin)
    const dx = to.x - from.x
    const dy = to.y - from.y
    const bow = (i % 2 ? -1 : 1) * (18 + i * 5)
    const mid = { x: dx * 0.5 - (dy / Math.hypot(dx, dy) || 0) * bow, y: dy * 0.5 - 40 - i * 4 }
    last = tween(
      coin,
      [
        { transform: "translate(-50%, -50%) scale(0.4)", opacity: 0 },
        {
          transform: `translate(calc(-50% + ${mid.x}px), calc(-50% + ${mid.y}px)) scale(1.1)`,
          opacity: 1,
          offset: 0.5,
        },
        {
          transform: `translate(calc(-50% + ${dx}px), calc(-50% + ${dy}px)) scale(0.8)`,
          opacity: 1,
          offset: 0.92,
        },
        {
          transform: `translate(calc(-50% + ${dx}px), calc(-50% + ${dy}px)) scale(0.2)`,
          opacity: 0,
        },
      ],
      { duration: 520, delay: base + i * 55, easing: "cubic-bezier(0.4, 0, 0.2, 1)" },
    ).then(() => coin.remove())
  }
  void last.then(() => options.landed?.())
  return last
}

const goldSpot = (root: ParentNode): Point | null => {
  const node = root.querySelector(".hud-gold")
  return node ? centerOf(node) : null
}

/** How many coins a price is worth drawing: every dollar up to a fistful. */
const coinCount = (dollars: number): number =>
  Math.max(2, Math.min(7, Math.round(Math.abs(dollars))))

/* ---------------------------------------------------------------- dealing */

const BACK_FLIP = 0.5

/**
 * One card thrown onto the table: out of a point, across an arc, turning over
 * on the way. The face is the real node; the back is a lid laid over it that
 * is dropped at the edge-on moment, which is the only frame at which the two
 * are indistinguishable. Resolves when the card has landed.
 */
function deal(
  node: HTMLElement,
  from: Point,
  options: { delay: number; duration?: number; lift?: number; fan?: number },
): Promise<void> {
  const rect = rectOf(node)
  if (!rect.width) return Promise.resolve()
  const home = { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 }
  const duration = options.duration ?? 640
  const fan = options.fan ?? 0
  const start = { x: from.x - home.x, y: from.y - home.y }
  const lid = document.createElement("div")
  lid.className = "deal-back"
  lid.style.opacity = "0"
  node.append(lid)
  const spin = start.x > 0 ? -1 : 1
  const path: Keyframe[] = []
  const steps = 12
  for (let i = 0; i <= steps; i++) {
    const t = i / steps
    const e = easeOut(t)
    const x = start.x * (1 - e)
    const y = start.y * (1 - e) - Math.sin(Math.PI * e) * 120
    const turn = 180 * (1 - Math.min(1, e * 1.15))
    const roll = (spin * 26 + fan) * (1 - e) - fan * Math.sin(Math.PI * e) * 0.3
    path.push({
      offset: t,
      transform: `translate(${x}px, ${y}px) perspective(900px) rotateY(${turn}deg) rotate(${roll}deg) scale(${0.55 + 0.45 * e})`,
      opacity: t < 0.04 ? t / 0.04 : 1,
    })
  }
  void tween(
    lid,
    [
      { opacity: 1 },
      { opacity: 1, offset: BACK_FLIP * 0.8 },
      { opacity: 0, offset: BACK_FLIP * 0.8 + 0.001 },
      { opacity: 0 },
    ],
    { duration, delay: options.delay, easing: "linear" },
  )
  // The card's own paint is held until its turn comes: the lid is above it and
  // opaque, so a card waiting on its delay shows a back sitting in its place,
  // and `backwards` fill keeps it at the deck instead.
  return tween(node, path, { duration, delay: options.delay, easing: "linear" }).then(() => {
    lid.remove()
  })
}

/** The whole shelf, one card after another out of the deck. */
function dealShelf(screen: ParentNode, startDelay = 0): void {
  const cards = screen.querySelectorAll<HTMLElement>(".shop-items > .shop-item:not(.sold)")
  const origin = deck()
  cards.forEach((card, i) => {
    void deal(card, origin, { delay: startDelay + i * 95, lift: 120 })
  })
}

/* ------------------------------------------------------------------ scenes */

const seatOf = (screen: ParentNode, kind: string, state: SceneContext["state"]): Element | null => {
  switch (kind) {
    case "relic": {
      const owned = screen.querySelectorAll(".relics > .relic:not(.empty)")
      return owned[Math.min(owned.length, state.relics.length) - 1] ?? null
    }
    case "consumable": {
      const held = screen.querySelectorAll(".consumables > .consumable")
      return held[held.length - 1] ?? null
    }
    case "level":
      return screen.querySelector(".shapes-line")
    default:
      return screen.querySelector(".hud")
  }
}

function buy(index: number, ctx: SceneContext, events: readonly GameEvent[]): void {
  const kept = before?.shelf[index]
  if (!kept) return
  const gold = goldSpot(ctx.root)
  const here = { x: kept.rect.left + kept.rect.width / 2, y: kept.rect.top + kept.rect.height / 2 }
  const kind = /kind-(\w+)/.exec(kept.ghost.className)?.[1] ?? "relic"
  const paid = events.find((e) => e.type === "gold")
  if (gold) coins(gold, here, coinCount(paid?.type === "gold" ? paid.delta : 3))

  if (kind === "pack") {
    void openPack(kept, ctx)
    return
  }
  const screen = ctx.root
  const held = ctx.state.placing
  // A card with no place to land yet (a modifier waiting for its letter) goes
  // to the middle of the table, where the picker opens over it.
  const target: Element | Point | null = held
    ? { x: window.innerWidth / 2, y: window.innerHeight / 2 }
    : seatOf(screen, kind, ctx.state)
  const dest =
    !held && target instanceof Element && kind !== "level" && kind !== "range" ? target : null
  const landing = flight(kept, target, { delay: 140, duration: 720, dest })
  void landing.then(() => {
    const at = target instanceof Element ? centerOf(target) : target
    if (at) burst("spark", at, { count: 10, speed: 320, life: 0.4, size: 2.5 })
  })
  const slot = ctx.screen.querySelectorAll<HTMLElement>(".shop-items > *")[index]
  if (slot?.classList.contains("sold")) {
    void tween(
      slot,
      [
        { opacity: 0, transform: "scale(0.92)" },
        { opacity: 0, transform: "scale(0.92)", offset: 0.4 },
        { opacity: 1, transform: "scale(1)" },
      ],
      { duration: 620, easing: "cubic-bezier(0.34, 1.56, 0.64, 1)" },
    )
  }
}

/**
 * A pack: carried to the middle of the table, shaken until it splits, and the
 * cards inside dealt out of the tear. The sheet has already been rendered and
 * is standing behind the pack; its cards are held back by the deal's delay, so
 * they cannot show before the pack does.
 */
async function openPack(kept: Kept, ctx: SceneContext): Promise<void> {
  const sheet = ctx.root.querySelector<HTMLElement>(".pack-sheet")
  const options = ctx.root.querySelectorAll<HTMLElement>(".pack-options > .shop-item:not(.sold)")
  const FLY = 620
  const SHAKE = 520
  const centre = sheet ? centerOf(sheet) : { x: window.innerWidth / 2, y: window.innerHeight / 2 }
  const ghost = seat(kept)
  const from = { x: kept.rect.left + kept.rect.width / 2, y: kept.rect.top + kept.rect.height / 2 }
  const path = arc(from, centre, { lift: 80, scale: [1, 1.18], spin: 6 })
  ghost.style.opacity = "0"
  // The cards are dealt from the middle of the pack, one after another, as
  // soon as the tear opens.
  options.forEach((card, i) => {
    void deal(card, centre, {
      delay: 140 + FLY + SHAKE + 120 + i * 130,
      duration: 700,
      fan: (i - (options.length - 1) / 2) * 16,
    })
  })
  await tween(ghost, frames(path), { duration: FLY, delay: 140, easing: "linear" })
  // A held breath, then the shake climbs: each swing is wider than the last.
  ghost.style.opacity = "1"
  ghost.style.transform = "scale(1.18)"
  await tween(
    ghost,
    [
      { transform: "scale(1.18) rotate(0deg)" },
      { transform: "scale(1.2) rotate(-3deg)", offset: 0.12 },
      { transform: "scale(1.22) rotate(4deg)", offset: 0.28 },
      { transform: "scale(1.24) rotate(-6deg)", offset: 0.46 },
      { transform: "scale(1.26) rotate(8deg)", offset: 0.64 },
      { transform: "scale(1.3) rotate(-10deg)", offset: 0.82 },
      { transform: "scale(1.34) rotate(0deg)" },
    ],
    { duration: SHAKE, easing: "linear" },
  )
  // The tear: the same card twice, cut across along a zigzag, the halves
  // thrown apart.
  const cut = "polygon(0 0, 100% 0, 100% 46%, 84% 52%, 68% 45%, 52% 53%, 36% 46%, 20% 52%, 0 47%)"
  const rest =
    "polygon(0 47%, 20% 52%, 36% 46%, 52% 53%, 68% 45%, 84% 52%, 100% 46%, 100% 100%, 0 100%)"
  const top = ghost
  const bottom = ghost.cloneNode(true) as HTMLElement
  top.style.clipPath = cut
  bottom.style.clipPath = rest
  fxDom().append(bottom)
  top.style.transform = "scale(1.34)"
  bottom.style.transform = "scale(1.34)"
  const rim = rareOf(top)
  burst("spark", centre, { count: 26, speed: 640, life: 0.55, colors: ["#fff", rim, "#ffd166"] })
  burst("ring", centre, { colors: [rim] })
  const done = [
    tween(
      top,
      [
        { transform: "scale(1.34) translate(0, 0) rotate(0deg)", opacity: 1 },
        { transform: "scale(1.4) translate(-30px, -90px) rotate(-16deg)", opacity: 0 },
      ],
      { duration: 420, easing: "cubic-bezier(0.2, 0.8, 0.3, 1)" },
    ),
    tween(
      bottom,
      [
        { transform: "scale(1.34) translate(0, 0) rotate(0deg)", opacity: 1 },
        { transform: "scale(1.3) translate(24px, 110px) rotate(12deg)", opacity: 0 },
      ],
      { duration: 460, easing: "cubic-bezier(0.5, 0, 0.9, 0.6)" },
    ),
  ]
  top.style.opacity = "0"
  bottom.style.opacity = "0"
  await Promise.all(done)
  top.remove()
  bottom.remove()
}

/** What a pick does: the card goes where it lives, the others fall away. */
function pick(action: Action, events: readonly GameEvent[], ctx: SceneContext): void {
  if (!before) return
  const picked = events.find((e) => e.type === "pack_picked")
  const closed = ctx.state.pack === null
  const taken = picked?.type === "pack_picked" ? picked.taken : null
  const index = action.type === "pick_pack" ? action.index : -1
  const cards = before.pack
  if (taken && cards[index]) {
    const kept = cards[index]
    const target = seatOf(ctx.root, taken.kind, ctx.state)
    const dest = target && (taken.kind === "relic" || taken.kind === "consumable") ? target : null
    void flight(kept, target, { duration: 640, lift: 70, dest }).then(() => {
      if (target) burst("spark", centerOf(target), { count: 12, speed: 340, life: 0.45, size: 2.5 })
    })
  }
  if (!closed) return
  // What was not chosen falls off the bottom of the table, tilting as it goes.
  cards.forEach((kept, i) => {
    if (!kept || i === index) return
    const ghost = seat(kept)
    ghost.style.opacity = "0"
    const way = i % 2 ? 1 : -1
    void tween(
      ghost,
      [
        { transform: "translate(0, 0) rotate(0deg) scale(1)", opacity: 1 },
        { transform: "translate(0, -16px) rotate(0deg) scale(1.03)", opacity: 1, offset: 0.2 },
        {
          transform: `translate(${way * 40}px, ${window.innerHeight * 0.5}px) rotate(${way * 18}deg) scale(0.9)`,
          opacity: 0,
        },
      ],
      { duration: 620, delay: 120 + i * 60, easing: "cubic-bezier(0.5, 0, 0.9, 0.5)" },
    ).then(() => ghost.remove())
  })
}

function sell(index: number, ctx: SceneContext, events: readonly GameEvent[]): void {
  const kept = before?.relics[index]
  if (!kept) return
  const ghost = seat(kept)
  const centre = {
    x: kept.rect.left + kept.rect.width / 2,
    y: kept.rect.top + kept.rect.height / 2,
  }
  const rim = rareOf(ghost)
  const paid = events.find((e) => e.type === "gold")
  const gold = goldSpot(ctx.root)
  ghost.style.opacity = "0"
  // A shudder, then it comes apart: swelling, whitening and gone, while its
  // own colour goes up as shards.
  void tween(
    ghost,
    [
      { transform: "scale(1) rotate(0deg)", filter: "brightness(1)", opacity: 1 },
      {
        transform: "scale(1.06) rotate(-4deg)",
        filter: "brightness(1.2)",
        opacity: 1,
        offset: 0.15,
      },
      { transform: "scale(1.06) rotate(4deg)", filter: "brightness(1.4)", opacity: 1, offset: 0.3 },
      {
        transform: "scale(1.1) rotate(-3deg)",
        filter: "brightness(1.8)",
        opacity: 1,
        offset: 0.45,
      },
      { transform: "scale(1.3) rotate(0deg)", filter: "brightness(3) blur(3px)", opacity: 0 },
    ],
    { duration: 520, easing: "linear" },
  ).then(() => ghost.remove())
  window.setTimeout(() => {
    burst("shard", centre, { count: 16, speed: 420, life: 0.7, colors: [rim, "#fff", "#ffd166"] })
  }, ms(230))
  if (gold) {
    coins(centre, gold, coinCount(paid?.type === "gold" ? paid.delta : 2), {
      delay: 260,
      landed: () => burst("spark", gold, { count: 12, speed: 300, life: 0.4, size: 2.5 }),
    })
  }
}

function reroll(ctx: SceneContext, events: readonly GameEvent[]): void {
  const old = before?.shelf ?? []
  const away = deck()
  // The old hand is swept toward the deck it came from, in the order it was
  // dealt, and the new one is dealt behind it before the last has gone.
  old.forEach((kept, i) => {
    if (!kept) return
    const ghost = seat(kept)
    ghost.style.opacity = "0"
    const dx = away.x - (kept.rect.left + kept.rect.width / 2)
    const dy = away.y - (kept.rect.top + kept.rect.height / 2)
    void tween(
      ghost,
      [
        { transform: "translate(0, 0) rotate(0deg) scale(1)", opacity: 1 },
        { transform: "translate(-14px, 6px) rotate(-3deg) scale(1.02)", opacity: 1, offset: 0.18 },
        { transform: `translate(${dx}px, ${dy}px) rotate(38deg) scale(0.7)`, opacity: 0.9 },
      ],
      { duration: 460, delay: i * 45, easing: "cubic-bezier(0.55, 0, 0.9, 0.6)" },
    ).then(() => ghost.remove())
  })
  dealShelf(ctx.screen, 240)
  const spin = ctx.root.querySelector(".shop-actions .secondary .icon")
  void tween(spin, [{ transform: "rotate(0deg)" }, { transform: "rotate(360deg)" }], {
    duration: 520,
    easing: "cubic-bezier(0.34, 1.2, 0.64, 1)",
  })
  const paid = events.find((e) => e.type === "gold")
  const gold = goldSpot(ctx.root)
  const button = ctx.root.querySelector(".shop-actions .secondary")
  if (paid && gold && button)
    coins(gold, centerOf(button), coinCount(paid.type === "gold" ? paid.delta : 3))
}

/** Called after every dispatched action's render, when juiced. */
export function playShopEvents(
  action: Action,
  events: readonly GameEvent[],
  ctx: SceneContext,
): void {
  if (!juiced()) return
  switch (action.type) {
    case "buy":
      buy(action.index, ctx, events)
      break
    case "sell_relic":
      sell(action.index, ctx, events)
      break
    case "reroll":
      reroll(ctx, events)
      break
    case "pick_pack":
    case "skip_pack":
      pick(action, events, ctx)
      break
    default:
      // Arriving is the one thing that is not a reply to a tap. The transition
      // to the shop is phase 6's; the shelf, dealt onto it, is this scene's.
      if (events.some((e) => e.type === "shop_entered")) dealShelf(ctx.screen, 320)
  }
  before = null
}

/* --------------------------------------------------------------- pointer */

/**
 * Cards answer the mouse: leaning toward the pointer in 3D under a light that
 * follows it, and, where the price is out of reach, a red shake on the price.
 *
 * Delegated from the document because the screen is rebuilt on every dispatch
 * and a listener on a card would die with it. Decided on `pointerType` per
 * event and never on a media query, for the reason `bindTips` gives: the APK's
 * WebView answers `(hover: hover)` like a desktop. The tilt is a CSS variable
 * and not a style the script owns, so a card whose class is taken away rests
 * where the stylesheet puts it, and a render that deletes the card takes the
 * tilt with it and leaves nothing behind.
 */
const TILTABLE =
  ".shop-screen .shop-item:not(.sold), .pack-sheet .shop-item:not(.sold), .relic:not(.empty)"

let bound = false
let hot: HTMLElement | null = null
let hotRect: DOMRect | null = null
let queued: PointerEvent | null = null

function untilt(): void {
  if (hot) {
    hot.classList.remove("tilting")
    for (const name of ["--rx", "--ry", "--mx", "--my", "--px", "--py"])
      hot.style.removeProperty(name)
  }
  hot = null
  hotRect = null
}

function tiltTo(event: PointerEvent): void {
  if (!hot || !hotRect) return
  const x = Math.max(0, Math.min(1, (event.clientX - hotRect.left) / hotRect.width))
  const y = Math.max(0, Math.min(1, (event.clientY - hotRect.top) / hotRect.height))
  // Ten degrees at the edge: enough to see the card lean, little enough that
  // the text on it stays readable at the extreme. The sign is the lean toward
  // the pointer: the near edge goes down.
  hot.style.setProperty("--rx", `${((0.5 - y) * 2 * 9).toFixed(2)}deg`)
  hot.style.setProperty("--ry", `${((x - 0.5) * 2 * 11).toFixed(2)}deg`)
  hot.style.setProperty("--mx", `${(x * 100).toFixed(1)}%`)
  hot.style.setProperty("--my", `${(y * 100).toFixed(1)}%`)
  hot.style.setProperty("--px", x.toFixed(3))
  hot.style.setProperty("--py", y.toFixed(3))
}

function bindPointer(): void {
  if (bound || typeof document === "undefined") return
  bound = true
  document.addEventListener(
    "pointermove",
    (event) => {
      if (event.pointerType !== "mouse" || !juiced()) {
        untilt()
        return
      }
      const target = event.target instanceof Element ? event.target : null
      const card = target?.closest<HTMLElement>(TILTABLE) ?? null
      if (card !== hot) {
        untilt()
        if (card) {
          // Measured before the lean, once: a rect taken from a card already
          // turned in 3D moves under the pointer that is turning it.
          hot = card
          hotRect = card.getBoundingClientRect()
          card.classList.add("tilting")
        }
      }
      if (!hot) return
      queued = event
      requestAnimationFrame(() => {
        if (queued) tiltTo(queued)
        queued = null
      })
    },
    { passive: true },
  )
  // Leaving the window. Moving off a card onto the table is the `pointermove`
  // above, which finds no card; leaving the page sends no move at all, and this
  // used to listen for `pointerleave` on the document, which does not bubble and
  // never reaches it, so a card the pointer left the window from stayed leaning.
  // `pointerout` bubbles, and its `relatedTarget` is null only when the pointer
  // has gone somewhere outside the page.
  document.addEventListener(
    "pointerout",
    (event) => {
      if (!event.relatedTarget) untilt()
    },
    { passive: true },
  )
  // A price that cannot be met shakes red. The click is seen on its way down,
  // ahead of the handler that refuses it, and nothing is rebuilt by a refusal,
  // so the class it leaves is still on the price when the toast arrives.
  document.addEventListener(
    "click",
    (event) => {
      if (!juiced() || !(event.target instanceof Element)) return
      const card = event.target.closest(".shop-screen .shop-item.broke")
      replay(card?.querySelector(".shop-item-cost"), "deny", 480)
    },
    true,
  )
}
