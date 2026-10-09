import { readFileSync } from "node:fs"

import { describe, expect, it } from "vitest"

import { precached } from "../tools/offline"

/*
 * The list the service worker is built from. What the worker does with it is
 * checked in a browser, since there is no worker in Node to run; what is
 * checked here is the one decision the build makes, which is what goes in.
 */
describe("offline precache", () => {
  const built = [
    "CNAME",
    "assets/index-abc.js",
    "assets/jost-latin-400-normal-abc.woff",
    "assets/jost-latin-400-normal-abc.woff2",
    "assets/promises-abc.ogg",
    "index.html",
    "og.png",
    "sw.js",
    "words/CLAUDE.md",
    "words/en/allowed.txt",
    "words/en/answers.txt",
  ]

  it("holds the page, the bundle, the recordings and the word lists", () => {
    expect(precached(built)).toEqual([
      "assets/index-abc.js",
      "assets/jost-latin-400-normal-abc.woff2",
      "assets/promises-abc.ogg",
      "index.html",
      "words/en/allowed.txt",
      "words/en/answers.txt",
    ])
  })

  it("never holds the worker itself", () => {
    // A cached sw.js is a worker that answers the browser's check for a newer
    // one with itself, which is an install nothing short of clearing site data
    // will ever update.
    expect(precached(["sw.js"])).toEqual([])
  })

  it("is the same list whatever order the disk gave it in", () => {
    // The version is a hash over this list in this order, and `readdir` makes
    // no promise about its own.
    expect(precached([...built].reverse())).toEqual(precached(built))
  })
})

/*
 * The worker's routing, run for real: `src/sw.js` under the constants the build
 * would have written, in a scope of stand-ins. It cannot say the worker works,
 * which takes a browser going offline, but which requests it answers and which
 * it leaves alone is plain logic, and the costly mistake in it is silent: a
 * navigation that matches nothing is a blank page with no network.
 */
describe("offline worker", () => {
  const files = ["assets/index-abc.js", "index.html", "privacy/index.html", "words/en/answers.txt"]
  const origin = "https://5-wild.com"

  /** Whether the worker answers a request itself or leaves it to the browser. */
  function answers(url: string, method = "GET"): boolean {
    const listeners: Record<string, (event: unknown) => void> = {}
    const scope = {
      location: new URL(`${origin}/sw.js`),
      addEventListener: (type: string, listener: (event: unknown) => void) => {
        listeners[type] = listener
      },
    }
    // A cache that holds nothing, so either strategy falls through to the network.
    const caches = { open: async () => ({ match: async () => undefined }) }
    const fetch = async () => new Response("")
    new Function("self", "caches", "fetch", "VERSION", "FILES", "SUMS", source)(
      scope,
      caches,
      fetch,
      "test",
      files,
      {},
    )
    let answered = false
    listeners.fetch?.({
      request: { url, method },
      respondWith: () => {
        answered = true
      },
    })
    return answered
  }

  const source = readFileSync("src/sw.js", "utf8")

  it("answers for the page however it is addressed", () => {
    expect(answers(`${origin}/`)).toBe(true)
    expect(answers(`${origin}/index.html`)).toBe(true)
    // A shared run: the address carries a query and is still the page.
    expect(answers(`${origin}/?seed=ABC&a=2`)).toBe(true)
    expect(answers(`${origin}/privacy/`)).toBe(true)
  })

  it("answers for the bundle and the word lists", () => {
    expect(answers(`${origin}/assets/index-abc.js`)).toBe(true)
    expect(answers(`${origin}/words/en/answers.txt`)).toBe(true)
  })

  it("leaves alone what it does not hold", () => {
    expect(answers(`${origin}/og.png`)).toBe(false)
    // The last build's bundle, asked for by a tab that is still running it.
    expect(answers(`${origin}/assets/index-old.js`)).toBe(false)
  })

  it("leaves alone what is not the site's", () => {
    // The telemetry worker and the spectator feed, and a POST wherever it goes.
    expect(answers("https://telemetry.5-wild.com/runs")).toBe(false)
    expect(answers(`${origin}/index.html`, "POST")).toBe(false)
  })
})
