/*
 * The service worker: what lets the site, opened once, be played with no
 * network. The APK and the desktop build carry the whole bundle and never
 * register it; see `src/ui/offline.ts`.
 *
 * This file is not bundled and is not what ships. `tools/offline.ts` writes
 * dist/sw.js as four constants followed by this text: `VERSION`, which names
 * the build; `FILES`, the paths it holds, relative to this script; `SUMS`, the
 * SHA-256 of each of those outside assets/; and `TRACKS`, the recordings, which
 * are held apart and for a reason of their own, below. None is declared here,
 * so on its own this file does not run, and `bun run dev` has no worker at all.
 *
 * The rule everything below follows is that a cache is one build, whole. It is
 * written once, by `install`, under that build's name, and nothing afterwards
 * adds to it: a fetch handler that kept what it saw would, over a few deploys,
 * assemble a cache holding one build's page beside another's word list, and
 * that mix is what an offline player would be dealt.
 *
 * The music is the one thing kept outside that, in a cache of its own that is
 * filled as it is played. The two recordings are 4.0MB against 1.4MB for
 * everything else, and holding them with the build made every first visit a
 * 5.4MB download, on whatever connection, for a player who may have the music
 * off and can only ever be listening to one. So a recording is kept when the
 * page first asks for it: the player who hears it has it offline from then on,
 * and nobody downloads one they never play. It costs the rule nothing, because
 * a recording is named by a hash of itself and so belongs to every build that
 * lists it. What it does cost is that a track never played is silent offline,
 * which is how the game already treats a recording that did not load.
 *
 * What it does not do is serve the page from the cache while the network is
 * there, which is what a precache usually means and what makes one launch
 * faster. The price of that is a site one deploy behind for as long as a tab
 * stays open, and an installed site on a phone is a tab that stays open for
 * days. So the network is asked first for anything whose name does not change
 * when it does, and the cache is what answers when the network cannot.
 */

const PREFIX = "5wild-"
const CACHE = PREFIX + VERSION
/** Outlives the build: see the header for why the music is not in `CACHE`. */
const PLAYED = `${PREFIX}tracks`

/**
 * How long the network gets to answer before the cache does, in ms. For the
 * connection that is neither up nor down, where a fetch does not fail but does
 * not return either, and a player with the whole game on the device watches a
 * blank page wait for a tower. It is a wait for headers, not for the body: a
 * word list that has started arriving is left to finish.
 */
const PATIENCE = 4000

/**
 * A file as the server has it now, and refused unless it is this build's.
 *
 * Past the browser's own HTTP cache for a name that outlives its contents,
 * since Pages lets those be reused for ten minutes; through it for assets/,
 * where the page has usually just fetched the same bytes.
 *
 * The check is for the deploy that is half way out: this script arrives from
 * an edge that has the new build while index.html is still answered by one
 * that has the last, and a cache stamped new is filed holding the old page,
 * for good, since an install that succeeded is not tried again. A mismatch
 * fails the install instead, and the browser's next look at sw.js retries it.
 */
async function download(file) {
  const want = SUMS[file]
  const response = await fetch(file, { cache: want ? "reload" : "default" })
  // `cache.put` stores a 404 as readily as a 200, so it has to be asked.
  if (!response.ok) throw new Error(`${file}: ${response.status}`)
  if (!want) return response
  const bytes = await response.arrayBuffer()
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", bytes))
  const got = Array.from(digest, (byte) => byte.toString(16).padStart(2, "0")).join("")
  if (got !== want) throw new Error(`${file}: not this build's`)
  return new Response(bytes, { headers: response.headers })
}

self.addEventListener("install", (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(CACHE)
      await Promise.all(
        FILES.map(async (file) => {
          // Vite names everything in assets/ by a hash of its contents, so the
          // same name in the last build's cache is the same bytes. Taken from
          // there, a deploy that changed one line of CSS downloads the page and
          // the word lists again and none of the faces, sounds or script.
          const held = file.startsWith("assets/") ? await caches.match(file) : undefined
          await cache.put(file, held ?? (await download(file)))
        }),
      )
      // Any one failure rejects the whole install and this build's worker is
      // discarded, which leaves the last one and its cache in charge. The
      // half-filled cache is filled over on the next attempt.
      //
      // No waiting for the old worker's tabs to close, which on a phone is a
      // wait of days. What that costs a tab still running the last build is
      // what a deploy cost it before there was a worker: anything it has not
      // fetched yet (the other recording, another look's faces) is a name the
      // server no longer has, and `activate` has dropped the cache that did.
      // A reload is the new build; nothing already loaded is taken away.
      await self.skipWaiting()
    })(),
  )
})

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      for (const name of await caches.keys()) {
        if (name.startsWith(PREFIX) && name !== CACHE && name !== PLAYED) await caches.delete(name)
      }
      // A recording that was replaced has a new name, and its old one would
      // otherwise sit here at a megabyte or two for as long as the site does.
      const played = await caches.open(PLAYED)
      const current = TRACKS.map((file) => new URL(file, self.location.href).href)
      for (const request of await played.keys()) {
        if (!current.includes(request.url)) await played.delete(request)
      }
      // The first visit's music started before there was a worker to see it
      // go by, so the track that player is listening to would not be kept
      // until they played it a second time, on a second visit, online. The
      // browser's HTTP cache has it, though, and `only-if-cached` is a fetch
      // that reads from there or fails: it never reaches the network, so this
      // is not the download the header says nobody is made to pay for.
      for (const file of TRACKS) {
        if (await played.match(file)) continue
        const had = await fetch(file, { cache: "only-if-cached", mode: "same-origin" }).catch(
          () => undefined,
        )
        if (had?.status === 200) await played.put(file, had)
      }
      // So the tab that registered this is covered from its first visit, and
      // not from its second.
      await self.clients.claim()
    })(),
  )
})

/** Which of `FILES` a URL is, with a directory read as its index.html. */
function fileOf(url) {
  const root = new URL("./", self.location.href).pathname
  if (!url.pathname.startsWith(root)) return ""
  const file = url.pathname.slice(root.length)
  return file === "" || file.endsWith("/") ? `${file}index.html` : file
}

/** The cache first: for a name that is a hash of what it names. */
async function kept(request, file) {
  const cache = await caches.open(CACHE)
  return (await cache.match(file)) ?? fetch(request)
}

/**
 * A recording: from the cache if it has been played, and into it if not.
 *
 * The copy is filed while the page is already reading its own, so the first
 * play is one download and not two, which holding these with the build made
 * it: the page asked for the track and `install` asked for it again.
 */
async function played(event, file) {
  const cache = await caches.open(PLAYED)
  const held = await cache.match(file)
  if (held) return held
  const response = await fetch(event.request)
  // 200 and nothing else: a 206 is part of a file, and `put` refuses it.
  if (response.status === 200) event.waitUntil(cache.put(file, response.clone()))
  return response
}

/** The network first, and the cache when it fails, refuses or stalls. */
async function fresh(request, file) {
  const cache = await caches.open(CACHE)
  const held = await cache.match(file)
  if (!held) return fetch(request)
  const stall = new AbortController()
  const timer = setTimeout(() => stall.abort(), PATIENCE)
  try {
    const response = await fetch(request, { signal: stall.signal })
    // A navigation keeps its redirects for the browser to follow, and one of
    // those is not `ok` and is not a failure either.
    return response.ok || response.type === "opaqueredirect" ? response : held
  } catch {
    return held
  } finally {
    clearTimeout(timer)
  }
}

self.addEventListener("fetch", (event) => {
  const { request } = event
  const url = new URL(request.url)
  // Returning without an answer leaves the request to the browser, untouched.
  // That is the telemetry POST and the spectator feed, which are other origins,
  // and anything here this build does not hold.
  if (request.method !== "GET" || url.origin !== self.location.origin) return
  // By file and not by request, so `/?seed=…` is the page and not a miss.
  const file = fileOf(url)
  if (TRACKS.includes(file)) return event.respondWith(played(event, file))
  if (!FILES.includes(file)) return
  event.respondWith(file.startsWith("assets/") ? kept(request, file) : fresh(request, file))
})
