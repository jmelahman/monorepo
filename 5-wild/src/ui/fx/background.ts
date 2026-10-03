import { hasFx } from "../skin"
import { reduced } from "./motion"

/**
 * The smoke over the table: two slow banks of it, teal and wine, drifting over
 * a near-black ground, drawn by a fragment shader on a canvas behind `#app` at a
 * third of the window's size in CSS pixels and stretched with
 * `image-rendering: pixelated`.
 *
 * It replaced a lamp on midnight baize with dust turning in the beam, which was
 * a room; this is a weather. The picture is made to look like a low-resolution
 * game's: the smoke's density is quantised to four levels and the steps between
 * them are broken up with an ordered (Bayer) dither, so the edge of a bank is a
 * stipple of hard 3px squares rather than a soft gradient, and the whole thing
 * is deliberately too coarse to fight a letter. Nothing in it is brighter than
 * about a tenth of a tile, and a vignette pulls the corners to the ground so the
 * board sits in the dark.
 *
 * Mood is the smoke's colour, not a repaint. A boss round turns both banks red
 * and pulls the teal out; the shop is the teal at full and the wine mostly gone;
 * a win is mint and cash; a loss drains to grey and thins. The same mood goes to
 * the root as `data-mood`, so any panel can answer it in CSS.
 *
 * The flat ground in `styles/skins/smoke/backdrop.css` stays underneath and is
 * what the game looks like when this does not run: no WebGL, a lost context,
 * another skin, and a reduced-motion frame not yet painted. The canvas fades in only
 * once a frame has been painted, so a failure at any step leaves the ground
 * rather than a hole.
 *
 * What it is allowed to cost:
 *
 * - A third of the window in CSS pixels, DPR ignored on purpose: the grain is
 *   meant to be the same size on every screen, and at 1440x900 that is 480x300
 *   fragments, which is the cheapest a full-window shader can be.
 * - 20 updates a second, not 60. The drift is slow enough that nobody sees the
 *   steps in it, and on a picture made of 3px squares a smooth 60fps glide is the
 *   one thing that would look out of place. When the frame rate says the machine
 *   is struggling it drops to 10: frames are timed by `requestAnimationFrame`
 *   deltas, deliberately, since the shader is not the only thing on the page and
 *   shedding load gives the scoring effects their budget back. 45 frames
 *   averaging over 22ms and it never climbs back: a game that flips between
 *   rates every few seconds looks worse than one that settles.
 * - Nothing when nobody can see it: a hidden tab, and while a sheet is open,
 *   since a sheet rises over a board that is dimmed and not being read.
 * - The same on the phone, which is where it is cheapest: the canvas is a third
 *   of the window in CSS pixels, so a 390x844 screen is 130x282 fragments, a
 *   sixth of the desktop's, at the same 20 updates a second and with the same
 *   step-down to 10. It is a property of the look and not of the layout, so the
 *   Smoke Room on a phone has its smoke, and every other look on either layout
 *   creates no canvas, no context and no listener but the class watcher, until
 *   the player has visited the room once (see `sync`).
 *
 * Reduced motion gets exactly one frame and then stops, and that frame is a
 * painted picture: motion is stopped, never compressed, so there is no
 * slow-motion version. The mood still changes, snapped, one frame per change,
 * because the colour is information (a boss round is red) and only its easing is
 * motion. The drift is a loop nobody is waiting on, so it is not on `--pace`.
 */

export type Mood = "round" | "boss" | "shop" | "victory" | "game_over"

type RGB = [number, number, number]

type Look = {
  /** The first bank's colour (the teal, at the round), linear-ish 0..1. */
  a: RGB
  /** The second bank's colour (the wine). */
  b: RGB
  /** How much of the banks shows over the ground: 0 is bare, 1 is the round's own. */
  power: number
  /** 1 is the colours as they are, 0 is grey. */
  sat: number
}

const RED: RGB = [0.42, 0.07, 0.1]
const MINT: RGB = [0.22, 0.6, 0.5]
const CASH: RGB = [0.5, 0.36, 0.1]

/**
 * The moods as targets, built once the tokens can be read. Kept dark on
 * purpose: everything on the table is lit better than the smoke, and a red that
 * was bright enough to shout was measured against the tile greens in the casino
 * build and lost them (yellow on crimson stopped reading as yellow), so the boss
 * is a dim red and the win is the only mood allowed to be brighter than the
 * round.
 */
let LOOKS: Record<Mood, Look> = fallbackLooks()

function looksFrom(a: RGB, b: RGB): Record<Mood, Look> {
  return {
    round: { a, b, power: 1, sat: 1 },
    boss: { a: RED, b, power: 0.9, sat: 1 },
    shop: { a, b: [b[0] * 0.4, b[1] * 0.4, b[2] * 0.4], power: 1.1, sat: 1 },
    victory: { a: MINT, b: CASH, power: 1.35, sat: 1.1 },
    game_over: { a, b, power: 0.4, sat: 0.1 },
  }
}

/** The literals the tokens have, for a page whose stylesheet has not landed. */
function fallbackLooks(): Record<Mood, Look> {
  return looksFrom([0.055, 0.165, 0.18], [0.165, 0.05, 0.094])
}

/** How long a mood takes to settle, in seconds: the time constant is a quarter of it. */
const EASE = 1.2
const FRAME_MS_CAP = 22
const SAMPLES = 45
const WARMUP_MS = 2000
/** CSS pixels per canvas pixel: the size of one dither cell. */
const CELL = 3
/** Milliseconds between updates: 20 a second. Doubled when the machine is struggling. */
const TICK_MS = 50
/** The one frame reduced motion draws: a time at which the banks are unremarkable. */
const STILL_T = 40

const VERT = `attribute vec2 p;void main(){gl_Position=vec4(p,0.,1.);}`

/**
 * Three parts. The smoke is fractal value noise (four octaves), each bank a
 * different sample of it moving a different way at about a hundredth of the
 * window a second, and warped by a slower sample of itself so the banks curl
 * instead of sliding. The quantiser is the look: density is scaled to four
 * levels and the fractional part is compared against a 4x4 Bayer matrix at the
 * fragment's own coordinates, which is what turns a gradient into stipple. The
 * vignette is a mix toward the darkest colour by distance from the middle, and
 * is applied after the quantiser so it stays smooth-edged in level only.
 */
const FRAG = `
#ifdef GL_FRAGMENT_PRECISION_HIGH
precision highp float;
#else
precision mediump float;
#endif
uniform vec2 uRes;
uniform float uT;
uniform vec3 uGround, uVig, uA, uB;
uniform vec2 uLook; // power, saturation
float hash(vec2 q){ return fract(sin(dot(q, vec2(12.9898, 78.233))) * 43758.5453); }
float vnoise(vec2 q){
  vec2 i = floor(q);
  vec2 f = fract(q);
  f = f * f * (3.0 - 2.0 * f);
  return mix(mix(hash(i), hash(i + vec2(1.0, 0.0)), f.x),
             mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), f.x), f.y);
}
float fbm(vec2 q){
  float v = 0.0;
  float a = 0.5;
  for (int k = 0; k < 4; k++) { v += a * vnoise(q); q = q * 2.03 + 11.7; a *= 0.5; }
  return v;
}
float bayer2(vec2 p){ return mod(2.0 * p.x + 3.0 * p.y, 4.0); }
float bayer4(vec2 p){
  vec2 q = mod(floor(p), 4.0);
  return (4.0 * bayer2(mod(q, 2.0)) + bayer2(floor(q / 2.0)) + 0.5) / 16.0;
}
float level(float d, vec2 frag){
  // Four levels, dithered: the fractional part decides which of two neighbours.
  float x = clamp((d - 0.28) * 2.6, 0.0, 1.0) * 4.0;
  return floor(x + bayer4(frag)) / 4.0;
}
void main(){
  vec2 frag = gl_FragCoord.xy;
  vec2 c = frag / uRes;
  float asp = uRes.x / uRes.y;
  vec2 uv = c * vec2(asp, 1.0) * 2.2;
  float t = uT * 0.012;
  vec2 w = vec2(fbm(uv + vec2(t * 3.0, 0.0)), fbm(uv + vec2(5.2, 1.3 - t * 2.0)));
  float da = fbm(uv * 0.9 + w * 1.2 + vec2(t * 4.0, t));
  float db = fbm(uv * 1.1 + w * 1.4 + vec2(-t * 3.0, 7.0 + t * 2.0));
  float la = level(da, frag) * uLook.x;
  float lb = level(db, frag) * uLook.x;
  vec3 col = uGround + uA * la * 0.7 + uB * lb * 0.7;
  float l = dot(col, vec3(0.299, 0.587, 0.114));
  col = mix(vec3(l), col, uLook.y);
  float v = smoothstep(0.3, 0.95, length((c - 0.5) * vec2(1.1, 1.25)));
  col = mix(col, uVig, v * 0.85);
  gl_FragColor = vec4(col, 1.0);
}`

type Gpu = {
  gl: WebGLRenderingContext
  uRes: WebGLUniformLocation | null
  uT: WebGLUniformLocation | null
  uA: WebGLUniformLocation | null
  uB: WebGLUniformLocation | null
  uLook: WebGLUniformLocation | null
}

let canvas: HTMLCanvasElement | null = null
let gpu: Gpu | null = null
let raf = 0
let watching = false

/** Wanted, not current: `setMood` may be called before the canvas exists. */
let target: Mood = "round"
let covered = false
let cur: Look = copy(LOOKS.round)
let t = 0
let last = 0
let lastDraw = 0
let began = 0
let stale = true
let capped = false
/**
 * Set once a build has failed, so the flat ground is the answer for the rest of
 * the session. `sync` runs on every change to the root's class, and without
 * this each one tried again: a new canvas and, on a GPU that compiles but will
 * not link, a new live context every time the pointer moved between a mouse
 * and a finger.
 */
let unable = false
const deltas: number[] = []

function copy(l: Look): Look {
  return { ...l, a: [...l.a], b: [...l.b] }
}

/** Parse `#rrggbb` from a token, falling back to the literal the token has. */
function token(name: string, fallback: RGB): RGB {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  const m = /^#([0-9a-f]{6})$/i.exec(raw)
  if (!m?.[1]) return fallback
  const n = Number.parseInt(m[1], 16)
  return [(n >> 16) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255]
}

function compile(gl: WebGLRenderingContext, type: number, src: string): WebGLShader | null {
  const shader = gl.createShader(type)
  if (!shader) return null
  gl.shaderSource(shader, src)
  gl.compileShader(shader)
  if (gl.getShaderParameter(shader, gl.COMPILE_STATUS)) return shader
  gl.deleteShader(shader)
  return null
}

function build(el: HTMLCanvasElement): Gpu | null {
  const gl = el.getContext("webgl", {
    alpha: false,
    antialias: false,
    depth: false,
    stencil: false,
    powerPreference: "low-power",
    preserveDrawingBuffer: false,
  })
  if (!gl) return null
  // A context that failed to build is handed back rather than left to the
  // collector, which is in no hurry: browsers cap live contexts at about
  // sixteen and drop the oldest with a console warning when a page goes past.
  const fail = () => {
    gl.getExtension("WEBGL_lose_context")?.loseContext()
    return null
  }
  const vs = compile(gl, gl.VERTEX_SHADER, VERT)
  const fs = compile(gl, gl.FRAGMENT_SHADER, FRAG)
  const prog = gl.createProgram()
  if (!vs || !fs || !prog) return fail()
  gl.attachShader(prog, vs)
  gl.attachShader(prog, fs)
  gl.linkProgram(prog)
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) return fail()
  gl.useProgram(prog)
  const buf = gl.createBuffer()
  gl.bindBuffer(gl.ARRAY_BUFFER, buf)
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW)
  const at = gl.getAttribLocation(prog, "p")
  gl.enableVertexAttribArray(at)
  gl.vertexAttribPointer(at, 2, gl.FLOAT, false, 0, 0)
  const u = (name: string) => gl.getUniformLocation(prog, name)
  gl.uniform3fv(u("uGround"), token("--ground", [0.024, 0.027, 0.039]))
  gl.uniform3fv(u("uVig"), token("--vig", [0.008, 0.012, 0.02]))
  // The banks' colours are read here, with the rest of the tokens, and the
  // moods are rebuilt from them: a skin that retunes `--smoke-a` retunes the
  // round, the shop and the loss with it, and only the boss's red and the
  // win's mint and cash are this file's own.
  LOOKS = looksFrom(
    token("--smoke-a", [0.055, 0.165, 0.18]),
    token("--smoke-b", [0.165, 0.05, 0.094]),
  )
  cur = copy(LOOKS[target])
  return { gl, uRes: u("uRes"), uT: u("uT"), uA: u("uA"), uB: u("uB"), uLook: u("uLook") }
}

function fitCanvas(): void {
  if (!canvas || !gpu) return
  const w = Math.max(1, Math.ceil(window.innerWidth / CELL))
  const h = Math.max(1, Math.ceil(window.innerHeight / CELL))
  if (canvas.width === w && canvas.height === h) return
  canvas.width = w
  canvas.height = h
  gpu.gl.viewport(0, 0, w, h)
  stale = true
}

function draw(): void {
  if (!gpu || !canvas) return
  const { gl } = gpu
  gl.uniform2f(gpu.uRes, canvas.width, canvas.height)
  gl.uniform1f(gpu.uT, t)
  gl.uniform3fv(gpu.uA, cur.a)
  gl.uniform3fv(gpu.uB, cur.b)
  gl.uniform2f(gpu.uLook, cur.power, cur.sat)
  gl.drawArrays(gl.TRIANGLES, 0, 3)
  stale = false
  canvas.classList.add("live")
}

/** Move `cur` toward the target look; true while it still has ground to cover. */
function ease(dt: number): boolean {
  const goal = LOOKS[target]
  const k = 1 - Math.exp(-dt / (EASE / 4))
  let gap = 0
  const step = (a: number, b: number): number => {
    gap = Math.max(gap, Math.abs(b - a))
    return a + (b - a) * k
  }
  cur = {
    a: [step(cur.a[0], goal.a[0]), step(cur.a[1], goal.a[1]), step(cur.a[2], goal.a[2])],
    b: [step(cur.b[0], goal.b[0]), step(cur.b[1], goal.b[1]), step(cur.b[2], goal.b[2])],
    power: step(cur.power, goal.power),
    sat: step(cur.sat, goal.sat),
  }
  if (gap > 0.002) return true
  cur = copy(goal)
  return false
}

const running = (): boolean =>
  canvas !== null && gpu !== null && !document.hidden && !covered && hasFx() && !reduced()

/** The room's own smoke: only the Smoke Room draws it. */
const lit = (): boolean => hasFx()

function frame(now: number): void {
  raf = 0
  if (!running()) return
  const dt = last ? Math.min(0.1, (now - last) / 1000) : 0
  if (!began) began = now
  // The first 2s are not sampled: shader compile, font load and the first
  // render all land there, and a cap decided by them is permanent. Measured in
  // headless Chromium at 4x throttle, where every later frame was 16.7ms and the
  // cap tripped anyway on the ones before.
  if (last && now - began > WARMUP_MS) {
    deltas.push(now - last)
    if (deltas.length > SAMPLES) deltas.shift()
    if (!capped && deltas.length === SAMPLES) {
      const mean = deltas.reduce((a, b) => a + b, 0) / SAMPLES
      if (mean > FRAME_MS_CAP) {
        capped = true
        if (canvas) canvas.dataset.fps = "10"
      }
    }
  }
  last = now
  raf = requestAnimationFrame(frame)
  // A hair under the period: rAF ticks at 16.7 or 8.3 depending on the display,
  // and a threshold on the tick itself would skip a beat when a tick lands
  // early.
  if (now - lastDraw < (capped ? TICK_MS * 2 : TICK_MS) - 4) return
  const sinceDraw = lastDraw ? Math.min(0.1, (now - lastDraw) / 1000) : dt
  lastDraw = now
  t += sinceDraw
  ease(sinceDraw)
  draw()
}

/** Start or stop the loop, and paint the one frame a still picture needs. */
function kick(): void {
  if (!canvas || !gpu) return
  if (running()) {
    if (!raf) {
      last = 0
      lastDraw = 0
      began = 0
      raf = requestAnimationFrame(frame)
    }
    return
  }
  if (raf) {
    cancelAnimationFrame(raf)
    raf = 0
  }
  // Reduced motion: one frame, then nothing. A hidden tab or an open sheet is
  // not painted, since nobody is looking, and `stale` sees to it on return.
  if (stale && !document.hidden && !covered && lit()) {
    ease(10)
    t = STILL_T
    draw()
  }
}

function create(): void {
  if (canvas || unable) return
  const el = document.createElement("canvas")
  el.id = "smoke"
  el.setAttribute("aria-hidden", "true")
  el.dataset.fps = "20"
  const g = build(el)
  if (!g) {
    unable = true
    return
  }
  canvas = el
  gpu = g
  const app = document.getElementById("app")
  if (app) app.before(el)
  else document.body.prepend(el)
  el.addEventListener("webglcontextlost", (e) => {
    // Without this the browser never restores the context. The canvas is
    // hidden meanwhile so the flat ground shows, which is the fallback and is
    // never a blank.
    e.preventDefault()
    if (raf) cancelAnimationFrame(raf)
    raf = 0
    gpu = null
    el.classList.remove("live")
  })
  el.addEventListener("webglcontextrestored", () => {
    gpu = build(el)
    if (!gpu) return
    stale = true
    fitCanvas()
    kick()
  })
  fitCanvas()
  stale = true
  kick()
}

/**
 * Make the canvas match whether the look has one and it is on.
 *
 * Created the first time the look is Smoke and kept from then on, stopped and
 * hidden (`#smoke` in `base.css`) under every other look. It used to be removed
 * on the way out and built again on the way back, which put a new WebGL context
 * and a shader compile behind every pass of the dial and replayed the 600ms
 * fade from the flat ground each time, so the room arrived in two steps. Kept,
 * it comes back on its last frame with `.live` still on it, and `stale` has the
 * next one drawn at once. The price is one idle context for the rest of the
 * session, and only for a player who has looked at the room at least once.
 */
function sync(): void {
  if (lit()) {
    create()
    stale = true
  }
  // Starts the loop under Smoke and stops it under anything else, since
  // `running` asks the look; idempotent, so a class change that is not a change
  // of look costs nothing.
  kick()
}

/**
 * Begin following the look. Called once by the shell after it lands. The look
 * can change at any moment (a tap on the dial, the window moving the default),
 * so this watches the root's class rather than being told, and the loop is
 * started and stopped to match. Nothing here creates a node until the skin says
 * it has the smoke.
 */
export function startBackground(): void {
  if (watching) return
  watching = true
  new MutationObserver(sync).observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class"],
  })
  window.addEventListener("resize", () => {
    fitCanvas()
    kick()
  })
  document.addEventListener("visibilitychange", kick)
  if (typeof matchMedia === "function") {
    matchMedia("(prefers-reduced-motion: reduce)").addEventListener("change", () => {
      stale = true
      kick()
    })
  }
  sync()
}

/**
 * Tell the background what kind of screen this is, and whether a sheet is over
 * it. Called from every render; cheap when nothing changed.
 */
export function setMood(mood: Mood, sheetOpen = false): void {
  const moved = mood !== target
  target = mood
  const wasCovered = covered
  covered = sheetOpen
  // Only where something reads it, which is the look with the smoke; the
  // attribute is dropped on the way out so it never outlives its reader.
  if (hasFx()) document.documentElement.dataset.mood = mood
  else document.documentElement.removeAttribute("data-mood")
  if (moved) stale = true
  if (moved || wasCovered !== covered) kick()
}

/** For tests and the report: is the loop alive, and at what cadence. */
export const backgroundState = (): { drawing: boolean; capped: boolean; reduced: boolean } => ({
  drawing: raf !== 0,
  capped,
  reduced: reduced(),
})
