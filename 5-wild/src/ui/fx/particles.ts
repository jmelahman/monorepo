import { fxCanvas, type Point } from "./layer"
import { juiced, rate } from "./motion"

/**
 * Sparks, coins, shards and confetti, drawn on one canvas.
 *
 * A canvas rather than DOM nodes because of what a burst is: forty things that
 * each live for half a second and move every frame. As elements that is forty
 * insertions, forty style recalcs a frame and forty removals, all landing in the
 * same frames as the scoring sequence's own DOM work, which is the work the
 * player is actually watching. On one 2D canvas it is forty `fillRect`s.
 *
 * Three limits keep it inside the budget (desktop Chromium, 60fps on an
 * integrated GPU; see `plans/ui-overhaul.md`):
 *
 * - The pool is fixed at `CAP`. A burst that would exceed it spawns what fits
 *   and drops the rest, so a thirty-relic chain degrades to fewer sparks
 *   rather than to fewer frames.
 * - The backing store is capped at 2x device pixels. A 4K laptop at 3x would
 *   otherwise be clearing and compositing a canvas nine times the area of the
 *   window for particles two pixels wide.
 * - The loop sleeps. With nothing alive it clears once and stops asking for
 *   frames, so the canvas costs nothing between moments.
 *
 * Nothing here runs unless `juiced()`: on a phone, or with reduced motion,
 * `burst` returns before the canvas is created, so the layer never exists.
 * Particles are pure decoration by construction: nothing a player needs to
 * know is ever said only by one, which is what makes dropping them safe.
 */

export type Kind = "spark" | "coin" | "shard" | "confetti" | "ember" | "ring"

type Particle = {
  kind: Kind
  x: number
  y: number
  vx: number
  vy: number
  /** Seconds lived, and seconds allowed. */
  age: number
  life: number
  size: number
  spin: number
  angle: number
  gravity: number
  drag: number
  color: string
}

const CAP = 600
const MAX_DPR = 2

const live: Particle[] = []
const spare: Particle[] = []

let ctx: CanvasRenderingContext2D | null = null
let frame = 0
let last = 0
let width = 0
let height = 0

export type Burst = {
  count?: number
  /** Launch speed in px/s, before the random spread. */
  speed?: number
  /** Direction in radians (0 is right, -PI/2 up) and the cone around it. */
  angle?: number
  spread?: number
  /** Seconds. */
  life?: number
  size?: number
  gravity?: number
  colors?: readonly string[]
}

/** How each kind behaves when a burst does not say otherwise. */
const DEFAULTS: Record<Kind, Required<Burst> & { drag: number }> = {
  spark: {
    count: 18,
    speed: 520,
    angle: -Math.PI / 2,
    spread: Math.PI * 2,
    life: 0.45,
    size: 3,
    gravity: 900,
    drag: 2.4,
    colors: ["#fff4c2", "#ffd166", "#ff9f1c"],
  },
  coin: {
    count: 8,
    speed: 420,
    angle: -Math.PI / 2,
    spread: Math.PI * 0.7,
    life: 0.9,
    size: 7,
    gravity: 1400,
    drag: 0.6,
    colors: ["#ffd166", "#f4b400"],
  },
  shard: {
    count: 14,
    speed: 380,
    angle: -Math.PI / 2,
    spread: Math.PI * 2,
    life: 0.8,
    size: 6,
    gravity: 1600,
    drag: 0.8,
    colors: ["#dfe8ff", "#9fb4d9", "#ffffff"],
  },
  confetti: {
    count: 60,
    speed: 700,
    angle: -Math.PI / 2,
    spread: Math.PI * 0.6,
    life: 2.2,
    size: 7,
    gravity: 700,
    drag: 1.6,
    colors: ["#ff4d6d", "#ffd166", "#06d6a0", "#4cc9f0", "#b388ff"],
  },
  ember: {
    count: 6,
    speed: 90,
    angle: -Math.PI / 2,
    spread: Math.PI * 0.5,
    life: 0.9,
    size: 4,
    gravity: -260,
    drag: 1.2,
    colors: ["#ffd166", "#ff7b00", "#ff3d00"],
  },
  ring: {
    count: 1,
    speed: 0,
    angle: 0,
    spread: 0,
    life: 0.45,
    size: 140,
    gravity: 0,
    drag: 0,
    colors: ["#ffffff"],
  },
}

function ensure(): CanvasRenderingContext2D | null {
  if (ctx) return ctx
  const canvas = fxCanvas()
  ctx = canvas.getContext("2d")
  resize()
  window.addEventListener("resize", resize)
  return ctx
}

function resize(): void {
  if (!ctx) return
  const canvas = ctx.canvas
  const dpr = Math.min(MAX_DPR, window.devicePixelRatio || 1)
  width = window.innerWidth
  height = window.innerHeight
  canvas.width = Math.round(width * dpr)
  canvas.height = Math.round(height * dpr)
  canvas.style.width = `${width}px`
  canvas.style.height = `${height}px`
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
}

/**
 * Throw a burst of `kind` from a point in layer coordinates. Returns how many
 * were actually spawned, which is fewer than asked when the pool is full, and
 * zero whenever the table's effects are off.
 */
export function burst(kind: Kind, at: Point, options: Burst = {}): number {
  if (!juiced() || !ensure()) return 0
  const base = DEFAULTS[kind]
  const count = Math.min(options.count ?? base.count, CAP - live.length)
  const speed = options.speed ?? base.speed
  const angle = options.angle ?? base.angle
  const spread = options.spread ?? base.spread
  const life = options.life ?? base.life
  const size = options.size ?? base.size
  const gravity = options.gravity ?? base.gravity
  const colors = options.colors ?? base.colors
  for (let i = 0; i < count; i++) {
    const p = spare.pop() ?? ({} as Particle)
    const heading = angle + (Math.random() - 0.5) * spread
    // Speeds are spread from 40% to 100% of the launch speed, so a burst has a
    // dense core and a ragged edge rather than an expanding ring of equals.
    const v = speed * (0.4 + Math.random() * 0.6)
    p.kind = kind
    p.x = at.x
    p.y = at.y
    p.vx = Math.cos(heading) * v
    p.vy = Math.sin(heading) * v
    p.age = 0
    p.life = life * (0.7 + Math.random() * 0.6)
    p.size = size * (0.7 + Math.random() * 0.6)
    p.spin = (Math.random() - 0.5) * 14
    p.angle = Math.random() * Math.PI * 2
    p.gravity = gravity
    p.drag = base.drag
    p.color = colors[Math.floor(Math.random() * colors.length)] ?? "#fff"
    live.push(p)
  }
  wake()
  return count
}

function wake(): void {
  if (frame) return
  last = performance.now()
  frame = requestAnimationFrame(tick)
}

function tick(now: number): void {
  // Clamped so a tab that was hidden mid-burst resumes where it was rather
  // than integrating a ten-second step and teleporting every particle off
  // screen. Scaled by the player's speed, since a burst is a one-shot motion
  // like any other and ×3 should finish it three times sooner. Floored at zero
  // because `now` is the frame's start and `last` came from `performance.now()`
  // in `wake`, which can be later: a burst thrown mid-frame took its first step
  // backwards, a few ms of every particle running the wrong way.
  const dt = (Math.max(0, Math.min(50, now - last)) / 1000) * rate()
  last = now
  const c = ctx
  if (!c) {
    frame = 0
    return
  }
  c.clearRect(0, 0, width, height)
  for (let i = live.length - 1; i >= 0; i--) {
    const p = live[i] as Particle
    p.age += dt
    if (p.age >= p.life || !juiced()) {
      // Swap-remove: order does not matter, and splicing a 600-long array
      // from the middle every frame does.
      live[i] = live[live.length - 1] as Particle
      live.pop()
      spare.push(p)
      continue
    }
    const damp = Math.exp(-p.drag * dt)
    p.vx *= damp
    p.vy = p.vy * damp + p.gravity * dt
    p.x += p.vx * dt
    p.y += p.vy * dt
    p.angle += p.spin * dt
    draw(c, p)
  }
  c.globalAlpha = 1
  if (live.length) frame = requestAnimationFrame(tick)
  else frame = 0
}

function draw(c: CanvasRenderingContext2D, p: Particle): void {
  const t = p.age / p.life
  // Everything fades over its last third; nothing pops out of existence.
  c.globalAlpha = t < 0.66 ? 1 : 1 - (t - 0.66) / 0.34
  c.fillStyle = p.color
  switch (p.kind) {
    case "spark":
    case "ember": {
      const s = p.size * (1 - t * 0.6)
      c.fillRect(p.x - s / 2, p.y - s / 2, s, s)
      break
    }
    case "coin": {
      // A coin turning in the air is an ellipse whose width follows the spin.
      const w = Math.abs(Math.cos(p.angle)) * p.size + 1
      c.beginPath()
      c.ellipse(p.x, p.y, w, p.size, 0, 0, Math.PI * 2)
      c.fill()
      break
    }
    case "shard":
    case "confetti": {
      c.save()
      c.translate(p.x, p.y)
      c.rotate(p.angle)
      if (p.kind === "shard") {
        c.beginPath()
        c.moveTo(0, -p.size)
        c.lineTo(p.size * 0.6, p.size * 0.7)
        c.lineTo(-p.size * 0.6, p.size * 0.4)
        c.closePath()
        c.fill()
      } else {
        // Confetti flutters: its visible height is the cosine of its spin.
        c.fillRect(
          -p.size / 2,
          (-p.size / 4) * Math.cos(p.angle * 2),
          p.size,
          (p.size / 2) * Math.cos(p.angle * 2),
        )
      }
      c.restore()
      break
    }
    case "ring": {
      const r = p.size * (0.15 + 0.85 * (1 - (1 - t) ** 3))
      c.globalAlpha = (1 - t) * 0.9
      c.strokeStyle = p.color
      c.lineWidth = 6 * (1 - t) + 1
      c.beginPath()
      c.arc(p.x, p.y, r, 0, Math.PI * 2)
      c.stroke()
      break
    }
  }
}

/** Drop everything in flight, for a screen that no longer has a place for it. */
export function clearParticles(): void {
  spare.push(...live)
  live.length = 0
  ctx?.clearRect(0, 0, width, height)
}
