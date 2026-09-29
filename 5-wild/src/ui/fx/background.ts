import { isTable } from "../table"
import { startMarquee, stopMarquee } from "./marquee"
import { juiced, reduced } from "./motion"

/**
 * The lamp over the table: midnight baize, one warm pool of light on it, dust
 * turning in the beam, all drawn by a fragment shader on a canvas behind `#app`.
 *
 * What it replaced was a slow paint-swirl of the felt's palette, which is the
 * one background everybody has seen in another card game. This one is a room:
 * fabric grain under a lamp that breathes and drifts a few pixels, and motes
 * that only exist where the beam is. Nothing in it has an edge to fight a letter
 * with, and the whole picture is darker than any tile.
 *
 * Mood is the light, not a repaint. A boss round is the lamp turned red and
 * pulled in tight; the shop is a warm amber pool; a win is the lamp at full and
 * gold; a loss is the lamp dying to a cold grey. The baize underneath keeps its
 * navy, so it reads as the same table lit differently, which is what a repaint of
 * the cloth never did. The same mood also goes to the root as `data-mood`, so
 * the marquee bulbs and any panel can answer it in CSS.
 *
 * The static felt in `styles/table/backdrop.css` stays underneath and is what the
 * game looks like when this does not run: no WebGL, a lost context, the phone,
 * the light theme, the "Table lights" setting off. The canvas fades in only once
 * a frame has been painted, so a failure at any step leaves the CSS felt rather
 * than a hole.
 *
 * What it is allowed to cost:
 *
 * - Half resolution, DPR included. The picture is a soft glow and the motes are
 *   meant to be soft, so the browser's bilinear stretch is the depth of field.
 *   At DPR 2 that is one canvas pixel per CSS pixel.
 * - 30fps when the frame rate says the machine is struggling. Frames are timed
 *   by `requestAnimationFrame` deltas, deliberately: the shader is not the only
 *   thing on the page, and shedding load is to give the scoring effects their
 *   budget back. 45 frames averaging over 22ms and it draws every other frame
 *   from then on. It never climbs back: a game that flips between 60 and 30
 *   every few seconds looks worse than one that settles on 30.
 * - Nothing when nobody can see it: a hidden tab, and while a sheet is open,
 *   since a sheet rises over a board that is dimmed and not being read.
 * - Nothing at all on the phone: no canvas, no context, no listener but the
 *   class watcher.
 *
 * Reduced motion gets exactly one frame and then stops, and that frame is a
 * painted picture: motion is stopped, never compressed, so there is no
 * slow-motion version. The mood still changes, snapped, one frame per change,
 * because the light is information (a boss round is red) and only its easing is
 * motion.
 */

export type Mood = "round" | "boss" | "shop" | "victory" | "game_over"

type Look = {
  /** The lamp's colour, linear-ish 0..1. */
  lamp: [number, number, number]
  /** How much light: 0 is a dead bulb, 1 is the round's own. */
  power: number
  /** Pool radius as a fraction of the screen's diagonal. */
  radius: number
  /** 1 is the baize as it is, 0 is grey. */
  sat: number
  /** Motes shown, 0..1. */
  dust: number
}

/**
 * The moods as targets. Kept dark on purpose: everything on the felt is lit
 * better than the felt, and a red that was bright enough to shout was measured
 * against the tile greens and lost them (yellow on crimson stopped reading as
 * yellow), so the boss lamp is a dim, tight red and the win is the only mood
 * allowed to be brighter than the round.
 */
const LOOKS: Record<Mood, Look> = {
  round: { lamp: [1, 0.8, 0.5], power: 1, radius: 0.55, sat: 1, dust: 1 },
  boss: { lamp: [1, 0.22, 0.2], power: 0.85, radius: 0.4, sat: 1, dust: 0.7 },
  shop: { lamp: [1, 0.66, 0.3], power: 1.1, radius: 0.62, sat: 1, dust: 1 },
  victory: { lamp: [1, 0.85, 0.4], power: 1.5, radius: 0.75, sat: 1.1, dust: 1 },
  game_over: { lamp: [0.55, 0.65, 0.85], power: 0.32, radius: 0.35, sat: 0.15, dust: 0.25 },
}

/** How long a mood takes to settle, in seconds: the time constant is a quarter of it. */
const EASE = 1.2
const FRAME_MS_CAP = 22
const SAMPLES = 45
const WARMUP_MS = 2000
const SCALE = 0.5
/** The one frame reduced motion draws: a time at which the lamp and motes are unremarkable. */
const STILL_T = 7

const VERT = `attribute vec2 p;void main(){gl_Position=vec4(p,0.,1.);}`

/**
 * Three parts. The cloth is a woven grain (two crossed sines, broken up by hash
 * noise so no repeat is findable) that darkens the baize by a few percent. The
 * lamp is a smooth falloff whose centre drifts on two slow sines and whose
 * radius breathes on a third, at periods of about 20 and 13 seconds, slow enough
 * that nobody sees it move, only that the room is not a picture. The dust is two
 * layers of cells, one mote per cell, each rising through its own cell on its own
 * phase with a sway and a fade at both ends, so a mote never crosses a cell
 * border and one texture lookup's worth of arithmetic per layer is the whole
 * cost. Motes are weighted by the pool, since dust is only visible in the beam.
 */
const FRAG = `
#ifdef GL_FRAGMENT_PRECISION_HIGH
precision highp float;
#else
precision mediump float;
#endif
uniform vec2 uRes;
uniform float uT;
uniform vec3 uLo, uMid, uHi, uLamp;
uniform vec4 uLook; // power, radius, saturation, dust
float hash(vec2 q){ return fract(sin(dot(q, vec2(12.9898, 78.233))) * 43758.5453); }
vec2 hash2(vec2 q){ return vec2(hash(q), hash(q + 17.31)); }
float mote(vec2 uv, float cells, float speed, float size, float seed){
  vec2 g = uv * cells;
  vec2 id = floor(g);
  vec2 f = fract(g);
  vec2 h = hash2(id + seed);
  float ph = fract(h.x * 7.0 + uT * speed * (0.6 + 0.8 * h.y));
  vec2 pos = vec2(0.5 + 0.28 * sin(uT * 0.4 + h.x * 40.0) * (h.y - 0.3), ph);
  float d = length((f - pos) * vec2(1.0, 1.0));
  float life = sin(3.14159 * ph);
  float tw = 0.65 + 0.35 * sin(uT * 1.7 + h.y * 30.0);
  return smoothstep(size, 0.0, d) * life * tw * step(0.9, h.y + h.x * 0.25);
}
void main(){
  vec2 frag = gl_FragCoord.xy;
  vec2 c = frag / uRes;
  float asp = uRes.x / uRes.y;
  vec2 uv = (c - vec2(0.5)) * vec2(asp, 1.0);
  float diag = length(vec2(asp, 1.0));
  vec2 centre = vec2(0.0, 0.08) + vec2(0.03 * sin(uT * 0.31), 0.02 * sin(uT * 0.23 + 1.0));
  float breath = 1.0 + 0.035 * sin(uT * 0.48);
  float dl = length((uv - centre) * vec2(1.0, 1.15)) / (uLook.y * diag * breath);
  float pool = exp(-dl * dl * 2.4) * uLook.x;
  vec3 base = mix(uLo, uMid, smoothstep(1.3, 0.0, dl));
  base = mix(base, uHi, clamp(pool * 0.95, 0.0, 1.0));
  float weave = 0.5 + 0.5 * sin(frag.x * 2.1) * sin(frag.y * 2.1);
  float grain = mix(0.94, 1.03, weave) * (0.96 + 0.08 * hash(floor(frag * 0.5)));
  vec3 col = base * grain;
  col *= mix(vec3(1.0), uLamp * 1.25, clamp(pool * 0.6, 0.0, 1.0));
  col += uLamp * pool * pool * 0.34 * grain;
  float dust = mote(uv + 0.5, 30.0, 0.012, 0.11, 1.0) + 0.8 * mote(uv + 0.31, 16.0, 0.008, 0.13, 5.0);
  col += uLamp * dust * clamp(pool * 1.4, 0.0, 1.0) * 0.5 * uLook.w;
  float l = dot(col, vec3(0.299, 0.587, 0.114));
  col = mix(vec3(l), col, uLook.z);
  col *= 1.0 - 0.6 * smoothstep(0.35, 0.95, length((c - 0.5) * vec2(1.1, 1.25)));
  gl_FragColor = vec4(col, 1.0);
}`

type Gpu = {
  gl: WebGLRenderingContext
  uRes: WebGLUniformLocation | null
  uT: WebGLUniformLocation | null
  uLamp: WebGLUniformLocation | null
  uLook: WebGLUniformLocation | null
}

let canvas: HTMLCanvasElement | null = null
let gpu: Gpu | null = null
let raf = 0
let watching = false

/** Wanted, not current: `setMood` may be called before the canvas exists. */
let target: Mood = "round"
let covered = false
let cur: Look = { ...LOOKS.round, lamp: [...LOOKS.round.lamp] }
let t = 0
let last = 0
let lastDraw = 0
let began = 0
let stale = true
let capped = false
const deltas: number[] = []

/** Parse `#rrggbb` from a token, falling back to the literal the token has. */
function token(name: string, fallback: [number, number, number]): [number, number, number] {
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
  const vs = compile(gl, gl.VERTEX_SHADER, VERT)
  const fs = compile(gl, gl.FRAGMENT_SHADER, FRAG)
  const prog = gl.createProgram()
  if (!vs || !fs || !prog) return null
  gl.attachShader(prog, vs)
  gl.attachShader(prog, fs)
  gl.linkProgram(prog)
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) return null
  gl.useProgram(prog)
  const buf = gl.createBuffer()
  gl.bindBuffer(gl.ARRAY_BUFFER, buf)
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW)
  const at = gl.getAttribLocation(prog, "p")
  gl.enableVertexAttribArray(at)
  gl.vertexAttribPointer(at, 2, gl.FLOAT, false, 0, 0)
  const u = (name: string) => gl.getUniformLocation(prog, name)
  gl.uniform3fv(u("uLo"), token("--felt-lo", [0.024, 0.043, 0.09]))
  gl.uniform3fv(u("uMid"), token("--felt-mid", [0.07, 0.137, 0.26]))
  gl.uniform3fv(u("uHi"), token("--felt-hi", [0.114, 0.208, 0.38]))
  return { gl, uRes: u("uRes"), uT: u("uT"), uLamp: u("uLamp"), uLook: u("uLook") }
}

function fitCanvas(): void {
  if (!canvas || !gpu) return
  const dpr = window.devicePixelRatio || 1
  const w = Math.max(1, Math.round(window.innerWidth * dpr * SCALE))
  const h = Math.max(1, Math.round(window.innerHeight * dpr * SCALE))
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
  gl.uniform3fv(gpu.uLamp, cur.lamp)
  gl.uniform4f(gpu.uLook, cur.power, cur.radius, cur.sat, cur.dust)
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
    lamp: [
      step(cur.lamp[0], goal.lamp[0]),
      step(cur.lamp[1], goal.lamp[1]),
      step(cur.lamp[2], goal.lamp[2]),
    ],
    power: step(cur.power, goal.power),
    radius: step(cur.radius, goal.radius),
    sat: step(cur.sat, goal.sat),
    dust: step(cur.dust, goal.dust),
  }
  if (gap > 0.002) return true
  cur = { ...goal, lamp: [...goal.lamp] }
  return false
}

const running = (): boolean =>
  canvas !== null && gpu !== null && !document.hidden && !covered && juiced()

/** The table's own lights: a table, and the player has not switched them off. */
const lit = (): boolean => isTable() && !document.documentElement.classList.contains("lights-off")

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
        if (canvas) canvas.dataset.fps = "30"
      }
    }
  }
  last = now
  raf = requestAnimationFrame(frame)
  // 28 rather than 33: rAF ticks at 16.7 or 8.3 depending on the display, and
  // a threshold on the tick itself would drop to 20fps on a 60Hz panel when the
  // second tick lands a hair early.
  if (capped && now - lastDraw < 28) return
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
  if (canvas) return
  const el = document.createElement("canvas")
  el.id = "felt-gl"
  el.setAttribute("aria-hidden", "true")
  el.dataset.fps = "60"
  const g = build(el)
  if (!g) return
  canvas = el
  gpu = g
  const app = document.getElementById("app")
  if (app) app.before(el)
  else document.body.prepend(el)
  el.addEventListener("webglcontextlost", (e) => {
    // Without this the browser never restores the context. The canvas is
    // hidden meanwhile so the CSS felt shows, which is the fallback and is
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

function destroy(): void {
  if (raf) cancelAnimationFrame(raf)
  raf = 0
  canvas?.remove()
  canvas = null
  gpu = null
  capped = false
  deltas.length = 0
}

/** Make the canvas match whether the table is on. */
function sync(): void {
  if (lit()) {
    create()
    startMarquee()
  } else {
    destroy()
    stopMarquee()
  }
}

/**
 * Begin following the table. Called once by the shell after the theme lands.
 * The table can flip at any moment (a theme tap, a window crossing the room
 * query), so this watches the root's class rather than being told, and the
 * canvas is created and removed to match. A phone never has one: nothing here
 * creates a node until `isTable()` says so.
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
  if (isTable()) document.documentElement.dataset.mood = mood
  if (moved) stale = true
  if (moved || wasCovered !== covered) kick()
}

/** For tests and the report: is the loop alive, and at what cadence. */
export const backgroundState = (): { drawing: boolean; capped: boolean; reduced: boolean } => ({
  drawing: raf !== 0,
  capped,
  reduced: reduced(),
})
