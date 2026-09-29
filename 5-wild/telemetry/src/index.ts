/**
 * Receives opted-in run replays from `src/ui/telemetry.ts` and files them.
 *
 * It checks shape and nothing else. Whether a run is *real* is a question only
 * a replay can answer, since a forged log is one the engine refuses somewhere,
 * and the replay happens on the analysis side, where the engine is (see
 * `test/telemetry-report.test.ts`). Putting the engine in here would pin the
 * worker to one `CONTENT_VERSION` and turn every balance change into a deploy.
 *
 * Nothing about the sender is kept. The IP feeds the rate limiter and goes no
 * further, and the only thing the server adds to a row is the day it arrived.
 */

type Env = {
  DB: D1Database
  LIMITER: { limit: (options: { key: string }) => Promise<{ success: boolean }> }
}

/**
 * The site, the APK (Capacitor serves the bundle from `https://localhost`), the
 * desktop build (Tauri serves it from `tauri://localhost` on Linux and from
 * `https://tauri.localhost` on Windows; see desktop/src-tauri/src/main.rs) and
 * the dev server.
 */
const ORIGINS = new Set([
  "https://5-wild.com",
  "https://localhost",
  "tauri://localhost",
  "https://tauri.localhost",
  "http://localhost:5173",
])

/** A long run is a few KB. Anything near this is not a run. */
const MAX_BYTES = 32 * 1024
/** A run past stage 20 is maybe 400 steps. This is a guard, not a rule. */
const MAX_STEPS = 4000

const WORDS = new Set(["en", "es", "fr", "de"])
const ENDS = new Set(["lost", "quit", "abandoned"])

type Run = {
  v: number
  content: number
  build: string
  commit: string
  words: string
  seed: number
  ascension: number
  nth: number
  end: string
  won: boolean
  stage: number
  round: number
  steps: unknown[]
}

const int = (value: unknown, min: number, max: number): value is number =>
  Number.isInteger(value) && (value as number) >= min && (value as number) <= max
const str = (value: unknown, max: number): value is string =>
  typeof value === "string" && value.length <= max

function valid(body: unknown): body is Run {
  if (typeof body !== "object" || body === null) return false
  const run = body as Record<string, unknown>
  return (
    run.v === 1 &&
    int(run.content, 0, 1e6) &&
    str(run.build, 32) &&
    str(run.commit, 40) &&
    typeof run.words === "string" &&
    WORDS.has(run.words) &&
    int(run.seed, 0, 2 ** 31) &&
    int(run.ascension, 0, 100) &&
    int(run.nth, 0, 1e7) &&
    typeof run.end === "string" &&
    ENDS.has(run.end) &&
    typeof run.won === "boolean" &&
    int(run.stage, 1, 1000) &&
    int(run.round, 0, 2) &&
    Array.isArray(run.steps) &&
    run.steps.length <= MAX_STEPS &&
    run.steps.every(
      (step) =>
        typeof step === "object" &&
        step !== null &&
        typeof (step as { type?: unknown }).type === "string",
    )
  )
}

function cors(origin: string | null): Record<string, string> {
  return origin && ORIGINS.has(origin)
    ? {
        "access-control-allow-origin": origin,
        "access-control-allow-methods": "POST, OPTIONS",
        "access-control-allow-headers": "content-type",
        vary: "origin",
      }
    : {}
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const headers = cors(request.headers.get("origin"))
    const reply = (status: number) => new Response(null, { status, headers })

    // The client sends `text/plain` so browsers skip this, but a client that
    // does not is still answered properly.
    if (request.method === "OPTIONS") return reply(204)
    if (request.method !== "POST" || new URL(request.url).pathname !== "/runs") return reply(404)

    const ip = request.headers.get("cf-connecting-ip") ?? "unknown"
    if (!(await env.LIMITER.limit({ key: ip })).success) return reply(429)

    const declared = Number(request.headers.get("content-length") ?? 0)
    if (declared > MAX_BYTES) return reply(413)
    const text = await request.text()
    if (text.length > MAX_BYTES) return reply(413)

    let body: unknown
    try {
      body = JSON.parse(text)
    } catch {
      return reply(400)
    }
    if (!valid(body)) return reply(400)

    await env.DB.prepare(
      `INSERT INTO runs (day, v, content, build, build_commit, words, ascension, nth, ended, won, stage, round, payload)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    )
      .bind(
        new Date().toISOString().slice(0, 10),
        body.v,
        body.content,
        body.build,
        body.commit,
        body.words,
        body.ascension,
        body.nth,
        body.end,
        body.won ? 1 : 0,
        body.stage,
        body.round,
        text,
      )
      .run()
    return reply(204)
  },
}
