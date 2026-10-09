import { createHash } from "node:crypto"
import { readdirSync, readFileSync, writeFileSync } from "node:fs"
import { join, relative, resolve } from "node:path"

import type { Plugin } from "vite"

/**
 * Writes dist/sw.js: `src/sw.js` under the list of files it is to hold and the
 * hashes it holds them to. See that file for what it does with both.
 *
 * It is forty lines here rather than vite-plugin-pwa because the list is the
 * only thing a build has to contribute. The worker's behaviour is the part
 * worth reading, and Workbox would put it behind a configuration object.
 *
 * The list is read back off the disk after the bundle is written, not taken
 * from Rollup's, because half of what the game fetches never passes through
 * Rollup: the word lists, the icons and the privacy page are public/, copied
 * across as they are.
 */

/**
 * What is in dist/ and is not the game's to play from.
 *
 * - `sw.js` is the worker; a worker that caches itself has pinned itself.
 * - `CNAME` is read by GitHub and `og.png` by link previews, neither by a page.
 * - `.md` is `public/words/CLAUDE.md`, which vite copies with its neighbours.
 * - `.woff` is each face's fallback for a browser without woff2, and every
 *   browser with service workers has woff2. They are 0.4MB that would be
 *   downloaded by every player and read by none.
 */
const SKIPPED = [/^sw\.js$/, /^CNAME$/, /^og\.png$/, /\.md$/, /\.woff$/]

/** The files a player needs on the device, out of everything a build wrote. */
export function precached(files: readonly string[]): string[] {
  return files.filter((file) => !SKIPPED.some((skip) => skip.test(file))).sort()
}

/** Every file under a directory, as paths relative to it with forward slashes. */
function walk(dir: string, from = dir): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name)
    return entry.isDirectory() ? walk(path, from) : [relative(from, path).replaceAll("\\", "/")]
  })
}

/** Hex SHA-256, which is also how the worker spells what it downloaded. */
const sum = (bytes: string | Uint8Array): string => createHash("sha256").update(bytes).digest("hex")

export function offline(): Plugin {
  let root = "."
  let outDir = "dist"
  return {
    name: "5wild:offline",
    apply: "build",
    configResolved(config) {
      root = config.root
      outDir = resolve(config.root, config.build.outDir)
    },
    // After the write, by which point public/ has been copied in as well.
    writeBundle() {
      const files = precached(walk(outDir))
      // From the root and not the working directory, the same as `outDir`, so
      // a build run from the monorepo above finds it.
      const worker = readFileSync(resolve(root, "src/sw.js"), "utf8")
      // What each file outside assets/ has to hash to. Those keep their names
      // from one build to the next, so the name says nothing about which
      // build's bytes a server handed back; see `download` in the worker.
      const sums: Record<string, string> = {}
      for (const file of files) {
        if (!file.startsWith("assets/")) sums[file] = sum(readFileSync(join(outDir, file)))
      }
      // Of the contents and not only the names, for the same reason: a sw.js
      // that comes out byte-identical is a deploy no browser will notice. The
      // worker's own text goes in too, so a change to how it caches starts a
      // cache of its own instead of writing over the one in use.
      const version = sum(worker + JSON.stringify([files, sums])).slice(0, 12)
      writeFileSync(
        join(outDir, "sw.js"),
        `const VERSION = ${JSON.stringify(version)}\n` +
          `const FILES = ${JSON.stringify(files)}\n` +
          `const SUMS = ${JSON.stringify(sums)}\n` +
          worker,
      )
    },
  }
}
