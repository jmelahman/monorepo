/**
 * A finished episode, as a video of the real game playing it.
 *
 *   bun tools/bench/record.ts <episode.json>... [--out bench/videos] [--size 1280x720]
 *       [--speed 1|2|3] [--skin smoke|tabletop|classic-dark|classic-light] [--webm]
 *       [--silent]
 *
 * Nothing here draws anything. It is `bench:replay` and the `?watch=` spectator
 * with a headless browser as the spectator and its screen kept: the episode's
 * steps go out on the feed, the tab deals the same run from the same seed, and
 * what is filmed is `App` playing it, scoring scenes and all. So a video cannot
 * show a run that was not played. A step the tab's engine refuses stops the
 * recording and fails it, exactly as it stops a person's tab.
 *
 * From the episode and not from the live run, although the feed could be
 * filmed live: a model thinks for a minute a move, a suite is twenty runs, and
 * several models play at once on ports that would have to be told apart. The
 * episode has none of those problems and loses nothing but the refusals, which
 * it does not keep.
 *
 * The look and the pace are set the way a player sets them, by writing the
 * settings keys before the page loads, so there is no recording mode in `src/`
 * to keep in step. `App.watch` promises a spectator writes no storage; it reads
 * it like anyone, and the context is thrown away with the video in it.
 *
 * `--speed` defaults to 2, the middle of the ladder in `speed.ts`: a run is
 * some hundreds of moves, and at authored pace a long one is a quarter of an
 * hour of film. It was 3 while the films were silent. With sound, 3 is a run of
 * cues landing on each other's tails, and 2 is the fastest at which a guess
 * still sounds like a guess. It is the game's own speed setting and not a
 * sped-up encode, so the tiles still land on their beats and nothing is pitched
 * up.
 *
 * The sound is taken inside the page, because Playwright records pixels and has
 * no audio track to offer. Everything the game plays, effects and music both,
 * ends at the destination of the one AudioContext in `audio.ts`, so the page is
 * handed a `connect` that sends whatever reaches the speakers to a recorder as
 * well, before any of the game's code runs. Nothing in `src/` knows. The
 * alternative was a headed browser on a virtual display with a virtual sound
 * card and ffmpeg grabbing both, which films what a player hears and depends on
 * what the machine has installed; this depends on Chromium. The two tracks are
 * laid together by wall clock, the recorder's start against the page's
 * creation, which is good to about a frame or two and not to a sample.
 * `--silent` leaves the sound out.
 *
 * Chromium writes WebM. A blog wants something Safari plays, so with `ffmpeg`
 * on the path each one is re-encoded to H.264 with the index up front, and the
 * WebM is dropped unless `--webm` asks to keep it. `playwright-core` brings no
 * browser of its own: `bunx playwright-core install chromium` once.
 */

import { execFileSync } from "node:child_process"
import { existsSync, mkdirSync, readFileSync, renameSync, rmSync, writeFileSync } from "node:fs"
import { basename, join, resolve } from "node:path"
import { chromium } from "playwright-core"
import { createServer } from "vite"
import type { Episode } from "../../src/bench/session"
import { CONTENT_VERSION } from "../../src/engine"
import { replay } from "./feed"
import { ROOT, slug } from "./host"

const log = (...parts: unknown[]) => console.error("[record]", ...parts)

const args = process.argv.slice(2)
const VALUED = ["--out", "--size", "--speed", "--skin"]
const flag = (name: string): string | undefined => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}
const files = args.filter(
  (arg, at) => !arg.startsWith("--") && !VALUED.includes(args[at - 1] ?? ""),
)

const out = resolve(flag("--out") ?? join(ROOT, "bench", "videos"))
const [width, height] = (flag("--size") ?? "1280x720").split("x").map(Number)
const speed = flag("--speed") ?? "2"
const skin = flag("--skin") ?? "smoke"
const keepWebm = args.includes("--webm")
const silent = args.includes("--silent")

if (!files.length || !width || !height) {
  console.error(
    "usage: bun tools/bench/record.ts <episode.json>... [--out dir] [--size WxH] " +
      "[--speed 1|2|3] [--skin name] [--webm] [--silent]",
  )
  process.exit(2)
}

// Two ports nobody else in the repo uses, so a recording can run beside
// `bun run dev` on 5173 and a live host's feed on 7777 without taking either.
const PAGE_PORT = 5188
const FEED_PORT = 7788

const hasFfmpeg = (() => {
  try {
    execFileSync("ffmpeg", ["-version"], { stdio: "ignore" })
    return true
  } catch {
    return false
  }
})()

/**
 * The dev server and not `dist/`: `dist/` is whatever was last built, and a
 * replay only holds against the engine that dealt the run. What is filmed has
 * to be this checkout, which is also what the content check below is checking.
 */
const server = await createServer({
  root: ROOT,
  logLevel: "error",
  server: { port: PAGE_PORT, strictPort: true },
})
await server.listen()

// A page may not make a sound before somebody has touched it, and nobody will:
// `bindAudioWake` in `app.ts` waits on a gesture the spectator never gets.
// Lifting the policy for this one browser is the honest version of faking a
// click, and it means the context runs from its first sample, which is what
// lets the sound be placed by the clock.
const browser = await chromium.launch({
  args: silent ? [] : ["--autoplay-policy=no-user-gesture-required"],
})

/**
 * Runs in the page ahead of the game. Any node connected to a destination is
 * connected to a recorder's too: the game's limiter is the only one there is,
 * but asking the question of every `connect` is what keeps this from knowing
 * that.
 */
const tap = () => {
  type Film = { recorder: MediaRecorder; chunks: Blob[]; at: number }
  const films = new WeakMap<BaseAudioContext, AudioNode>()
  const connect = AudioNode.prototype.connect as (...args: unknown[]) => unknown
  AudioNode.prototype.connect = function (this: AudioNode, ...args: unknown[]) {
    const target = args[0]
    if (target instanceof AudioDestinationNode) {
      const ctx = target.context as AudioContext
      let sink = films.get(ctx)
      if (!sink) {
        const stream = ctx.createMediaStreamDestination()
        sink = stream
        films.set(ctx, sink)
        const recorder = new MediaRecorder(stream.stream, {
          mimeType: "audio/webm;codecs=opus",
          audioBitsPerSecond: 160_000,
        })
        const film: Film = { recorder, chunks: [], at: 0 }
        recorder.ondataavailable = (event) => film.chunks.push(event.data)
        recorder.onstart = () => {
          film.at = Date.now()
        }
        recorder.start(1000)
        ;(window as unknown as { film?: Film }).film = film
      }
      connect.call(this, sink)
    }
    return connect.apply(this, args)
  } as typeof AudioNode.prototype.connect
}

let failed = 0

for (const file of files) {
  const episode = JSON.parse(readFileSync(file, "utf8")) as Episode
  const name = `${slug(episode.label)}-seed-${episode.seed}-a${episode.ascension}`
  // Refused here rather than left to the banner: the spectator warns about a
  // content mismatch and plays on until it diverges, which is right for a
  // person and is a video of half a run for a script.
  if (episode.content !== CONTENT_VERSION) {
    log(
      `${basename(file)}: content ${episode.content}, this checkout is ${CONTENT_VERSION}; skipped`,
    )
    failed++
    continue
  }

  const feed = replay(file, FEED_PORT)
  mkdirSync(out, { recursive: true })
  const context = await browser.newContext({
    viewport: { width, height },
    recordVideo: { dir: out, size: { width, height } },
    // The Smoke Room stills itself under this preference, and a headless
    // browser's answer to it is not something to leave to a default.
    reducedMotion: "no-preference",
  })
  await context.addInitScript(
    ([speed, skin]) => {
      localStorage.setItem("5wild:speed", speed)
      localStorage.setItem("5wild:skin", skin)
    },
    [speed, skin] as const,
  )
  if (!silent) await context.addInitScript(tap)
  // The film starts when the page exists, so this is its zero for the sound.
  const rolling = Date.now()
  const page = await context.newPage()
  await page.goto(`http://localhost:${PAGE_PORT}/?watch=${encodeURIComponent(feed.url)}`)

  /**
   * The banner is the only thing the spectator says out loud, so it is read
   * the way a person reads it: a label with no live dot is a run that is over,
   * and a bad note that stays is a divergence. "Stays" because the same class
   * marks a dropped connection, which `EventSource` mends by itself.
   *
   * And a banner that never names a run is a tab the feed never reached, which
   * says nothing at all and so used to be waited on for ever: the page failed
   * to load, or the feed's port was still held by the episode before this one,
   * which `startFeed` logs and survives. Half a minute is many times what a
   * replay takes to deal, and the hour is only there so that no fault this
   * comment did not think of can hold a batch overnight.
   */
  const started = Date.now()
  let bad = 0
  let unheard = 0
  let ok = false
  for (;;) {
    await page.waitForTimeout(1000)
    const state = await page.evaluate(() => {
      const banner = document.querySelector(".watch-banner")
      const label = !!banner?.querySelector(".watch-label")
      return {
        label,
        over: label && !banner?.querySelector(".watch-live"),
        bad: banner?.querySelector(".watch-note.bad")?.textContent ?? null,
      }
    })
    if (state.over) {
      ok = true
      break
    }
    unheard = state.label ? 0 : unheard + 1
    if (unheard >= 30) {
      log(`${name}: the tab never heard from the feed`)
      break
    }
    if (Date.now() - started > 60 * 60_000) {
      log(`${name}: still playing after an hour`)
      break
    }
    bad = state.bad ? bad + 1 : 0
    if (bad >= 10) {
      log(`${name}: ${state.bad}`)
      break
    }
  }
  // Long enough for the end screen to finish arriving, so the last frame is
  // the result and not the board it replaced.
  if (ok) await page.waitForTimeout(4000)

  /**
   * The sound comes out as text because that is the only way out of a page:
   * `evaluate` returns JSON. A few megabytes of base64 for a five-minute run.
   * No sound at all is not a failure, since a muted run made none.
   */
  const sound = silent
    ? null
    : await page.evaluate(async () => {
        type Film = { recorder: MediaRecorder; chunks: Blob[]; at: number }
        const film = (window as unknown as { film?: Film }).film
        if (!film) return null
        await new Promise<void>((done) => {
          film.recorder.onstop = () => done()
          film.recorder.stop()
        })
        const bytes = new Uint8Array(await new Blob(film.chunks).arrayBuffer())
        let text = ""
        for (let at = 0; at < bytes.length; at += 0x8000)
          text += String.fromCharCode(...bytes.subarray(at, at + 0x8000))
        return { at: film.at, data: btoa(text) }
      })

  const video = page.video()
  await context.close()
  feed.close()
  const raw = await video?.path()
  if (!ok || !raw) {
    if (raw) rmSync(raw, { force: true })
    failed++
    continue
  }

  const webm = join(out, `${name}.webm`)
  renameSync(raw, webm)
  let made = webm
  // Without ffmpeg there is nothing to lay the sound in with, and the WebM
  // stays the silent film it was.
  const track = join(out, `${name}.audio.webm`)
  // A recorder that never reported starting has no place on the clock, and its
  // `at` of zero would ask ffmpeg to hold the sound back by fifty years.
  const heard = sound?.at && hasFfmpeg ? sound : null
  if (heard) writeFileSync(track, Buffer.from(heard.data, "base64"))
  if (hasFfmpeg) {
    const mp4 = join(out, `${name}.mp4`)
    // Caught, because the film already exists as a WebM and the next episode
    // has nothing to do with this one's encode: thrown from here it would skip
    // the rest of the batch and leave the browser and the dev server holding
    // the process open.
    try {
      execFileSync(
        "ffmpeg",
        // yuv420p and faststart are the two things a `<video>` tag on a static
        // site needs and ffmpeg does not do unasked: Safari refuses 4:4:4, and
        // without the index up front nothing plays until the whole file is down.
        // `-itsoffset` holds the sound back by as long as the page had existed
        // when the recorder started.
        ["-y", "-i", webm]
          .concat(
            heard ? ["-itsoffset", ((heard.at - rolling) / 1000).toFixed(3), "-i", track] : [],
          )
          .concat(["-map", "0:v"], heard ? ["-map", "1:a", "-c:a", "aac", "-b:a", "160k"] : ["-an"])
          .concat(["-c:v", "libx264", "-crf", "23", "-preset", "slow", "-pix_fmt", "yuv420p"])
          .concat(["-movflags", "+faststart", mp4]),
        { stdio: "ignore" },
      )
    } catch {
      log(`${name}: ffmpeg failed; the WebM is kept, silent`)
      rmSync(mp4, { force: true })
      failed++
    }
    rmSync(track, { force: true })
    if (existsSync(mp4)) {
      if (!keepWebm) rmSync(webm)
      made = mp4
    }
  }
  log(
    `${name}: ${episode.steps.length} steps, ${episode.result.roundsCleared} rounds, ` +
      `${Math.round((Date.now() - started) / 1000)}s, ${heard ? "with sound" : "silent"} -> ${made}`,
  )
}

await browser.close()
await server.close()
process.exit(failed ? 1 : 0)
