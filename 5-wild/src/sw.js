/*
 * The service worker: what lets the site, opened once, be played with no
 * network. The APK and the desktop build carry the whole bundle and never
 * register it; see `src/ui/offline.ts`.
 *
 * This file is not bundled and is not what ships. `tools/offline.ts` writes
 * dist/sw.js as three constants followed by this text: `VERSION`, which names
 * the build; `FILES`, the paths it holds, relative to this script; and `SUMS`,
 * the SHA-256 of each of those outside assets/. None is declared here, so on
 * its own this file does not run, and `bun run dev` has no worker at all.
 *
 * The rule everything below follows is that a cache is one build, whole. It is
 * written once, by `install`, under that build's name, and nothing afterwards
 * adds to it: a fetch handler that kept what it saw would, over a few deploys,
 * assemble a cache holding one build's page beside another's word list, and
 * that mix is what an offline player would be dealt.
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
          // there, a deploy that changed one line of CSS does not download the
          // two recordings again, which are 4.0MB of the 5.4MB this holds.
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
        if (name.startsWith(PREFIX) && name !== CACHE) await caches.delete(name)
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
  if (!FILES.includes(file)) return
  event.respondWith(file.startsWith("assets/") ? kept(request, file) : fresh(request, file))
})
